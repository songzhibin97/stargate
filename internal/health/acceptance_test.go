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

// TestHealthCheckAcceptanceCriteria 验证健康检查的完整功能
// 功能要求1：能在指定时间内发现并隔离故障实例
// 功能要求2：能在实例恢复后将其重新纳入服务池
func TestHealthCheckAcceptanceCriteria(t *testing.T) {
	t.Log("开始验证功能功能要求")

	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建可控制的模拟服务器
	var serverHealthy bool = true
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
	host, port := parseAcceptanceServerAddress(server.URL)

	// 创建上游服务
	upstream := &types.Upstream{
		ID:   "acceptance-upstream",
		Name: "Acceptance Test Upstream",
		Targets: []*types.Target{
			{Host: host, Port: port, Healthy: true}, // 初始标记为健康
		},
		HealthCheck: &types.HealthCheck{
			Type:               "http",
			Path:               "/",
			Interval:           1, // 1秒检查一次
			Timeout:            2, // 2秒超时
			HealthyThreshold:   1, // 1次成功标记为健康
			UnhealthyThreshold: 2, // 2次失败标记为不健康
		},
	}

	// 用于跟踪健康状态变化
	var changesMu sync.Mutex
	healthChanges := make([]AcceptanceHealthChangeEvent, 0)

	// 添加健康状态变化回调
	checker.AddHealthChangeCallback(func(upstreamID string, target *types.Target, healthy bool) {
		changesMu.Lock()
		defer changesMu.Unlock()
		healthChanges = append(healthChanges, AcceptanceHealthChangeEvent{
			UpstreamID: upstreamID,
			Target:     target,
			Healthy:    healthy,
			Timestamp:  time.Now(),
		})
		t.Logf("健康状态变化: %s:%d -> %v", target.Host, target.Port, healthy)
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

	// 功能要求1：能在指定时间内发现并隔离故障实例
	t.Log("功能要求1：测试故障实例发现和隔离")

	// 让服务器变为故障状态
	mu.Lock()
	serverHealthy = false
	mu.Unlock()
	t.Log("模拟服务器故障")

	// 等待故障检测（需要2次失败，每次间隔1秒，所以至少3秒）
	time.Sleep(4 * time.Second)

	// 验证故障被检测到
	changesMu.Lock()
	foundFailure := false
	for _, change := range healthChanges {
		if !change.Healthy {
			foundFailure = true
			t.Logf("功能要求1通过：故障实例在 %v 被成功发现和隔离",
				change.Timestamp.Format("15:04:05"))
			break
		}
	}
	changesMu.Unlock()

	if !foundFailure {
		t.Error("功能要求1失败：故障实例未被发现和隔离")
		return
	}

	// 验证当前健康状态
	healthStatus := checker.GetUpstreamHealth("acceptance-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	targets := healthStatus["targets"].([]map[string]interface{})
	if len(targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(targets))
	}

	if targets[0]["healthy"].(bool) {
		t.Error("功能要求1失败：故障实例仍被标记为健康")
		return
	}

	t.Log("功能要求1完全通过：故障实例已被隔离")

	// 功能要求2：能在实例恢复后将其重新纳入服务池
	t.Log("功能要求2：测试实例恢复后重新纳入")

	// 修复服务器
	mu.Lock()
	serverHealthy = true
	mu.Unlock()
	t.Log("模拟服务器恢复")

	// 等待恢复检测（需要1次成功，间隔1秒，所以至少2秒）
	time.Sleep(3 * time.Second)

	// 验证恢复被检测到
	changesMu.Lock()
	foundRecovery := false
	for i := len(healthChanges) - 1; i >= 0; i-- {
		change := healthChanges[i]
		if change.Healthy {
			foundRecovery = true
			t.Logf("功能要求2通过：实例恢复在 %v 被成功检测并重新纳入",
				change.Timestamp.Format("15:04:05"))
			break
		}
	}
	changesMu.Unlock()

	if !foundRecovery {
		t.Error("功能要求2失败：实例恢复未被检测")
		return
	}

	// 验证当前健康状态
	healthStatus = checker.GetUpstreamHealth("acceptance-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	targets = healthStatus["targets"].([]map[string]interface{})
	if len(targets) != 1 {
		t.Fatalf("Expected 1 target, got %d", len(targets))
	}

	if !targets[0]["healthy"].(bool) {
		t.Error("功能要求2失败：恢复的实例未被标记为健康")
		return
	}

	t.Log("功能要求2完全通过：实例已重新纳入服务池")

	// 总结功能要求验证结果
	changesMu.Lock()
	totalChanges := len(healthChanges)
	changesMu.Unlock()

	t.Logf("健康状态变化总数: %d", totalChanges)
	t.Log("功能要求完全通过！")
	t.Log("功能要求1：能在指定时间内发现并隔离故障实例 - 通过")
	t.Log("功能要求2：能在实例恢复后将其重新纳入服务池 - 通过")
}

// TestHealthCheckerIntegrationWithLoadBalancer 测试健康检查器与负载均衡器的集成
func TestHealthCheckerIntegrationWithLoadBalancer(t *testing.T) {
	t.Log("🔗 测试健康检查器与负载均衡器集成")

	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建健康和故障服务器
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
	healthyHost, healthyPort := parseAcceptanceServerAddress(healthyServer.URL)
	faultyHost, faultyPort := parseAcceptanceServerAddress(faultyServer.URL)

	// 创建上游服务
	upstream := &types.Upstream{
		ID:   "integration-upstream",
		Name: "Integration Test Upstream",
		Targets: []*types.Target{
			{Host: healthyHost, Port: healthyPort, Healthy: true},
			{Host: faultyHost, Port: faultyPort, Healthy: true}, // 初始标记为健康
		},
		HealthCheck: &types.HealthCheck{
			Type:               "http",
			Path:               "/",
			Interval:           1, // 1秒检查一次
			Timeout:            2, // 2秒超时
			HealthyThreshold:   1, // 1次成功标记为健康
			UnhealthyThreshold: 1, // 1次失败标记为不健康
		},
	}

	// 模拟负载均衡器回调
	var callbackMu sync.Mutex
	callbackCalls := 0

	checker.AddHealthChangeCallback(func(upstreamID string, target *types.Target, healthy bool) {
		callbackMu.Lock()
		defer callbackMu.Unlock()
		callbackCalls++
		t.Logf("🔄 负载均衡器收到健康状态变化通知: %s:%d -> %v",
			target.Host, target.Port, healthy)
	})

	// 启动健康检查
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

	// 验证回调被调用
	callbackMu.Lock()
	totalCallbacks := callbackCalls
	callbackMu.Unlock()

	if totalCallbacks == 0 {
		t.Error(" 集成测试失败：负载均衡器未收到健康状态变化通知")
		return
	}

	t.Logf(" 集成测试通过：负载均衡器收到 %d 次健康状态变化通知", totalCallbacks)

	// 验证最终健康状态
	healthStatus := checker.GetUpstreamHealth("integration-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	healthyTargets := healthStatus["healthy_targets"].(int)
	totalTargets := healthStatus["total_targets"].(int)

	t.Logf("最终状态：%d/%d 个目标健康", healthyTargets, totalTargets)

	if healthyTargets == 0 {
		t.Error(" 集成测试失败：没有健康的目标")
		return
	}

	t.Log(" 健康检查器与负载均衡器集成测试通过")
}

// AcceptanceHealthChangeEvent 健康状态变化事件
type AcceptanceHealthChangeEvent struct {
	UpstreamID string
	Target     *types.Target
	Healthy    bool
	Timestamp  time.Time
}

// parseAcceptanceServerAddress 解析服务器地址
func parseAcceptanceServerAddress(url string) (string, int) {
	url = strings.TrimPrefix(url, "http://")
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "localhost", 8080
	}

	host := parts[0]
	port := 8080
	if len(parts) > 1 {
		for i, c := range parts[1] {
			if c < '0' || c > '9' {
				parts[1] = parts[1][:i]
				break
			}
		}
		if parts[1] != "" {
			var p int
			if n, err := fmt.Sscanf(parts[1], "%d", &p); n == 1 && err == nil {
				port = p
			}
		}
	}

	return host, port
}
