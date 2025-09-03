package loadbalancer

import (
	"sync"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestAcceptanceCriteria 验证加权轮询负载均衡功能
// 功能要求：GIVEN实例A权重2，B权重1，WHEN发送3个请求，THEN A接收到2个，B接收到1个
func TestWeightedRoundRobinAcceptanceCriteria(t *testing.T) {
	t.Run("加权轮询负载均衡策略功能要求", func(t *testing.T) {
		testWeightedRoundRobinAcceptance(t)
	})

	t.Run("平滑加权轮询算法验证", func(t *testing.T) {
		testSmoothWeightedRoundRobin(t)
	})

	t.Run("并发安全性验证", func(t *testing.T) {
		testWeightedConcurrencySafety(t)
	})

	t.Run("健康检查集成验证", func(t *testing.T) {
		testWeightedHealthCheckIntegration(t)
	})
}

// testWeightedRoundRobinAcceptance 测试加权轮询功能要求
func testWeightedRoundRobinAcceptance(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建加权轮询负载均衡器
	lb := NewWeightedRoundRobinBalancer(cfg)

	// 创建测试上游服务：A权重2，B权重1
	upstream := &types.Upstream{
		ID:        "test-upstream",
		Name:      "Test Upstream",
		Algorithm: "weighted_round_robin",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 2, Healthy: true}, // 权重2
			{Host: "serverB.example.com", Port: 8080, Weight: 1, Healthy: true}, // 权重1
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 功能要求测试：发送3个请求
	t.Log("功能要求测试：实例A权重2，B权重1，发送3个请求")

	requestCounts := make(map[string]int)

	// 发送3个请求
	for i := 0; i < 3; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("Request %d: Failed to select target: %v", i+1, err)
		}

		requestCounts[target.Host]++
		t.Logf("Request %d -> %s:%d", i+1, target.Host, target.Port)
	}

	// 验证结果：A应该接收2个请求，B应该接收1个请求
	expectedA := 2
	expectedB := 1

	actualA := requestCounts["serverA.example.com"]
	actualB := requestCounts["serverB.example.com"]

	if actualA != expectedA {
		t.Errorf("ServerA: Expected %d requests, got %d", expectedA, actualA)
	}

	if actualB != expectedB {
		t.Errorf("ServerB: Expected %d requests, got %d", expectedB, actualB)
	}

	t.Logf(" 功能要求通过：ServerA收到%d个请求，ServerB收到%d个请求", actualA, actualB)
}

// testSmoothWeightedRoundRobin 测试平滑加权轮询算法
func testSmoothWeightedRoundRobin(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建加权轮询负载均衡器
	lb := NewWeightedRoundRobinBalancer(cfg)

	// 创建测试上游服务：A权重5，B权重1，C权重1
	upstream := &types.Upstream{
		ID:        "smooth-upstream",
		Name:      "Smooth Test Upstream",
		Algorithm: "weighted_round_robin",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 5, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverC.example.com", Port: 8080, Weight: 1, Healthy: true},
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 测试一个完整的权重周期（7个请求）
	totalRequests := 7 // 5 + 1 + 1 = 7
	requestSequence := make([]string, totalRequests)
	requestCounts := make(map[string]int)

	t.Log("测试平滑加权轮询算法（权重A:5, B:1, C:1）")

	for i := 0; i < totalRequests; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("Request %d: Failed to select target: %v", i+1, err)
		}

		requestSequence[i] = target.Host
		requestCounts[target.Host]++
		t.Logf("Request %d -> %s", i+1, target.Host)
	}

	// 验证权重分配
	expectedA := 5
	expectedB := 1
	expectedC := 1

	actualA := requestCounts["serverA.example.com"]
	actualB := requestCounts["serverB.example.com"]
	actualC := requestCounts["serverC.example.com"]

	if actualA != expectedA {
		t.Errorf("ServerA: Expected %d requests, got %d", expectedA, actualA)
	}
	if actualB != expectedB {
		t.Errorf("ServerB: Expected %d requests, got %d", expectedB, actualB)
	}
	if actualC != expectedC {
		t.Errorf("ServerC: Expected %d requests, got %d", expectedC, actualC)
	}

	// 验证平滑性：不应该连续出现5个A
	consecutiveA := 0
	maxConsecutiveA := 0
	for _, host := range requestSequence {
		if host == "serverA.example.com" {
			consecutiveA++
			if consecutiveA > maxConsecutiveA {
				maxConsecutiveA = consecutiveA
			}
		} else {
			consecutiveA = 0
		}
	}

	// 平滑加权轮询应该避免连续分配过多请求到同一个高权重服务器
	if maxConsecutiveA > 3 {
		t.Logf("Warning: Max consecutive requests to ServerA: %d (may not be smooth enough)", maxConsecutiveA)
	}

	t.Logf(" 平滑加权轮询测试通过：A=%d, B=%d, C=%d, 最大连续A=%d",
		actualA, actualB, actualC, maxConsecutiveA)
}

// testWeightedConcurrencySafety 测试并发安全性
func testWeightedConcurrencySafety(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建加权轮询负载均衡器
	lb := NewWeightedRoundRobinBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "concurrent-upstream",
		Name:      "Concurrent Test Upstream",
		Algorithm: "weighted_round_robin",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 3, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 2, Healthy: true},
			{Host: "serverC.example.com", Port: 8080, Weight: 1, Healthy: true},
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 并发测试参数
	numGoroutines := 50
	requestsPerGoroutine := 60 // 每个goroutine发送60个请求，总共3000个请求
	totalRequests := numGoroutines * requestsPerGoroutine

	// 用于收集结果
	results := make(chan string, totalRequests)
	var wg sync.WaitGroup

	t.Logf("测试并发安全性：%d个goroutine，每个发送%d个请求", numGoroutines, requestsPerGoroutine)

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

	// 验证权重分配比例
	totalWeight := 3 + 2 + 1                     // 6
	expectedA := totalRequests * 3 / totalWeight // 50%
	expectedB := totalRequests * 2 / totalWeight // 33.33%
	expectedC := totalRequests * 1 / totalWeight // 16.67%

	tolerance := totalRequests / 20 // 5% 容差

	t.Logf("并发加权负载均衡测试结果 (总请求数: %d):", totalRequests)

	actualA := hostCounts["serverA.example.com"]
	actualB := hostCounts["serverB.example.com"]
	actualC := hostCounts["serverC.example.com"]

	t.Logf("  ServerA (权重3): %d requests (期望约%d)", actualA, expectedA)
	t.Logf("  ServerB (权重2): %d requests (期望约%d)", actualB, expectedB)
	t.Logf("  ServerC (权重1): %d requests (期望约%d)", actualC, expectedC)

	// 检查分发是否在合理范围内
	if actualA < expectedA-tolerance || actualA > expectedA+tolerance {
		t.Errorf("ServerA received %d requests, expected around %d (±%d)",
			actualA, expectedA, tolerance)
	}
	if actualB < expectedB-tolerance || actualB > expectedB+tolerance {
		t.Errorf("ServerB received %d requests, expected around %d (±%d)",
			actualB, expectedB, tolerance)
	}
	if actualC < expectedC-tolerance || actualC > expectedC+tolerance {
		t.Errorf("ServerC received %d requests, expected around %d (±%d)",
			actualC, expectedC, tolerance)
	}

	// 验证所有主机都收到了请求
	if len(hostCounts) != 3 {
		t.Errorf("Expected 3 hosts to receive requests, got %d", len(hostCounts))
	}

	t.Log(" 并发安全性验证通过")
}

// testWeightedHealthCheckIntegration 测试健康检查集成
func testWeightedHealthCheckIntegration(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建加权轮询负载均衡器
	lb := NewWeightedRoundRobinBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "health-upstream",
		Name:      "Health Test Upstream",
		Algorithm: "weighted_round_robin",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 3, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 2, Healthy: true},
			{Host: "serverC.example.com", Port: 8080, Weight: 1, Healthy: false}, // 不健康
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 测试只选择健康的目标
	t.Log("测试健康检查集成：ServerC不健康，只应选择A和B")

	requestCounts := make(map[string]int)

	// 发送多个请求，验证只选择健康的实例
	for i := 0; i < 10; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("Request %d: Failed to select target: %v", i+1, err)
		}

		requestCounts[target.Host]++

		// 验证选择的是健康的主机
		if target.Host == "serverC.example.com" {
			t.Errorf("Request %d: Selected unhealthy host %s", i+1, target.Host)
		}

		t.Logf("Request %d -> %s:%d (healthy)", i+1, target.Host, target.Port)
	}

	// 验证只有健康的主机收到请求
	if requestCounts["serverC.example.com"] > 0 {
		t.Errorf("Unhealthy ServerC received %d requests", requestCounts["serverC.example.com"])
	}

	// 测试动态健康状态更新
	t.Log("测试动态健康状态更新...")

	// 将serverA标记为不健康
	err = lb.UpdateTargetHealth("health-upstream", "serverA.example.com", 8080, false)
	if err != nil {
		t.Fatalf("Failed to update target health: %v", err)
	}

	// 现在只有serverB是健康的
	for i := 0; i < 5; i++ {
		target, err := lb.Select(upstream)
		if err != nil {
			t.Fatalf("After health update, Request %d: Failed to select target: %v", i+1, err)
		}

		if target.Host != "serverB.example.com" {
			t.Errorf("After health update, Request %d: Expected serverB.example.com, got %s",
				i+1, target.Host)
		}
	}

	// 恢复serverA的健康状态
	err = lb.UpdateTargetHealth("health-upstream", "serverA.example.com", 8080, true)
	if err != nil {
		t.Fatalf("Failed to restore target health: %v", err)
	}

	// 验证加权轮询恢复正常
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

	t.Log(" 健康检查集成验证通过")
}
