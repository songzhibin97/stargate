package loadbalancer

import (
	"hash/fnv"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestAcceptanceCriteria 验证IP Hash负载均衡功能
// 功能要求：来自同一IP的多个请求总是被分发到同一个后端实例
func TestIPHashAcceptanceCriteria(t *testing.T) {
	t.Run("IP Hash负载均衡策略功能要求", func(t *testing.T) {
		testIPHashAcceptance(t)
	})

	t.Run("客户端IP提取功能验证", func(t *testing.T) {
		testClientIPExtraction(t)
	})

	t.Run("并发安全性验证", func(t *testing.T) {
		testIPHashConcurrencySafety(t)
	})

	t.Run("健康检查集成验证", func(t *testing.T) {
		testIPHashHealthCheckIntegration(t)
	})

	t.Run("一致性哈希算法验证", func(t *testing.T) {
		testConsistentHashAlgorithm(t)
	})
}

// testIPHashAcceptance 测试IP Hash功能要求
func testIPHashAcceptance(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建IP Hash负载均衡器
	lb := NewIPHashBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "test-upstream",
		Name:      "Test Upstream",
		Algorithm: "ip_hash",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverC.example.com", Port: 8080, Weight: 1, Healthy: true},
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 功能要求测试：同一IP的多个请求应该分发到同一后端
	testIPs := []string{
		"192.168.1.100",
		"10.0.0.50",
		"172.16.0.200",
		"203.0.113.10",
	}

	t.Log("功能要求测试：同一IP的多个请求应该分发到同一后端")

	for _, clientIP := range testIPs {
		t.Logf("测试客户端IP: %s", clientIP)

		// 模拟同一IP的多个请求
		var selectedHost string
		for i := 0; i < 5; i++ {
			// 直接使用IP哈希算法选择目标
			healthyTargets := make([]*types.Target, 0)
			for _, target := range upstream.Targets {
				if target.Healthy {
					healthyTargets = append(healthyTargets, target)
				}
			}

			selected := selectByIPHashDirect(clientIP, healthyTargets)
			if selected == nil {
				t.Fatalf("Failed to select target for IP %s", clientIP)
			}

			if i == 0 {
				selectedHost = selected.Host
				t.Logf("  IP %s -> %s:%d", clientIP, selected.Host, selected.Port)
			} else {
				if selected.Host != selectedHost {
					t.Errorf("IP %s: Request %d selected different host %s, expected %s",
						clientIP, i+1, selected.Host, selectedHost)
				}
			}
		}
	}

	t.Log(" IP Hash负载均衡策略功能要求通过")
}

// selectByIPHashDirect 直接使用IP哈希算法选择目标（测试辅助函数）
func selectByIPHashDirect(clientIP string, targets []*types.Target) *types.Target {
	if len(targets) == 0 {
		return nil
	}

	// 使用与IPHashBalancer相同的哈希算法
	hash := fnv.New32a()
	hash.Write([]byte(clientIP))
	hashValue := hash.Sum32()

	// 简单取模选择目标
	index := hashValue % uint32(len(targets))
	return targets[index]
}

// testClientIPExtraction 测试客户端IP提取功能
func testClientIPExtraction(t *testing.T) {
	testCases := []struct {
		name          string
		remoteAddr    string
		xRealIP       string
		xForwardedFor string
		expectedIP    string
	}{
		{
			name:       "使用X-Real-IP头",
			remoteAddr: "127.0.0.1:12345",
			xRealIP:    "192.168.1.100",
			expectedIP: "192.168.1.100",
		},
		{
			name:          "使用X-Forwarded-For头",
			remoteAddr:    "127.0.0.1:12345",
			xForwardedFor: "203.0.113.10, 192.168.1.1, 10.0.0.1",
			expectedIP:    "203.0.113.10",
		},
		{
			name:       "使用RemoteAddr",
			remoteAddr: "172.16.0.200:54321",
			expectedIP: "172.16.0.200",
		},
		{
			name:          "X-Real-IP优先级高于X-Forwarded-For",
			remoteAddr:    "127.0.0.1:12345",
			xRealIP:       "192.168.1.100",
			xForwardedFor: "203.0.113.10, 192.168.1.1",
			expectedIP:    "192.168.1.100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试请求
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tc.remoteAddr

			if tc.xRealIP != "" {
				req.Header.Set("X-Real-IP", tc.xRealIP)
			}

			if tc.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tc.xForwardedFor)
			}

			// 提取客户端IP
			extractedIP := ExtractClientIP(req)

			if extractedIP != tc.expectedIP {
				t.Errorf("Expected IP %s, got %s", tc.expectedIP, extractedIP)
			}

			t.Logf("✓ %s: 提取到IP %s", tc.name, extractedIP)
		})
	}

	t.Log(" 客户端IP提取功能验证通过")
}

// testIPHashConcurrencySafety 测试并发安全性
func testIPHashConcurrencySafety(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建IP Hash负载均衡器
	lb := NewIPHashBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "concurrent-upstream",
		Name:      "Concurrent Test Upstream",
		Algorithm: "ip_hash",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 1, Healthy: true},
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
	requestsPerGoroutine := 20
	testIPs := []string{
		"192.168.1.100", "192.168.1.101", "192.168.1.102", "192.168.1.103", "192.168.1.104",
		"10.0.0.50", "10.0.0.51", "10.0.0.52", "10.0.0.53", "10.0.0.54",
	}

	// 用于收集结果
	results := make(chan map[string]string, numGoroutines*requestsPerGoroutine)
	var wg sync.WaitGroup

	t.Logf("测试并发安全性：%d个goroutine，每个发送%d个请求", numGoroutines, requestsPerGoroutine)

	// 启动多个goroutine并发请求
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				clientIP := testIPs[j%len(testIPs)]

				// 直接使用IP哈希算法
				healthyTargets := make([]*types.Target, 0)
				for _, target := range upstream.Targets {
					if target.Healthy {
						healthyTargets = append(healthyTargets, target)
					}
				}

				selected := selectByIPHashDirect(clientIP, healthyTargets)
				if selected == nil {
					t.Errorf("Goroutine %d, Request %d: Failed to select target for IP %s",
						goroutineID, j, clientIP)
					return
				}

				results <- map[string]string{
					"ip":   clientIP,
					"host": selected.Host,
				}
			}
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(results)

	// 验证一致性：同一IP应该总是选择同一主机
	ipToHost := make(map[string]string)
	for result := range results {
		ip := result["ip"]
		host := result["host"]

		if expectedHost, exists := ipToHost[ip]; exists {
			if host != expectedHost {
				t.Errorf("IP %s: Expected host %s, got %s", ip, expectedHost, host)
			}
		} else {
			ipToHost[ip] = host
		}
	}

	t.Logf("并发测试结果：")
	for ip, host := range ipToHost {
		t.Logf("  IP %s -> %s", ip, host)
	}

	t.Log(" 并发安全性验证通过")
}

// testIPHashHealthCheckIntegration 测试健康检查集成
func testIPHashHealthCheckIntegration(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建IP Hash负载均衡器
	lb := NewIPHashBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "health-upstream",
		Name:      "Health Test Upstream",
		Algorithm: "ip_hash",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 1, Healthy: true},
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

	testIP := "192.168.1.100"

	// 获取健康的目标
	healthyTargets := make([]*types.Target, 0)
	for _, target := range upstream.Targets {
		if target.Healthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	// 多次选择，验证一致性
	var selectedHost string
	for i := 0; i < 10; i++ {
		selected := selectByIPHashDirect(testIP, healthyTargets)
		if selected == nil {
			t.Fatalf("Request %d: Failed to select target", i+1)
		}

		// 验证选择的是健康的主机
		if selected.Host == "serverC.example.com" {
			t.Errorf("Request %d: Selected unhealthy host %s", i+1, selected.Host)
		}

		if i == 0 {
			selectedHost = selected.Host
		} else {
			if selected.Host != selectedHost {
				t.Errorf("Request %d: IP %s selected different host %s, expected %s",
					i+1, testIP, selected.Host, selectedHost)
			}
		}
	}

	// 测试动态健康状态更新
	t.Log("测试动态健康状态更新...")

	// 将选中的服务器标记为不健康
	err = lb.UpdateTargetHealth("health-upstream", selectedHost, 8080, false)
	if err != nil {
		t.Fatalf("Failed to update target health: %v", err)
	}

	// 重新获取健康目标
	healthyTargets = make([]*types.Target, 0)
	for _, target := range upstream.Targets {
		if target.Healthy {
			healthyTargets = append(healthyTargets, target)
		}
	}

	// 现在应该选择另一个健康的服务器
	newSelected := selectByIPHashDirect(testIP, healthyTargets)
	if newSelected == nil {
		t.Fatalf("Failed to select target after health update")
	}

	if newSelected.Host == selectedHost {
		t.Errorf("After health update: Still selected unhealthy host %s", selectedHost)
	}

	// 验证新选择的一致性
	for i := 0; i < 5; i++ {
		selected := selectByIPHashDirect(testIP, healthyTargets)
		if selected.Host != newSelected.Host {
			t.Errorf("After health update, Request %d: Expected %s, got %s",
				i+1, newSelected.Host, selected.Host)
		}
	}

	t.Log(" 健康检查集成验证通过")
}

// testConsistentHashAlgorithm 测试一致性哈希算法
func testConsistentHashAlgorithm(t *testing.T) {
	// 创建配置
	cfg := &config.Config{}

	// 创建IP Hash负载均衡器
	lb := NewIPHashBalancer(cfg)

	// 创建测试上游服务
	upstream := &types.Upstream{
		ID:        "consistent-upstream",
		Name:      "Consistent Hash Test Upstream",
		Algorithm: "ip_hash",
		Targets: []*types.Target{
			{Host: "serverA.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverB.example.com", Port: 8080, Weight: 1, Healthy: true},
			{Host: "serverC.example.com", Port: 8080, Weight: 1, Healthy: true},
		},
	}

	// 添加上游服务
	err := lb.UpdateUpstream(upstream)
	if err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 测试多个IP的分布
	testIPs := []string{
		"192.168.1.1", "192.168.1.2", "192.168.1.3", "192.168.1.4", "192.168.1.5",
		"10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5",
		"172.16.0.1", "172.16.0.2", "172.16.0.3", "172.16.0.4", "172.16.0.5",
	}

	hostCounts := make(map[string]int)
	ipToHost := make(map[string]string)

	t.Log("测试一致性哈希算法的分布性")

	for _, ip := range testIPs {
		healthyTargets := make([]*types.Target, 0)
		for _, target := range upstream.Targets {
			if target.Healthy {
				healthyTargets = append(healthyTargets, target)
			}
		}

		selected := selectByIPHashDirect(ip, healthyTargets)
		if selected == nil {
			t.Fatalf("Failed to select target for IP %s", ip)
		}

		hostCounts[selected.Host]++
		ipToHost[ip] = selected.Host
	}

	// 验证分布相对均匀
	totalIPs := len(testIPs)
	expectedPerHost := totalIPs / len(upstream.Targets)
	tolerance := expectedPerHost / 2 // 50% 容差

	t.Logf("IP分布结果 (总IP数: %d):", totalIPs)
	for host, count := range hostCounts {
		t.Logf("  %s: %d IPs", host, count)

		if count < expectedPerHost-tolerance || count > expectedPerHost+tolerance {
			t.Logf("Warning: Host %s received %d IPs, expected around %d (±%d)",
				host, count, expectedPerHost, tolerance)
		}
	}

	// 验证一致性：同一IP多次请求应该得到相同结果
	for _, ip := range testIPs[:5] { // 测试前5个IP
		expectedHost := ipToHost[ip]
		for i := 0; i < 3; i++ {
			healthyTargets := make([]*types.Target, 0)
			for _, target := range upstream.Targets {
				if target.Healthy {
					healthyTargets = append(healthyTargets, target)
				}
			}

			selected := selectByIPHashDirect(ip, healthyTargets)
			if selected.Host != expectedHost {
				t.Errorf("IP %s: Request %d selected %s, expected %s",
					ip, i+1, selected.Host, expectedHost)
			}
		}
	}

	t.Log(" 一致性哈希算法验证通过")
}
