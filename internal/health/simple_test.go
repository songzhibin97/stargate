package health

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestBasicHealthCheck 基本健康检查功能测试
func TestBasicHealthCheck(t *testing.T) {
	// 创建健康检查器
	cfg := &config.Config{}
	checker := NewActiveHealthChecker(cfg)

	// 创建健康的模拟服务器
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	}))
	defer healthyServer.Close()

	// 创建故障的模拟服务器
	faultyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error"))
	}))
	defer faultyServer.Close()

	// 解析服务器地址
	healthyHost, healthyPort := parseSimpleServerAddress(healthyServer.URL)
	faultyHost, faultyPort := parseSimpleServerAddress(faultyServer.URL)

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
			Path:               "/",
			Interval:           1, // 1秒检查一次
			Timeout:            2, // 2秒超时
			HealthyThreshold:   1, // 1次成功标记为健康
			UnhealthyThreshold: 1, // 1次失败标记为不健康
		},
	}

	// 添加上游服务
	err := checker.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 启动健康检查器
	err = checker.Start()
	if err != nil {
		t.Fatalf("Failed to start health checker: %v", err)
	}

	// 等待健康检查
	time.Sleep(3 * time.Second)

	// 停止健康检查器
	err = checker.Stop()
	if err != nil {
		t.Fatalf("Failed to stop health checker: %v", err)
	}

	// 验证健康状态
	healthStatus := checker.GetUpstreamHealth("test-upstream")
	if healthStatus == nil {
		t.Fatal("Failed to get upstream health status")
	}

	targets := healthStatus["targets"].([]map[string]interface{})
	if len(targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(targets))
	}

	// 验证至少有一个目标是健康的（健康服务器）
	healthyCount := 0
	for _, target := range targets {
		if target["healthy"].(bool) {
			healthyCount++
		}
	}

	if healthyCount == 0 {
		t.Error("Expected at least one healthy target")
	}

	t.Logf(" 基本健康检查测试通过：%d个健康目标", healthyCount)
}

// TestHealthCheckConfiguration 测试健康检查配置
func TestHealthCheckConfiguration(t *testing.T) {
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

	host, port := parseSimpleServerAddress(server.URL)

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

	// 添加上游服务
	err := checker.AddUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 启动健康检查器
	err = checker.Start()
	if err != nil {
		t.Fatalf("Failed to start health checker: %v", err)
	}

	// 等待健康检查
	time.Sleep(2 * time.Second)

	// 停止健康检查器
	err = checker.Stop()
	if err != nil {
		t.Fatalf("Failed to stop health checker: %v", err)
	}

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

// parseSimpleServerAddress 解析服务器地址
func parseSimpleServerAddress(url string) (string, int) {
	// 简化的地址解析
	url = strings.TrimPrefix(url, "http://")
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "localhost", 8080
	}

	host := parts[0]
	port := 8080
	if len(parts) > 1 {
		// 简单解析端口号
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
