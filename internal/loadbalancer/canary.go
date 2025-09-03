package loadbalancer

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/health"
	"github.com/songzhibin97/stargate/internal/types"
)

// CanaryBalancer 金丝雀/灰度发布负载均衡器
// 支持将流量按权重分配到不同版本的上游服务
type CanaryBalancer struct {
	mu            sync.RWMutex
	upstreams     map[string]*canaryUpstreamGroup
	config        *config.Config
	healthChecker *health.ActiveHealthChecker
	rand          *rand.Rand
}

// canaryUpstreamGroup 金丝雀上游服务组
// 包含多个版本的上游服务及其权重配置
type canaryUpstreamGroup struct {
	groupID   string
	versions  []*canaryVersion
	totalWeight int
	strategy  string // "weighted", "percentage", "header_based"
}

// canaryVersion 金丝雀版本配置
type canaryVersion struct {
	version    string           // 版本标识，如 "v1", "v2", "canary"
	upstream   *types.Upstream  // 对应的上游服务
	weight     int              // 权重值
	percentage float64          // 百分比 (0-100)
	targets    []*types.Target  // 健康的目标实例
	metadata   map[string]string // 版本元数据
}

// CanaryConfig 金丝雀发布配置
type CanaryConfig struct {
	GroupID   string                    `json:"group_id"`
	Strategy  string                    `json:"strategy"` // "weighted", "percentage", "header_based"
	Versions  []*CanaryVersionConfig    `json:"versions"`
	Rules     []*CanaryRule            `json:"rules,omitempty"`
}

// CanaryVersionConfig 金丝雀版本配置
type CanaryVersionConfig struct {
	Version    string            `json:"version"`
	UpstreamID string            `json:"upstream_id"`
	Weight     int               `json:"weight"`
	Percentage float64           `json:"percentage"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// CanaryRule 金丝雀路由规则
type CanaryRule struct {
	Type      string            `json:"type"`      // "header", "cookie", "query", "ip"
	Key       string            `json:"key"`       // 规则键名
	Value     string            `json:"value"`     // 规则值
	Version   string            `json:"version"`   // 目标版本
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NewCanaryBalancer 创建新的金丝雀负载均衡器
func NewCanaryBalancer(config *config.Config) *CanaryBalancer {
	return &CanaryBalancer{
		upstreams:     make(map[string]*canaryUpstreamGroup),
		config:        config,
		rand:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select 根据金丝雀策略选择目标实例
func (cb *CanaryBalancer) Select(upstream *types.Upstream) (*types.Target, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// 查找对应的金丝雀组
	group, exists := cb.upstreams[upstream.ID]
	if !exists {
		// 如果没有配置金丝雀，回退到普通负载均衡
		return cb.selectFromSingleUpstream(upstream)
	}

	// 根据策略选择版本
	selectedVersion, err := cb.selectVersion(group)
	if err != nil {
		return nil, fmt.Errorf("failed to select canary version: %w", err)
	}

	// 从选中的版本中选择目标实例
	return cb.selectTargetFromVersion(selectedVersion)
}

// selectVersion 根据配置的策略选择版本
func (cb *CanaryBalancer) selectVersion(group *canaryUpstreamGroup) (*canaryVersion, error) {
	switch group.strategy {
	case "weighted":
		return cb.selectVersionByWeight(group)
	case "percentage":
		return cb.selectVersionByPercentage(group)
	case "header_based":
		// TODO: 实现基于请求头的路由（需要请求上下文）
		return cb.selectVersionByWeight(group)
	default:
		return cb.selectVersionByWeight(group)
	}
}

// selectVersionByWeight 基于权重选择版本
func (cb *CanaryBalancer) selectVersionByWeight(group *canaryUpstreamGroup) (*canaryVersion, error) {
	if len(group.versions) == 0 {
		return nil, fmt.Errorf("no versions available in group %s", group.groupID)
	}

	// 过滤出有健康目标的版本
	healthyVersions := make([]*canaryVersion, 0)
	totalWeight := 0
	
	for _, version := range group.versions {
		if len(version.targets) > 0 {
			healthyVersions = append(healthyVersions, version)
			totalWeight += version.weight
		}
	}

	if len(healthyVersions) == 0 {
		return nil, fmt.Errorf("no healthy versions available in group %s", group.groupID)
	}

	if totalWeight == 0 {
		// 如果所有权重都是0，平均分配
		index := cb.rand.Intn(len(healthyVersions))
		return healthyVersions[index], nil
	}

	// 加权随机选择
	randomWeight := cb.rand.Intn(totalWeight)
	currentWeight := 0

	for _, version := range healthyVersions {
		currentWeight += version.weight
		if randomWeight < currentWeight {
			return version, nil
		}
	}

	// 兜底返回第一个健康版本
	return healthyVersions[0], nil
}

// selectVersionByPercentage 基于百分比选择版本
func (cb *CanaryBalancer) selectVersionByPercentage(group *canaryUpstreamGroup) (*canaryVersion, error) {
	if len(group.versions) == 0 {
		return nil, fmt.Errorf("no versions available in group %s", group.groupID)
	}

	// 过滤出有健康目标的版本
	healthyVersions := make([]*canaryVersion, 0)
	totalPercentage := 0.0
	
	for _, version := range group.versions {
		if len(version.targets) > 0 {
			healthyVersions = append(healthyVersions, version)
			totalPercentage += version.percentage
		}
	}

	if len(healthyVersions) == 0 {
		return nil, fmt.Errorf("no healthy versions available in group %s", group.groupID)
	}

	// 生成0-100的随机数
	randomPercentage := cb.rand.Float64() * 100
	currentPercentage := 0.0

	for _, version := range healthyVersions {
		currentPercentage += version.percentage
		if randomPercentage <= currentPercentage {
			return version, nil
		}
	}

	// 兜底返回第一个健康版本
	return healthyVersions[0], nil
}

// selectTargetFromVersion 从指定版本中选择目标实例
func (cb *CanaryBalancer) selectTargetFromVersion(version *canaryVersion) (*types.Target, error) {
	if len(version.targets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for version %s", version.version)
	}

	// 简单轮询选择（可以后续扩展为其他算法）
	index := cb.rand.Intn(len(version.targets))
	return version.targets[index], nil
}

// selectFromSingleUpstream 从单个上游服务中选择目标（回退逻辑）
func (cb *CanaryBalancer) selectFromSingleUpstream(upstream *types.Upstream) (*types.Target, error) {
	healthyTargets := make([]*types.Target, 0)
	for _, target := range upstream.Targets {
		if target.Healthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	if len(healthyTargets) == 0 {
		return nil, fmt.Errorf("no healthy targets available for upstream %s", upstream.ID)
	}

	// 简单轮询选择
	index := cb.rand.Intn(len(healthyTargets))
	return healthyTargets[index], nil
}

// UpdateUpstream 更新上游服务配置
func (cb *CanaryBalancer) UpdateUpstream(upstream *types.Upstream) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 检查上游服务的元数据，判断是否属于某个金丝雀组
	groupID := upstream.Metadata["canary_group"]
	version := upstream.Metadata["canary_version"]

	if groupID == "" || version == "" {
		// 不属于任何金丝雀组，作为独立上游处理
		return cb.updateStandaloneUpstream(upstream)
	}

	// 更新金丝雀组中的版本
	return cb.updateCanaryVersion(groupID, version, upstream)
}

// updateStandaloneUpstream 更新独立的上游服务
func (cb *CanaryBalancer) updateStandaloneUpstream(upstream *types.Upstream) error {
	// 为独立上游创建单版本组
	group := &canaryUpstreamGroup{
		groupID:  upstream.ID,
		strategy: "single",
		versions: []*canaryVersion{
			{
				version:  "default",
				upstream: upstream,
				weight:   100,
				percentage: 100.0,
				targets:  cb.getHealthyTargets(upstream.Targets),
				metadata: upstream.Metadata,
			},
		},
		totalWeight: 100,
	}

	cb.upstreams[upstream.ID] = group
	return nil
}

// updateCanaryVersion 更新金丝雀组中的特定版本
func (cb *CanaryBalancer) updateCanaryVersion(groupID, version string, upstream *types.Upstream) error {
	group, exists := cb.upstreams[groupID]
	if !exists {
		return fmt.Errorf("canary group %s not found", groupID)
	}

	// 查找并更新对应版本
	for _, v := range group.versions {
		if v.version == version {
			v.upstream = upstream
			v.targets = cb.getHealthyTargets(upstream.Targets)
			return nil
		}
	}

	return fmt.Errorf("version %s not found in canary group %s", version, groupID)
}

// getHealthyTargets 获取健康的目标实例
func (cb *CanaryBalancer) getHealthyTargets(targets []*types.Target) []*types.Target {
	healthy := make([]*types.Target, 0)
	for _, target := range targets {
		if target.Healthy {
			healthy = append(healthy, target)
		}
	}
	return healthy
}

// UpdateTargetHealth 更新目标实例的健康状态
func (cb *CanaryBalancer) UpdateTargetHealth(upstreamID, targetHost string, targetPort int, healthy bool) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 查找包含该上游的金丝雀组
	for _, group := range cb.upstreams {
		for _, version := range group.versions {
			if version.upstream != nil && version.upstream.ID == upstreamID {
				// 更新目标健康状态
				for _, target := range version.upstream.Targets {
					if target.Host == targetHost && target.Port == targetPort {
						target.Healthy = healthy
						// 重新计算健康目标列表
						version.targets = cb.getHealthyTargets(version.upstream.Targets)
						return nil
					}
				}
			}
		}
	}

	return fmt.Errorf("target %s:%d not found in upstream %s", targetHost, targetPort, upstreamID)
}

// UpdateCanaryGroup 更新金丝雀组配置
func (cb *CanaryBalancer) UpdateCanaryGroup(config *CanaryConfig) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	group := &canaryUpstreamGroup{
		groupID:   config.GroupID,
		strategy:  config.Strategy,
		versions:  make([]*canaryVersion, 0),
	}

	totalWeight := 0
	for _, versionConfig := range config.Versions {
		version := &canaryVersion{
			version:    versionConfig.Version,
			weight:     versionConfig.Weight,
			percentage: versionConfig.Percentage,
			targets:    make([]*types.Target, 0),
			metadata:   versionConfig.Metadata,
		}
		
		group.versions = append(group.versions, version)
		totalWeight += versionConfig.Weight
	}

	group.totalWeight = totalWeight
	cb.upstreams[config.GroupID] = group

	return nil
}

// RemoveUpstream 移除上游服务
func (cb *CanaryBalancer) RemoveUpstream(id string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	delete(cb.upstreams, id)
	return nil
}

// Health 返回负载均衡器的健康状态
func (cb *CanaryBalancer) Health() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	health := map[string]interface{}{
		"type":         "canary",
		"groups_count": len(cb.upstreams),
		"timestamp":    time.Now().Unix(),
	}

	groups := make(map[string]interface{})
	for groupID, group := range cb.upstreams {
		versions := make([]map[string]interface{}, 0)
		for _, version := range group.versions {
			versions = append(versions, map[string]interface{}{
				"version":      version.version,
				"weight":       version.weight,
				"percentage":   version.percentage,
				"targets_count": len(version.targets),
			})
		}
		
		groups[groupID] = map[string]interface{}{
			"strategy":     group.strategy,
			"total_weight": group.totalWeight,
			"versions":     versions,
		}
	}

	health["groups"] = groups
	return health
}

// GetUpstream 获取上游服务（兼容现有接口）
func (cb *CanaryBalancer) GetUpstream(id string) (*types.Upstream, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	group, exists := cb.upstreams[id]
	if !exists {
		return nil, fmt.Errorf("upstream %s not found", id)
	}

	// 如果是单版本组，直接返回该版本的上游
	if len(group.versions) == 1 {
		return group.versions[0].upstream, nil
	}

	// 对于多版本组，返回主版本（权重最大的版本）
	var mainVersion *canaryVersion
	maxWeight := 0
	for _, version := range group.versions {
		if version.weight > maxWeight {
			maxWeight = version.weight
			mainVersion = version
		}
	}

	if mainVersion != nil && mainVersion.upstream != nil {
		return mainVersion.upstream, nil
	}

	return nil, fmt.Errorf("no main version found for upstream group %s", id)
}

// GetCanaryGroup 获取金丝雀组配置
func (cb *CanaryBalancer) GetCanaryGroup(groupID string) (*CanaryConfig, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	group, exists := cb.upstreams[groupID]
	if !exists {
		return nil, fmt.Errorf("canary group %s not found", groupID)
	}

	config := &CanaryConfig{
		GroupID:  group.groupID,
		Strategy: group.strategy,
		Versions: make([]*CanaryVersionConfig, 0),
	}

	for _, version := range group.versions {
		versionConfig := &CanaryVersionConfig{
			Version:    version.version,
			Weight:     version.weight,
			Percentage: version.percentage,
			Metadata:   version.metadata,
		}
		if version.upstream != nil {
			versionConfig.UpstreamID = version.upstream.ID
		}
		config.Versions = append(config.Versions, versionConfig)
	}

	return config, nil
}

// ListCanaryGroups 列出所有金丝雀组
func (cb *CanaryBalancer) ListCanaryGroups() []*CanaryConfig {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	configs := make([]*CanaryConfig, 0)
	for groupID := range cb.upstreams {
		if config, err := cb.GetCanaryGroup(groupID); err == nil {
			configs = append(configs, config)
		}
	}

	return configs
}

// RemoveCanaryGroup 移除金丝雀组
func (cb *CanaryBalancer) RemoveCanaryGroup(groupID string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if _, exists := cb.upstreams[groupID]; !exists {
		return fmt.Errorf("canary group %s not found", groupID)
	}

	delete(cb.upstreams, groupID)
	return nil
}

// SetHealthChecker 设置健康检查器
func (cb *CanaryBalancer) SetHealthChecker(healthChecker *health.ActiveHealthChecker) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.healthChecker = healthChecker
}

// Start 启动负载均衡器
func (cb *CanaryBalancer) Start() error {
	// 启动健康检查器
	if cb.healthChecker != nil {
		return cb.healthChecker.Start()
	}
	return nil
}

// Stop 停止负载均衡器
func (cb *CanaryBalancer) Stop() error {
	// 停止健康检查器
	if cb.healthChecker != nil {
		return cb.healthChecker.Stop()
	}
	return nil
}
