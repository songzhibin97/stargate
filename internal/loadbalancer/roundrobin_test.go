package loadbalancer

import (
	"sync"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestAcceptanceCriteria 验证轮询负载均衡功能
// 功能要求：请求被依次、循环地分发到每个健康的后端实例
func TestRoundRobinAcceptanceCriteria(t *testing.T) {
	t.Run("轮询负载均衡策略功能要求", func(t *testing.T) {
		testRoundRobinAcceptance(t)
	})

	t.Run("并发安全性功能要求", func(t *testing.T) {
		testConcurrencySafety(t)
	})

	t.Run("健康检查集成功能要求", func(t *testing.T) {
		testHealthCheckIntegration(t)
	})
}

// testRoundRobinAcceptance 测试轮询分发逻辑
func testRoundRobinAcceptance(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建负载均衡器
	lb := NewRoundRobinBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "test-upstream",
		Name:      "Test Upstream",
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{Host: "server1.example.com", Port: 8080, Weight: 100, Healthy: true},
			{Host: "server2.example.com", Port: 8080, Weight: 100, Healthy: true},
			{Host: "server3.example.com", Port: 8080, Weight: 100, Healthy: true},
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 测试轮询分发
	expectedHosts := []string{"server1.example.com", "server2.example.com", "server3.example.com"}

	// 进行多轮测试，确保轮询循环正确
	for round := 0; round < 3; round++ {
		t.Logf("Round %d:", round+1)
		for i, expectedHost := range expectedHosts {
			target, err := lb.Select(upstream)
			if err != nil {
				t.Fatalf("Round %d, Request %d: Failed to select target: %v", round+1, i+1, err)
			}

			if target.Host != expectedHost {
				t.Errorf("Round %d, Request %d: Expected host %s, got %s",
					round+1, i+1, expectedHost, target.Host)
			}

			t.Logf("  Request %d -> %s:%d ✓", i+1, target.Host, target.Port)
		}
	}

	t.Log(" 轮询负载均衡策略功能要求通过")
}

// testConcurrencySafety 测试并发安全性
func testConcurrencySafety(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建负载均衡器
	lb := NewRoundRobinBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "concurrent-upstream",
		Name:      "Concurrent Test Upstream",
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{Host: "server1.example.com", Port: 8080, Weight: 100, Healthy: true},
			{Host: "server2.example.com", Port: 8080, Weight: 100, Healthy: true},
			{Host: "server3.example.com", Port: 8080, Weight: 100, Healthy: true},
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 并发测试参数
	numGoroutines := 100
	requestsPerGoroutine := 100
	totalRequests := numGoroutines * requestsPerGoroutine

	// 用于收集结果
	results := make(chan string, totalRequests)
	var wg sync.WaitGroup

	// 启动多个goroutine并发请求
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				target, err := lb.Select(upstream)
				if err != nil {
					t.Errorf("Goroutine %d, Request %d: Failed to select target: %v",
						goroutineID, j, err)
					return
				}
				results <- target.Host
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(results)

	// 统计结果
	hostCounts := make(map[string]int)
	for host := range results {
		hostCounts[host]++
	}

	// 验证分发是否相对均匀
	expectedCount := totalRequests / len(upstream.Targets)
	tolerance := expectedCount / 10 // 10% 容差

	t.Logf("并发测试结果 (总请求数: %d):", totalRequests)
	for host, count := range hostCounts {
		t.Logf("  %s: %d requests", host, count)

		// 检查分发是否在合理范围内
		if count < expectedCount-tolerance || count > expectedCount+tolerance {
			t.Errorf("Host %s received %d requests, expected around %d (±%d)",
				host, count, expectedCount, tolerance)
		}
	}

	// 验证所有主机都收到了请求
	if len(hostCounts) != len(upstream.Targets) {
		t.Errorf("Expected %d hosts to receive requests, got %d",
			len(upstream.Targets), len(hostCounts))
	}

	t.Log(" 并发安全性功能要求通过")
}

// testHealthCheckIntegration 测试健康检查集成
func testHealthCheckIntegration(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建负载均衡器
	lb := NewRoundRobinBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "health-upstream",
		Name:      "Health Test Upstream",
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{Host: "server1.example.com", Port: 8080, Weight: 100, Healthy: true},
			{Host: "server2.example.com", Port: 8080, Weight: 100, Healthy: true},
			{Host: "server3.example.com", Port: 8080, Weight: 100, Healthy: false}, // 不健康
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 测试只选择健康的目标
	healthyHosts := []string{"server1.example.com", "server2.example.com"}

	for i := 0; i < 10; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("Request %d: Failed to select target: %v", i+1, err)
		}

		// 验证选择的是健康的主机
		isHealthy := false
		for _, healthyHost := range healthyHosts {
			if target.Host == healthyHost {
				isHealthy = true
				break
			}
		}

		if !isHealthy {
			t.Errorf("Request %d: Selected unhealthy host %s", i+1, target.Host)
		}

		t.Logf("Request %d -> %s:%d (healthy) ✓", i+1, target.Host, target.Port)
	}

	// 测试动态健康状态更新
	t.Log("Testing dynamic health status updates...")

	// 将server1标记为不健康
	err = lb.UpdateTargetHealth("health-upstream", "server1.example.com", 8080, false)
	if err != nil {
		t.Fatalf("Failed to update target health: %v", err)
	}

	// 现在只有server2是健康的
	for i := 0; i < 5; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("After health update, Request %d: Failed to select target: %v", i+1, err)
		}

		if target.Host != "server2.example.com" {
			t.Errorf("After health update, Request %d: Expected server2.example.com, got %s",
				i+1, target.Host)
		}
	}

	// 恢复server1的健康状态
	err = lb.UpdateTargetHealth("health-upstream", "server1.example.com", 8080, true)
	if err != nil {
		t.Fatalf("Failed to restore target health: %v", err)
	}

	// 验证轮询恢复正常
	hostsSeen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("After health restore, Request %d: Failed to select target: %v", i+1, err)
		}
		hostsSeen[target.Host] = true
	}

	// 应该看到两个健康的主机
	if len(hostsSeen) != 2 {
		t.Errorf("After health restore: Expected to see 2 healthy hosts, saw %d: %v",
			len(hostsSeen), hostsSeen)
	}

	t.Log(" 健康检查集成功能要求通过")
}
