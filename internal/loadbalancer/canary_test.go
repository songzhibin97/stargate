package loadbalancer

import (
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

func TestCanaryBalancer_WeightedSelection(t *testing.T) {
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// 创建金丝雀组配置
	canaryConfig := &CanaryConfig{
		GroupID:  "test-group",
		Strategy: "weighted",
		Versions: []*CanaryVersionConfig{
			{
				Version:    "v1",
				UpstreamID: "upstream-v1",
				Weight:     90,
				Percentage: 90.0,
			},
			{
				Version:    "v2",
				UpstreamID: "upstream-v2",
				Weight:     10,
				Percentage: 10.0,
			},
		},
	}

	err := cb.UpdateCanaryGroup(canaryConfig)
	if err != nil {
		t.Fatalf("Failed to update canary group: %v", err)
	}

	// 创建上游服务
	upstreamV1 := &types.Upstream{
		ID:   "upstream-v1",
		Name: "Service V1",
		Targets: []*types.Target{
			{Host: "v1-host1", Port: 8080, Healthy: true},
			{Host: "v1-host2", Port: 8080, Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "test-group",
			"canary_version": "v1",
		},
	}

	upstreamV2 := &types.Upstream{
		ID:   "upstream-v2",
		Name: "Service V2",
		Targets: []*types.Target{
			{Host: "v2-host1", Port: 8080, Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "test-group",
			"canary_version": "v2",
		},
	}

	// 更新上游服务
	err = cb.UpdateUpstream(upstreamV1)
	if err != nil {
		t.Fatalf("Failed to update upstream v1: %v", err)
	}

	err = cb.UpdateUpstream(upstreamV2)
	if err != nil {
		t.Fatalf("Failed to update upstream v2: %v", err)
	}

	// 测试权重分配
	v1Count := 0
	v2Count := 0
	totalRequests := 1000

	for i := 0; i < totalRequests; i++ {
		target, err := cb.Select(&types.Upstream{ID: "test-group"})
		if err != nil {
			t.Fatalf("Failed to select target: %v", err)
		}

		if target.Host == "v1-host1" || target.Host == "v1-host2" {
			v1Count++
		} else if target.Host == "v2-host1" {
			v2Count++
		}
	}

	// 验证权重分配（允许5%的误差）
	v1Percentage := float64(v1Count) / float64(totalRequests) * 100
	v2Percentage := float64(v2Count) / float64(totalRequests) * 100

	if v1Percentage < 85 || v1Percentage > 95 {
		t.Errorf("V1 percentage should be around 90%%, got %.2f%%", v1Percentage)
	}

	if v2Percentage < 5 || v2Percentage > 15 {
		t.Errorf("V2 percentage should be around 10%%, got %.2f%%", v2Percentage)
	}

	t.Logf("V1: %.2f%%, V2: %.2f%%", v1Percentage, v2Percentage)
}

func TestCanaryBalancer_PercentageSelection(t *testing.T) {
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// 创建基于百分比的金丝雀组配置
	canaryConfig := &CanaryConfig{
		GroupID:  "percentage-group",
		Strategy: "percentage",
		Versions: []*CanaryVersionConfig{
			{
				Version:    "stable",
				UpstreamID: "upstream-stable",
				Weight:     0, // 权重不使用
				Percentage: 80.0,
			},
			{
				Version:    "canary",
				UpstreamID: "upstream-canary",
				Weight:     0, // 权重不使用
				Percentage: 20.0,
			},
		},
	}

	err := cb.UpdateCanaryGroup(canaryConfig)
	if err != nil {
		t.Fatalf("Failed to update canary group: %v", err)
	}

	// 创建上游服务
	upstreamStable := &types.Upstream{
		ID:   "upstream-stable",
		Name: "Stable Service",
		Targets: []*types.Target{
			{Host: "stable-host", Port: 8080, Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "percentage-group",
			"canary_version": "stable",
		},
	}

	upstreamCanary := &types.Upstream{
		ID:   "upstream-canary",
		Name: "Canary Service",
		Targets: []*types.Target{
			{Host: "canary-host", Port: 8080, Healthy: true},
		},
		Metadata: map[string]string{
			"canary_group":   "percentage-group",
			"canary_version": "canary",
		},
	}

	// 更新上游服务
	err = cb.UpdateUpstream(upstreamStable)
	if err != nil {
		t.Fatalf("Failed to update stable upstream: %v", err)
	}

	err = cb.UpdateUpstream(upstreamCanary)
	if err != nil {
		t.Fatalf("Failed to update canary upstream: %v", err)
	}

	// 测试百分比分配
	stableCount := 0
	canaryCount := 0
	totalRequests := 1000

	for i := 0; i < totalRequests; i++ {
		target, err := cb.Select(&types.Upstream{ID: "percentage-group"})
		if err != nil {
			t.Fatalf("Failed to select target: %v", err)
		}

		if target.Host == "stable-host" {
			stableCount++
		} else if target.Host == "canary-host" {
			canaryCount++
		}
	}

	// 验证百分比分配（允许5%的误差）
	stablePercentage := float64(stableCount) / float64(totalRequests) * 100
	canaryPercentage := float64(canaryCount) / float64(totalRequests) * 100

	if stablePercentage < 75 || stablePercentage > 85 {
		t.Errorf("Stable percentage should be around 80%%, got %.2f%%", stablePercentage)
	}

	if canaryPercentage < 15 || canaryPercentage > 25 {
		t.Errorf("Canary percentage should be around 20%%, got %.2f%%", canaryPercentage)
	}

	t.Logf("Stable: %.2f%%, Canary: %.2f%%", stablePercentage, canaryPercentage)
}

func TestCanaryBalancer_SingleUpstream(t *testing.T) {
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// 测试单个上游服务（非金丝雀组）
	upstream := &types.Upstream{
		ID:   "single-upstream",
		Name: "Single Service",
		Targets: []*types.Target{
			{Host: "single-host1", Port: 8080, Healthy: true},
			{Host: "single-host2", Port: 8080, Healthy: true},
		},
	}

	err := cb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to update single upstream: %v", err)
	}

	// 选择目标
	target, err := cb.Select(upstream)
	if err != nil {
		t.Fatalf("Failed to select target: %v", err)
	}

	if target.Host != "single-host1" && target.Host != "single-host2" {
		t.Errorf("Unexpected target host: %s", target.Host)
	}
}

func TestCanaryBalancer_NoHealthyTargets(t *testing.T) {
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// 创建金丝雀组配置
	canaryConfig := &CanaryConfig{
		GroupID:  "unhealthy-group",
		Strategy: "weighted",
		Versions: []*CanaryVersionConfig{
			{
				Version:    "v1",
				UpstreamID: "upstream-v1",
				Weight:     100,
			},
		},
	}

	err := cb.UpdateCanaryGroup(canaryConfig)
	if err != nil {
		t.Fatalf("Failed to update canary group: %v", err)
	}

	// 创建没有健康目标的上游服务
	upstream := &types.Upstream{
		ID:   "upstream-v1",
		Name: "Unhealthy Service",
		Targets: []*types.Target{
			{Host: "unhealthy-host", Port: 8080, Healthy: false},
		},
		Metadata: map[string]string{
			"canary_group":   "unhealthy-group",
			"canary_version": "v1",
		},
	}

	err = cb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to update upstream: %v", err)
	}

	// 尝试选择目标，应该失败
	_, err = cb.Select(&types.Upstream{ID: "unhealthy-group"})
	if err == nil {
		t.Error("Expected error when no healthy targets available")
	}
}

func TestCanaryBalancer_GetCanaryGroup(t *testing.T) {
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// 创建金丝雀组配置
	originalConfig := &CanaryConfig{
		GroupID:  "test-group",
		Strategy: "weighted",
		Versions: []*CanaryVersionConfig{
			{
				Version:    "v1",
				UpstreamID: "upstream-v1",
				Weight:     70,
				Percentage: 70.0,
			},
			{
				Version:    "v2",
				UpstreamID: "upstream-v2",
				Weight:     30,
				Percentage: 30.0,
			},
		},
	}

	err := cb.UpdateCanaryGroup(originalConfig)
	if err != nil {
		t.Fatalf("Failed to update canary group: %v", err)
	}

	// 获取金丝雀组配置
	retrievedConfig, err := cb.GetCanaryGroup("test-group")
	if err != nil {
		t.Fatalf("Failed to get canary group: %v", err)
	}

	if retrievedConfig.GroupID != originalConfig.GroupID {
		t.Errorf("Expected group ID %s, got %s", originalConfig.GroupID, retrievedConfig.GroupID)
	}

	if retrievedConfig.Strategy != originalConfig.Strategy {
		t.Errorf("Expected strategy %s, got %s", originalConfig.Strategy, retrievedConfig.Strategy)
	}

	if len(retrievedConfig.Versions) != len(originalConfig.Versions) {
		t.Errorf("Expected %d versions, got %d", len(originalConfig.Versions), len(retrievedConfig.Versions))
	}
}

func TestCanaryBalancer_Health(t *testing.T) {
	config := &config.Config{}
	cb := NewCanaryBalancer(config)

	// 创建金丝雀组配置
	canaryConfig := &CanaryConfig{
		GroupID:  "health-test-group",
		Strategy: "weighted",
		Versions: []*CanaryVersionConfig{
			{
				Version:    "v1",
				UpstreamID: "upstream-v1",
				Weight:     80,
			},
			{
				Version:    "v2",
				UpstreamID: "upstream-v2",
				Weight:     20,
			},
		},
	}

	err := cb.UpdateCanaryGroup(canaryConfig)
	if err != nil {
		t.Fatalf("Failed to update canary group: %v", err)
	}

	// 获取健康状态
	health := cb.Health()

	if health["type"] != "canary" {
		t.Errorf("Expected type 'canary', got %v", health["type"])
	}

	if health["groups_count"] != 1 {
		t.Errorf("Expected 1 group, got %v", health["groups_count"])
	}

	groups, ok := health["groups"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected groups to be a map")
	}

	if _, exists := groups["health-test-group"]; !exists {
		t.Error("Expected health-test-group to exist in health status")
	}
}
