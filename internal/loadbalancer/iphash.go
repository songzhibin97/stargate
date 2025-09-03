package loadbalancer

import (
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/types"
)

// IPHashBalancer IP哈希负载均衡器
type IPHashBalancer struct {
	mu            sync.RWMutex
	upstreams     map[string]*ipHashUpstreamState
	config        *config.Config
	healthChecker *health.ActiveHealthChecker
}

// ipHashUpstreamState 维护IP哈希上游服务的状态
type ipHashUpstreamState struct {
	upstream *types.Upstream
	targets  []*types.Target
	ring     *consistentHashRing // 一致性哈希环
}

// consistentHashRing 一致性哈希环
type consistentHashRing struct {
	nodes    []hashNode
	replicas int // 虚拟节点数量
}

// hashNode 哈希环节点
type hashNode struct {
	hash   uint32
	target *types.Target
}

// NewIPHashBalancer 创建新的IP哈希负载均衡器
func NewIPHashBalancer(cfg *config.Config) *IPHashBalancer {
	ih := &IPHashBalancer{
		upstreams:     make(map[string]*ipHashUpstreamState),
		config:        cfg,
		healthChecker: health.NewActiveHealthChecker(cfg),
	}

	// 添加健康状态变化回调
	ih.healthChecker.AddHealthChangeCallback(ih.onHealthChange)

	// 启动健康检查器
	ih.healthChecker.Start()

	return ih
}

// onHealthChange 健康状态变化回调
func (ih *IPHashBalancer) onHealthChange(upstreamID string, target *types.Target, healthy bool) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	// 更新目标实例的健康状态
	if state, exists := ih.upstreams[upstreamID]; exists {
		for _, t := range state.targets {
			if t.Host == target.Host && t.Port == target.Port {
				t.Healthy = healthy
				break
			}
		}
		// 重建一致性哈希环
		state.ring = ih.buildConsistentHashRing(state.targets)
	}
}

// Select 使用IP哈希算法选择目标实例
func (ih *IPHashBalancer) Select(upstream *types.Upstream) (*types.Target, error) {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	// 获取或创建上游状态
	state, exists := ih.upstreams[upstream.ID]
	if !exists {
		err := ih.updateUpstreamLocked(upstream)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize upstream: %w", err)
		}
		state = ih.upstreams[upstream.ID]
	}

	// 获取健康的目标实例（包括被动健康检查状态）
	healthyTargets := ih.getHealthyTargets(state.targets)
	if len(healthyTargets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for upstream %s", upstream.ID)
	}

	// 从上下文中获取客户端IP（这里先使用占位符，后续在pipeline中设置）
	clientIP := ih.getClientIPFromContext()
	if clientIP == "" {
		// 如果无法获取客户端IP，回退到轮询
		return healthyTargets[0], nil
	}

	// 使用IP哈希选择目标
	selected := ih.selectByIPHash(clientIP, healthyTargets)
	if selected == nil {
		return nil, fmt.Errorf("failed to select target for upstream %s", upstream.ID)
	}

	return selected, nil
}

// selectByIPHash 使用IP哈希算法选择目标
func (ih *IPHashBalancer) selectByIPHash(clientIP string, targets []*types.Target) *types.Target {
	if len(targets) == 0 {
		return nil
	}

	// 使用FNV哈希算法
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	hashValue := hash.Sum32()

	// 简单取模选择目标
	index := hashValue % uint32(len(targets))
	return targets[index]
}

// selectByConsistentHash 使用一致性哈希算法选择目标
func (ih *IPHashBalancer) selectByConsistentHash(clientIP string, ring *consistentHashRing) *types.Target {
	if len(ring.nodes) == 0 {
		return nil
	}

	// 计算客户端IP的哈希值
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	clientHash := hash.Sum32()

	// 在哈希环上找到第一个大于等于客户端哈希值的节点
	index := sort.Search(len(ring.nodes), func(i int) bool {
		return ring.nodes[i].hash >= clientHash
	})

	// 如果没找到，回到环的开头（环形结构）
	if index == len(ring.nodes) {
		index = 0
	}

	return ring.nodes[index].target
}

// getClientIPFromContext 从上下文中获取客户端IP（占位符实现）
func (ih *IPHashBalancer) getClientIPFromContext() string {
	// 这里是占位符实现，实际的IP提取将在pipeline中完成
	// 并通过上下文传递给负载均衡器
	return ""
}

// ExtractClientIP 从HTTP请求中提取客户端IP地址
func ExtractClientIP(r *http.Request) string {
	// 优先级：X-Real-IP > X-Forwarded-For > RemoteAddr
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return strings.TrimSpace(ip)
	}

	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For 可能包含多个IP，取第一个（原始客户端IP）
		if ips := strings.Split(forwarded, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// 从RemoteAddr提取IP地址
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}

	return r.RemoteAddr
}

// getHealthyTargets 获取健康的目标实例
func (ih *IPHashBalancer) getHealthyTargets(targets []*types.Target) []*types.Target {
	healthy := make([]*types.Target, 0, len(targets))
	for _, target := range targets {
		if target.Healthy {
			healthy = append(healthy, target)
		}
	}
	return healthy
}

// UpdateUpstream 更新或添加上游服务
func (ih *IPHashBalancer) UpdateUpstream(upstream *types.Upstream) error {
	ih.mu.Lock()
	defer ih.mu.Unlock()
	return ih.updateUpstreamLocked(upstream)
}

// updateUpstreamLocked 内部更新上游服务方法（需要持有锁）
func (ih *IPHashBalancer) updateUpstreamLocked(upstream *types.Upstream) error {
	// 验证上游服务配置
	if err := ih.validateUpstream(upstream); err != nil {
		return err
	}

	// 创建目标实例副本
	targets := make([]*types.Target, len(upstream.Targets))
	copy(targets, upstream.Targets)

	// 创建一致性哈希环
	ring := ih.buildConsistentHashRing(targets)

	// 创建或更新上游状态
	ih.upstreams[upstream.ID] = &ipHashUpstreamState{
		upstream: upstream,
		targets:  targets,
		ring:     ring,
	}

	// 添加到健康检查器
	if ih.healthChecker != nil {
		ih.healthChecker.AddUpstream(upstream)
	}

	return nil
}

// buildConsistentHashRing 构建一致性哈希环
func (ih *IPHashBalancer) buildConsistentHashRing(targets []*types.Target) *consistentHashRing {
	ring := &consistentHashRing{
		replicas: 150, // 每个实际节点对应150个虚拟节点
	}

	// 为每个目标创建虚拟节点
	for _, target := range targets {
		for i := 0; i < ring.replicas; i++ {
			virtualKey := fmt.Sprintf("%s:%d#%d", target.Host, target.Port, i)
			hash := fnv.New32a()
			hash.Write([]byte(virtualKey))
			
			ring.nodes = append(ring.nodes, hashNode{
				hash:   hash.Sum32(),
				target: target,
			})
		}
	}

	// 按哈希值排序
	sort.Slice(ring.nodes, func(i, j int) bool {
		return ring.nodes[i].hash < ring.nodes[j].hash
	})

	return ring
}

// RemoveUpstream 移除上游服务
func (ih *IPHashBalancer) RemoveUpstream(id string) error {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	if _, exists := ih.upstreams[id]; !exists {
		return fmt.Errorf("upstream %s not found", id)
	}

	delete(ih.upstreams, id)

	// 从健康检查器中移除
	if ih.healthChecker != nil {
		ih.healthChecker.RemoveUpstream(id)
	}

	return nil
}

// GetUpstream 获取上游服务
func (ih *IPHashBalancer) GetUpstream(id string) (*types.Upstream, error) {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	state, exists := ih.upstreams[id]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", id)
	}

	return state.upstream, nil
}

// ListUpstreams 列出所有上游服务
func (ih *IPHashBalancer) ListUpstreams() []*types.Upstream {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	upstreams := make([]*types.Upstream, 0, len(ih.upstreams))
	for _, state := range ih.upstreams {
		upstreams = append(upstreams, state.upstream)
	}

	return upstreams
}

// UpdateTargetHealth 更新目标实例的健康状态
func (ih *IPHashBalancer) UpdateTargetHealth(upstreamID, targetHost string, targetPort int, healthy bool) error {
	ih.mu.Lock()
	defer ih.mu.Unlock()

	state, exists := ih.upstreams[upstreamID]
	if !exists {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	// 查找并更新目标健康状态
	for _, target := range state.targets {
		if target.Host == targetHost && target.Port == targetPort {
			target.Healthy = healthy
			
			// 重建一致性哈希环
			state.ring = ih.buildConsistentHashRing(state.targets)
			return nil
		}
	}

	return fmt.Errorf("target %s:%d not found in upstream %s", targetHost, targetPort, upstreamID)
}

// Health 返回负载均衡器的健康状态
func (ih *IPHashBalancer) Health() map[string]interface{} {
	ih.mu.RLock()
	defer ih.mu.RUnlock()

	health := map[string]interface{}{
		"type":              "ip_hash",
		"upstreams_count":   len(ih.upstreams),
		"timestamp":         time.Now().Unix(),
	}

	// 添加每个上游的详细信息
	upstreams := make(map[string]interface{})
	for id, state := range ih.upstreams {
		healthyCount := 0
		targetDetails := make([]map[string]interface{}, len(state.targets))
		
		for i, target := range state.targets {
			if target.Healthy {
				healthyCount++
			}
			targetDetails[i] = map[string]interface{}{
				"host":    target.Host,
				"port":    target.Port,
				"healthy": target.Healthy,
			}
		}

		upstreams[id] = map[string]interface{}{
			"total_targets":    len(state.targets),
			"healthy_targets":  healthyCount,
			"virtual_nodes":    len(state.ring.nodes),
			"targets":          targetDetails,
		}
	}

	health["upstreams"] = upstreams
	return health
}

// validateUpstream 验证上游服务配置
func (ih *IPHashBalancer) validateUpstream(upstream *types.Upstream) error {
	if upstream == nil {
		return fmt.Errorf("upstream cannot be nil")
	}

	if upstream.ID == "" {
		return fmt.Errorf("upstream ID cannot be empty")
	}

	if len(upstream.Targets) == 0 {
		return fmt.Errorf("upstream must have at least one target")
	}

	// 验证目标配置
	for i, target := range upstream.Targets {
		if target.Host == "" {
			return fmt.Errorf("target %d: host cannot be empty", i)
		}
		if target.Port <= 0 || target.Port > 65535 {
			return fmt.Errorf("target %d: invalid port %d", i, target.Port)
		}
	}

	return nil
}
