package loadbalancer

import (
	"fmt"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/types"
)

// WeightedRoundRobinBalancer 加权轮询负载均衡器
type WeightedRoundRobinBalancer struct {
	mu            sync.RWMutex
	upstreams     map[string]*weightedUpstreamState
	config        *config.Config
	healthChecker *health.ActiveHealthChecker
}

// weightedUpstreamState 维护加权上游服务的状态
type weightedUpstreamState struct {
	upstream    *types.Upstream
	targets     []*weightedTarget
	totalWeight int
}

// weightedTarget 加权目标实例
type weightedTarget struct {
	target        *types.Target
	weight        int  // 配置的静态权重
	currentWeight int  // 动态当前权重
	healthy       bool // 健康状态
}

// NewWeightedRoundRobinBalancer 创建新的加权轮询负载均衡器
func NewWeightedRoundRobinBalancer(cfg *config.Config) *WeightedRoundRobinBalancer {
	wrr := &WeightedRoundRobinBalancer{
		upstreams:     make(map[string]*weightedUpstreamState),
		config:        cfg,
		healthChecker: health.NewActiveHealthChecker(cfg),
	}

	// 添加健康状态变化回调
	wrr.healthChecker.AddHealthChangeCallback(wrr.onHealthChange)

	// 启动健康检查器
	wrr.healthChecker.Start()

	return wrr
}

// onHealthChange 健康状态变化回调
func (wrr *WeightedRoundRobinBalancer) onHealthChange(upstreamID string, target *types.Target, healthy bool) {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	// 更新目标实例的健康状态
	if state, exists := wrr.upstreams[upstreamID]; exists {
		for _, wt := range state.targets {
			if wt.target.Host == target.Host && wt.target.Port == target.Port {
				wt.healthy = healthy
				wt.target.Healthy = healthy
				break
			}
		}
	}
}

// Select 使用加权轮询算法选择目标实例
func (wrr *WeightedRoundRobinBalancer) Select(upstream *types.Upstream) (*types.Target, error) {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	// 获取或创建上游状态
	state, exists := wrr.upstreams[upstream.ID]
	if !exists {
		err := wrr.updateUpstreamLocked(upstream)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize upstream: %w", err)
		}
		state = wrr.upstreams[upstream.ID]
	}

	// 获取健康的目标实例
	healthyTargets := wrr.getHealthyTargets(state.targets)
	if len(healthyTargets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for upstream %s", upstream.ID)
	}

	// 执行加权轮询选择
	selected := wrr.selectWeightedTarget(healthyTargets)
	if selected == nil {
		return nil, fmt.Errorf("failed to select target for upstream %s", upstream.ID)
	}
	
	return selected, nil
}

// selectWeightedTarget 执行平滑加权轮询算法
func (wrr *WeightedRoundRobinBalancer) selectWeightedTarget(targets []*weightedTarget) *types.Target {
	var selected *weightedTarget
	totalWeight := 0

	// 第一步：增加所有目标的当前权重，并计算总权重
	for _, target := range targets {
		if !target.healthy {
			continue
		}
		
		target.currentWeight += target.weight
		totalWeight += target.weight

		// 选择当前权重最大的目标
		if selected == nil || target.currentWeight > selected.currentWeight {
			selected = target
		}
	}

	// 第二步：减少选中目标的当前权重
	if selected != nil {
		selected.currentWeight -= totalWeight
		return selected.target
	}

	return nil
}

// getHealthyTargets 获取健康的目标实例
func (wrr *WeightedRoundRobinBalancer) getHealthyTargets(targets []*weightedTarget) []*weightedTarget {
	healthy := make([]*weightedTarget, 0, len(targets))
	for _, target := range targets {
		// 检查目标是否健康且未被被动健康检查隔离
		if target.healthy && target.target.Healthy {
			healthy = append(healthy, target)
		}
	}
	return healthy
}

// UpdateUpstream 更新或添加上游服务
func (wrr *WeightedRoundRobinBalancer) UpdateUpstream(upstream *types.Upstream) error {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()
	return wrr.updateUpstreamLocked(upstream)
}

// updateUpstreamLocked 内部更新上游服务方法（需要持有锁）
func (wrr *WeightedRoundRobinBalancer) updateUpstreamLocked(upstream *types.Upstream) error {
	// 验证上游服务配置
	if err := wrr.validateUpstream(upstream); err != nil {
		return err
	}

	// 创建加权目标实例
	weightedTargets := make([]*weightedTarget, len(upstream.Targets))
	totalWeight := 0

	for i, target := range upstream.Targets {
		weight := target.Weight
		if weight <= 0 {
			weight = 1 // 默认权重为1
		}

		weightedTargets[i] = &weightedTarget{
			target:        target,
			weight:        weight,
			currentWeight: 0, // 初始当前权重为0
			healthy:       target.Healthy,
		}
		totalWeight += weight
	}

	// 创建或更新上游状态
	wrr.upstreams[upstream.ID] = &weightedUpstreamState{
		upstream:    upstream,
		targets:     weightedTargets,
		totalWeight: totalWeight,
	}

	// 添加到健康检查器
	if wrr.healthChecker != nil {
		wrr.healthChecker.AddUpstream(upstream)
	}

	return nil
}

// RemoveUpstream 移除上游服务
func (wrr *WeightedRoundRobinBalancer) RemoveUpstream(id string) error {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	if _, exists := wrr.upstreams[id]; !exists {
		return fmt.Errorf("upstream %s not found", id)
	}

	delete(wrr.upstreams, id)

	// 从健康检查器中移除
	if wrr.healthChecker != nil {
		wrr.healthChecker.RemoveUpstream(id)
	}

	return nil
}

// GetUpstream 获取上游服务
func (wrr *WeightedRoundRobinBalancer) GetUpstream(id string) (*types.Upstream, error) {
	wrr.mu.RLock()
	defer wrr.mu.RUnlock()

	state, exists := wrr.upstreams[id]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", id)
	}

	return state.upstream, nil
}

// ListUpstreams 列出所有上游服务
func (wrr *WeightedRoundRobinBalancer) ListUpstreams() []*types.Upstream {
	wrr.mu.RLock()
	defer wrr.mu.RUnlock()

	upstreams := make([]*types.Upstream, 0, len(wrr.upstreams))
	for _, state := range wrr.upstreams {
		upstreams = append(upstreams, state.upstream)
	}

	return upstreams
}

// UpdateTargetHealth 更新目标实例的健康状态
func (wrr *WeightedRoundRobinBalancer) UpdateTargetHealth(upstreamID, targetHost string, targetPort int, healthy bool) error {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	state, exists := wrr.upstreams[upstreamID]
	if !exists {
		return fmt.Errorf("upstream %s not found", upstreamID)
	}

	// 查找并更新目标健康状态
	for _, target := range state.targets {
		if target.target.Host == targetHost && target.target.Port == targetPort {
			target.healthy = healthy
			target.target.Healthy = healthy
			return nil
		}
	}

	return fmt.Errorf("target %s:%d not found in upstream %s", targetHost, targetPort, upstreamID)
}

// Health 返回负载均衡器的健康状态
func (wrr *WeightedRoundRobinBalancer) Health() map[string]interface{} {
	wrr.mu.RLock()
	defer wrr.mu.RUnlock()

	health := map[string]interface{}{
		"type":              "weighted_round_robin",
		"upstreams_count":   len(wrr.upstreams),
		"timestamp":         time.Now().Unix(),
	}

	// 添加每个上游的详细信息
	upstreams := make(map[string]interface{})
	for id, state := range wrr.upstreams {
		healthyCount := 0
		targetDetails := make([]map[string]interface{}, len(state.targets))
		
		for i, target := range state.targets {
			if target.healthy {
				healthyCount++
			}
			targetDetails[i] = map[string]interface{}{
				"host":           target.target.Host,
				"port":           target.target.Port,
				"weight":         target.weight,
				"current_weight": target.currentWeight,
				"healthy":        target.healthy,
			}
		}

		upstreams[id] = map[string]interface{}{
			"total_targets":  len(state.targets),
			"healthy_targets": healthyCount,
			"total_weight":   state.totalWeight,
			"targets":        targetDetails,
		}
	}

	health["upstreams"] = upstreams
	return health
}

// validateUpstream 验证上游服务配置
func (wrr *WeightedRoundRobinBalancer) validateUpstream(upstream *types.Upstream) error {
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
		if target.Weight < 0 {
			return fmt.Errorf("target %d: weight cannot be negative", i)
		}
	}

	return nil
}
