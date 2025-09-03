package health

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestAcceptanceCriteria 验证健康检查功能
// 功能要求：能在指定时间内发现并隔离故障实例，能在实例恢复后将其重新纳入服务池
func TestActiveHealthCheckerAcceptanceCriteria(t *testing.T) {
	t.Run("故障实例发现和隔离", func(t *testing.T) {
		testFailureDetectionAndIsolation(t)
	})

	t.Run("实例恢复后重新纳入", func(t *testing.T) {
		testInstanceRecoveryAndReintegration(t)
	})

	t.Run("并发健康检查", func(t *testing.T) {
		testConcurrentHealthChecks(t)
	})

	t.Run("健康检查配置管理", func(t *testing.T) {
		testHealthCheckConfiguration(t)
	})
}

// testFailureDetectionAndIsolation 测试故障实例发现和隔离
func testFailureDetectionAndIsolation(t *testing.T) {
	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建模拟服务器：一个健康，一个故障
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))
	defer healthyServer.Close()

	faultyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer faultyServer.Close()

	// 解析服务器地址
	healthyHost, healthyPort := parseServerAddress(healthyServer.URL)
	faultyHost, faultyPort := parseServerAddress(faultyServer.URL)

	// 创建上游服务
	upstream := &types.Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []*types.Target{
			{Host: healthyHost, Port: healthyPort, Healthy: true},
			{Host: faultyHost, Port: faultyPort, Healthy: true}, // 初始标记为健康
		},
		HealthCheck: &types.HealthCheck{
			Type:               "http",
			Path:               "/health",
			Interval:           1, // 1秒检查一次
			Timeout:            2, // 2秒超时
			HealthyThreshold:   1, // 1次成功标记为健康
			UnhealthyThreshold: 2, // 2次失败标记为不健康
		},
	}

	// 用于跟踪健康状态变化
	var mu sync.Mutex
	healthChanges := make([]HealthChangeEvent, 0)

	// 添加健康状态变化回调
	checker.AddHealthChangeCallback(func(upstreamID string, target *types.Target, healthy bool) {
		mu.Lock()
		defer mu.Unlock()
		healthChanges = append(healthChanges, HealthChangeEvent{
			UpstreamID: upstreamID,
			Target:     target,
			Healthy:    healthy,
			Timestamp:  time.Now(),
		})
		t.Logf("Health change: %s:%d -> %v", target.Host, target.Port, healthy)
	})

	// 添加上游服务并启动健康检查
	err := checker.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	err = checker.Start()
	if err != nil {
		t.Fatalf("Failed to start health checker: %v", err)
	}
	defer checker.Stop()

	// 等待足够的时间让健康检查器检测到故障
	// 需要至少 2 * interval 时间来触发 unhealthy_threshold
	time.Sleep(5 * time.Second)

	// 验证健康状态变化
	mu.Lock()
	defer mu.Unlock()

	t.Logf("Total health changes: %d", len(healthChanges))

	// 应该有至少一个健康状态变化（故障服务器变为不健康）
	if len(healthChanges) == 0 {
		t.Error("Expected at least one health change, got none")
		return
	}

	// 查找故障服务器的健康状态变化
	faultyServerFound := false
	for _, change := range healthChanges {
		if change.Target.Host == faultyHost && change.Target.Port == faultyPort {
			if !change.Healthy {
				faultyServerFound = true
				t.Logf("✓ Faulty server %s:%d correctly marked as unhealthy",
					faultyHost, faultyPort)
				break
			}
		}
	}

	if !faultyServerFound {
		t.Error("Faulty server was not marked as unhealthy")
	}

	// 验证上游服务的健康状态
	healthStatus := checker.GetUpstreamHealth("test-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	targets := healthStatus["targets"].([]map[string]interface{})
	healthyCount := 0
	for _, target := range targets {
		if target["healthy"].(bool) {
			healthyCount++
		}
	}

	// 应该只有一个健康的目标（健康服务器）
	if healthyCount != 1 {
		t.Errorf("Expected 1 healthy target, got %d", healthyCount)
	}

	t.Log(" 故障实例发现和隔离测试通过")
}

// testInstanceRecoveryAndReintegration 测试实例恢复后重新纳入
func testInstanceRecoveryAndReintegration(t *testing.T) {
	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建可控制的模拟服务器
	var serverHealthy bool = false
	var mu sync.RWMutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		healthy := serverHealthy
		mu.RUnlock()

		if healthy {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		}
	}))
	defer server.Close()

	// 解析服务器地址
	host, port := parseServerAddress(server.URL)

	// 创建上游服务
	upstream := &types.Upstream{
		ID:   "recovery-upstream",
		Name: "Recovery Test Upstream",
		Targets: []*types.Target{
			{Host: host, Port: port, Healthy: true}, // 初始标记为健康
		},
		HealthCheck: &types.HealthCheck{
			Type:               "http",
			Path:               "/",
			Interval:           1, // 1秒检查一次
			Timeout:            2, // 2秒超时
			HealthyThreshold:   2, // 2次成功标记为健康
			UnhealthyThreshold: 2, // 2次失败标记为不健康
		},
	}

	// 用于跟踪健康状态变化
	var changesMu sync.Mutex
	healthChanges := make([]HealthChangeEvent, 0)

	// 添加健康状态变化回调
	checker.AddHealthChangeCallback(func(upstreamID string, target *types.Target, healthy bool) {
		changesMu.Lock()
		defer changesMu.Unlock()
		healthChanges = append(healthChanges, HealthChangeEvent{
			UpstreamID: upstreamID,
			Target:     target,
			Healthy:    healthy,
			Timestamp:  time.Now(),
		})
		t.Logf("Health change: %s:%d -> %v", target.Host, target.Port, healthy)
	})

	// 添加上游服务并启动健康检查
	err := checker.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	err = checker.Start()
	if err != nil {
		t.Fatalf("Failed to start health checker: %v", err)
	}
	defer checker.Stop()

	// 阶段1：等待服务器被标记为不健康（初始状态是故障）
	t.Log("Phase 1: Waiting for server to be marked as unhealthy...")
	time.Sleep(4 * time.Second)

	// 阶段2：修复服务器
	t.Log("Phase 2: Fixing server...")
	mu.Lock()
	serverHealthy = true
	mu.Unlock()

	// 等待服务器被标记为健康
	t.Log("Phase 3: Waiting for server to be marked as healthy...")
	time.Sleep(4 * time.Second)

	// 验证健康状态变化
	changesMu.Lock()
	defer changesMu.Unlock()

	t.Logf("Total health changes: %d", len(healthChanges))

	// 应该有两个健康状态变化：健康->不健康->健康
	if len(healthChanges) < 2 {
		t.Errorf("Expected at least 2 health changes, got %d", len(healthChanges))
		return
	}

	// 验证状态变化序列
	foundUnhealthy := false
	foundHealthyAgain := false

	for _, change := range healthChanges {
		if !change.Healthy {
			foundUnhealthy = true
			t.Logf("✓ Server correctly marked as unhealthy")
		} else if foundUnhealthy {
			foundHealthyAgain = true
			t.Logf("✓ Server correctly marked as healthy again")
		}
	}

	if !foundUnhealthy {
		t.Error("Server was not marked as unhealthy")
	}

	if !foundHealthyAgain {
		t.Error("Server was not marked as healthy again after recovery")
	}

	t.Log(" 实例恢复后重新纳入测试通过")
}

// testConcurrentHealthChecks 测试并发健康检查
func testConcurrentHealthChecks(t *testing.T) {
	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建多个模拟服务器
	numServers := 5
	servers := make([]*httptest.Server, numServers)
	targets := make([]*types.Target, numServers)

	for i := 0; i < numServers; i++ {
		serverID := i
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 模拟不同的响应时间
			time.Sleep(time.Duration(serverID*100) * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("server-%d", serverID)))
		}))
		servers[i] = server
		defer server.Close()

		host, port := parseServerAddress(server.URL)
		targets[i] = &types.Target{
			Host:    host,
			Port:    port,
			Healthy: true,
		}
	}

	// 创建上游服务
	upstream := &types.Upstream{
		ID:      "concurrent-upstream",
		Name:    "Concurrent Test Upstream",
		Targets: targets,
		HealthCheck: &types.HealthCheck{
			Type:               "http",
			Path:               "/",
			Interval:           2, // 2秒检查一次
			Timeout:            5, // 5秒超时
			HealthyThreshold:   1, // 1次成功标记为健康
			UnhealthyThreshold: 1, // 1次失败标记为不健康
		},
	}

	// 添加上游服务并启动健康检查
	err := checker.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	err = checker.Start()
	if err != nil {
		t.Fatalf("Failed to start health checker: %v", err)
	}
	defer checker.Stop()

	// 等待几轮健康检查
	time.Sleep(6 * time.Second)

	// 验证所有服务器都保持健康状态
	healthStatus := checker.GetUpstreamHealth("concurrent-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	targets_status := healthStatus["targets"].([]map[string]interface{})
	healthyCount := 0
	for _, target := range targets_status {
		if target["healthy"].(bool) {
			healthyCount++
		}
	}

	if healthyCount != numServers {
		t.Errorf("Expected %d healthy targets, got %d", numServers, healthyCount)
	}

	t.Log(" 并发健康检查测试通过")
}

// testHealthCheckConfiguration 测试健康检查配置管理
func testHealthCheckConfiguration(t *testing.T) {
	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/custom-health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("custom healthy"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	host, port := parseServerAddress(server.URL)

	// 创建带自定义配置的上游服务
	upstream := &types.Upstream{
		ID:   "config-upstream",
		Name: "Configuration Test Upstream",
		Targets: []*types.Target{
			{Host: host, Port: port, Healthy: true},
		},
		HealthCheck: &types.HealthCheck{
			Type:               "http",
			Path:               "/custom-health", // 自定义健康检查路径
			Interval:           1,                // 1秒检查一次
			Timeout:            3,                // 3秒超时
			HealthyThreshold:   1,                // 1次成功标记为健康
			UnhealthyThreshold: 1,                // 1次失败标记为不健康
		},
	}

	// 添加上游服务并启动健康检查
	err := checker.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	err = checker.Start()
	if err != nil {
		t.Fatalf("Failed to start health checker: %v", err)
	}
	defer checker.Stop()

	// 等待健康检查
	time.Sleep(3 * time.Second)

	// 验证健康状态
	healthStatus := checker.GetUpstreamHealth("config-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	targets := healthStatus["targets"].([]map[string]interface{})
	if len(targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(targets))
	}

	if !targets[0]["healthy"].(bool) {
		t.Error("Target should be healthy with custom configuration")
	}

	t.Log(" 健康检查配置管理测试通过")
}

// HealthChangeEvent 健康状态变化事件
type HealthChangeEvent struct {
	UpstreamID string
	Target     *types.Target
	Healthy    bool
	Timestamp  time.Time
}

// parseServerAddress 解析服务器地址
func parseServerAddress(url string) (string, int) {
	// 简化的地址解析，实际应该使用更健壮的方法
	url = strings.TrimPrefix(url, "http://")
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "localhost", 8080
	}

	host := parts[0]
	port := 8080
	fmt.Sscanf(parts[1], "%d", &port)

	return host, port
}
