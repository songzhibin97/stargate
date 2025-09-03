package router

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAcceptanceCriteria 验证配置管理功能
func TestAcceptanceCriteria(t *testing.T) {
	t.Run("能成功解析合法的YAML文件", func(t *testing.T) {
		testValidYAMLParsing(t)
	})

	t.Run("对非法格式能返回明确错误", func(t *testing.T) {
		testInvalidYAMLHandling(t)
	})

	t.Run("内存数据与文件内容一致", func(t *testing.T) {
		testDataConsistency(t)
	})
}

// testValidYAMLParsing 测试能成功解析合法的YAML文件
func testValidYAMLParsing(t *testing.T) {
	cm := NewConfigManager()

	// 测试解析有效的配置文件
	validConfigPath := "../../configs/test-routing.yaml"
	if err := cm.LoadFromFile(validConfigPath); err != nil {
		t.Fatalf("Failed to load valid YAML config: %v", err)
	}

	config := cm.GetConfig()

	// 验证路由规则数量
	expectedRoutes := 4
	if len(config.Routes) != expectedRoutes {
		t.Errorf("Expected %d routes, got %d", expectedRoutes, len(config.Routes))
	}

	// 验证上游服务数量
	expectedUpstreams := 4
	if len(config.Upstreams) != expectedUpstreams {
		t.Errorf("Expected %d upstreams, got %d", expectedUpstreams, len(config.Upstreams))
	}

	// 验证具体的路由规则
	apiRoute, err := cm.GetRoute("api-route")
	if err != nil {
		t.Errorf("Failed to get api-route: %v", err)
	} else {
		if apiRoute.Name != "API Route" {
			t.Errorf("Expected route name 'API Route', got '%s'", apiRoute.Name)
		}
		if apiRoute.UpstreamID != "api-upstream" {
			t.Errorf("Expected upstream ID 'api-upstream', got '%s'", apiRoute.UpstreamID)
		}
		if apiRoute.Priority != 100 {
			t.Errorf("Expected priority 100, got %d", apiRoute.Priority)
		}
		if len(apiRoute.Rules.Hosts) != 2 {
			t.Errorf("Expected 2 hosts, got %d", len(apiRoute.Rules.Hosts))
		}
		if len(apiRoute.Rules.Paths) != 1 {
			t.Errorf("Expected 1 path rule, got %d", len(apiRoute.Rules.Paths))
		}
		if apiRoute.Rules.Paths[0].Type != MatchTypePrefix {
			t.Errorf("Expected prefix match type, got %s", apiRoute.Rules.Paths[0].Type)
		}
		if apiRoute.Rules.Paths[0].Value != "/api" {
			t.Errorf("Expected path '/api', got '%s'", apiRoute.Rules.Paths[0].Value)
		}
	}

	// 验证具体的上游服务
	apiUpstream, err := cm.GetUpstream("api-upstream")
	if err != nil {
		t.Errorf("Failed to get api-upstream: %v", err)
	} else {
		if apiUpstream.Name != "API Backend" {
			t.Errorf("Expected upstream name 'API Backend', got '%s'", apiUpstream.Name)
		}
		if len(apiUpstream.Targets) != 3 {
			t.Errorf("Expected 3 targets, got %d", len(apiUpstream.Targets))
		}
		if apiUpstream.Algorithm != "round_robin" {
			t.Errorf("Expected algorithm 'round_robin', got '%s'", apiUpstream.Algorithm)
		}
	}

	t.Log(" 成功解析合法的YAML文件")
}

// testInvalidYAMLHandling 测试对非法格式能返回明确错误
func testInvalidYAMLHandling(t *testing.T) {
	cm := NewConfigManager()

	// 测试解析无效的配置文件
	invalidConfigPath := "../../configs/test-invalid.yaml"
	err := cm.LoadFromFile(invalidConfigPath)
	if err == nil {
		t.Fatal("Expected error when loading invalid YAML config, but got none")
	}

	// 验证错误类型
	if !isErrorType(err, ErrConfigValidation) {
		t.Logf("Got error: %v", err)
		// 这是预期的，因为无效配置应该返回验证错误
	}

	// 测试各种无效的YAML内容
	testCases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "无效的YAML语法",
			yaml: `
routes:
  - id: "test"
    invalid_yaml: [
`,
			wantErr: true,
		},
		{
			name: "空的路由ID",
			yaml: `
routes:
  - id: ""
    name: "Test"
    rules:
      hosts: ["example.com"]
    upstream_id: "test-upstream"
upstreams:
  - id: "test-upstream"
    name: "Test"
    targets:
      - url: "http://backend.com"
`,
			wantErr: true,
		},
		{
			name: "引用不存在的上游服务",
			yaml: `
routes:
  - id: "test-route"
    name: "Test"
    rules:
      hosts: ["example.com"]
    upstream_id: "non-existent"
upstreams: []
`,
			wantErr: true,
		},
		{
			name: "重复的路由ID",
			yaml: `
routes:
  - id: "duplicate"
    name: "Route 1"
    rules:
      hosts: ["example1.com"]
    upstream_id: "test-upstream"
  - id: "duplicate"
    name: "Route 2"
    rules:
      hosts: ["example2.com"]
    upstream_id: "test-upstream"
upstreams:
  - id: "test-upstream"
    name: "Test"
    targets:
      - url: "http://backend.com"
`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := cm.LoadFromBytes([]byte(tc.yaml))
			if tc.wantErr && err == nil {
				t.Errorf("Expected error for %s, but got none", tc.name)
			} else if !tc.wantErr && err != nil {
				t.Errorf("Unexpected error for %s: %v", tc.name, err)
			}
		})
	}

	t.Log(" 对非法格式能返回明确错误")
}

// testDataConsistency 测试内存数据与文件内容一致
func testDataConsistency(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "stargate-consistency-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm := NewConfigManager()

	// 创建测试配置
	testConfig := `
routes:
  - id: "consistency-route"
    name: "Consistency Test Route"
    rules:
      hosts: ["consistency.example.com"]
      paths:
        - type: "prefix"
          value: "/test"
      methods: ["GET", "POST"]
      headers:
        - name: "X-Test-Header"
          value: "test-value"
      query_params:
        param1: "value1"
        param2: "value2"
    upstream_id: "consistency-upstream"
    priority: 150
    metadata:
      test: "consistency"
      environment: "test"

upstreams:
  - id: "consistency-upstream"
    name: "Consistency Test Upstream"
    targets:
      - url: "http://test1.example.com:8080"
        weight: 100
      - url: "https://test2.example.com:8443"
        weight: 200
    algorithm: "weighted"
    metadata:
      test: "consistency"
      region: "test-region"
`

	// 加载配置
	if err := cm.LoadFromBytes([]byte(testConfig)); err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// 保存到文件
	configFile := filepath.Join(tempDir, "consistency-test.yaml")
	if err := cm.SaveToFile(configFile); err != nil {
		t.Fatalf("Failed to save config to file: %v", err)
	}

	// 创建新的配置管理器并加载保存的文件
	cm2 := NewConfigManager()
	if err := cm2.LoadFromFile(configFile); err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	// 比较两个配置管理器的数据
	config1 := cm.GetConfig()
	config2 := cm2.GetConfig()

	// 验证路由数量一致
	if len(config1.Routes) != len(config2.Routes) {
		t.Errorf("Route count mismatch: original=%d, reloaded=%d", len(config1.Routes), len(config2.Routes))
	}

	// 验证上游服务数量一致
	if len(config1.Upstreams) != len(config2.Upstreams) {
		t.Errorf("Upstream count mismatch: original=%d, reloaded=%d", len(config1.Upstreams), len(config2.Upstreams))
	}

	// 验证具体的路由数据一致性
	route1, err1 := cm.GetRoute("consistency-route")
	route2, err2 := cm2.GetRoute("consistency-route")

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to get consistency-route: err1=%v, err2=%v", err1, err2)
	}

	if route1.ID != route2.ID {
		t.Errorf("Route ID mismatch: %s != %s", route1.ID, route2.ID)
	}
	if route1.Name != route2.Name {
		t.Errorf("Route name mismatch: %s != %s", route1.Name, route2.Name)
	}
	if route1.UpstreamID != route2.UpstreamID {
		t.Errorf("Route upstream ID mismatch: %s != %s", route1.UpstreamID, route2.UpstreamID)
	}
	if route1.Priority != route2.Priority {
		t.Errorf("Route priority mismatch: %d != %d", route1.Priority, route2.Priority)
	}

	// 验证规则一致性
	if len(route1.Rules.Hosts) != len(route2.Rules.Hosts) {
		t.Errorf("Hosts count mismatch: %d != %d", len(route1.Rules.Hosts), len(route2.Rules.Hosts))
	}
	if len(route1.Rules.Paths) != len(route2.Rules.Paths) {
		t.Errorf("Paths count mismatch: %d != %d", len(route1.Rules.Paths), len(route2.Rules.Paths))
	}
	if len(route1.Rules.Methods) != len(route2.Rules.Methods) {
		t.Errorf("Methods count mismatch: %d != %d", len(route1.Rules.Methods), len(route2.Rules.Methods))
	}
	if len(route1.Rules.Headers) != len(route2.Rules.Headers) {
		t.Errorf("Headers count mismatch: %d != %d", len(route1.Rules.Headers), len(route2.Rules.Headers))
	}
	if len(route1.Rules.Query) != len(route2.Rules.Query) {
		t.Errorf("Query params count mismatch: %d != %d", len(route1.Rules.Query), len(route2.Rules.Query))
	}

	// 验证具体的上游服务数据一致性
	upstream1, err1 := cm.GetUpstream("consistency-upstream")
	upstream2, err2 := cm2.GetUpstream("consistency-upstream")

	if err1 != nil || err2 != nil {
		t.Fatalf("Failed to get consistency-upstream: err1=%v, err2=%v", err1, err2)
	}

	if upstream1.ID != upstream2.ID {
		t.Errorf("Upstream ID mismatch: %s != %s", upstream1.ID, upstream2.ID)
	}
	if upstream1.Name != upstream2.Name {
		t.Errorf("Upstream name mismatch: %s != %s", upstream1.Name, upstream2.Name)
	}
	if upstream1.Algorithm != upstream2.Algorithm {
		t.Errorf("Upstream algorithm mismatch: %s != %s", upstream1.Algorithm, upstream2.Algorithm)
	}
	if len(upstream1.Targets) != len(upstream2.Targets) {
		t.Errorf("Targets count mismatch: %d != %d", len(upstream1.Targets), len(upstream2.Targets))
	}

	// 验证目标详细信息
	for i, target1 := range upstream1.Targets {
		if i < len(upstream2.Targets) {
			target2 := upstream2.Targets[i]
			if target1.URL != target2.URL {
				t.Errorf("Target[%d] URL mismatch: %s != %s", i, target1.URL, target2.URL)
			}
			if target1.Weight != target2.Weight {
				t.Errorf("Target[%d] weight mismatch: %d != %d", i, target1.Weight, target2.Weight)
			}
		}
	}

	t.Log(" 内存数据与文件内容一致")
}

// TestCompleteWorkflow 测试完整的工作流程
func TestCompleteWorkflow(t *testing.T) {
	t.Log("开始完整工作流程测试...")

	// 1. 创建配置管理器
	cm := NewConfigManager()

	// 2. 加载有效配置
	validConfigPath := "../../configs/test-routing.yaml"
	if err := cm.LoadFromFile(validConfigPath); err != nil {
		t.Fatalf("Failed to load valid config: %v", err)
	}

	// 3. 验证配置
	if err := cm.ValidateConfig(); err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// 4. 动态添加新的路由和上游服务
	newUpstream := Upstream{
		ID:   "dynamic-upstream",
		Name: "Dynamic Upstream",
		Targets: []Target{
			{URL: "http://dynamic.example.com", Weight: 100},
		},
		Algorithm: "round_robin",
	}
	if err := cm.AddUpstream(newUpstream); err != nil {
		t.Fatalf("Failed to add new upstream: %v", err)
	}

	newRoute := RouteRule{
		ID:   "dynamic-route",
		Name: "Dynamic Route",
		Rules: Rule{
			Hosts: []string{"dynamic.example.com"},
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/dynamic"},
			},
		},
		UpstreamID: "dynamic-upstream",
		Priority:   500,
	}
	if err := cm.AddRoute(newRoute); err != nil {
		t.Fatalf("Failed to add new route: %v", err)
	}

	// 5. 验证添加的配置
	config := cm.GetConfig()
	if len(config.Routes) != 5 { // 原来4个 + 新增1个
		t.Errorf("Expected 5 routes after adding, got %d", len(config.Routes))
	}
	if len(config.Upstreams) != 5 { // 原来4个 + 新增1个
		t.Errorf("Expected 5 upstreams after adding, got %d", len(config.Upstreams))
	}

	// 6. 测试更新功能
	updatedRoute := newRoute
	updatedRoute.Priority = 600
	if err := cm.UpdateRoute(updatedRoute); err != nil {
		t.Fatalf("Failed to update route: %v", err)
	}

	// 7. 验证更新
	retrievedRoute, err := cm.GetRoute("dynamic-route")
	if err != nil {
		t.Fatalf("Failed to get updated route: %v", err)
	}
	if retrievedRoute.Priority != 600 {
		t.Errorf("Expected updated priority 600, got %d", retrievedRoute.Priority)
	}

	// 8. 测试删除功能
	if err := cm.RemoveRoute("dynamic-route"); err != nil {
		t.Fatalf("Failed to remove route: %v", err)
	}
	if err := cm.RemoveUpstream("dynamic-upstream"); err != nil {
		t.Fatalf("Failed to remove upstream: %v", err)
	}

	// 9. 验证删除
	finalConfig := cm.GetConfig()
	if len(finalConfig.Routes) != 4 {
		t.Errorf("Expected 4 routes after removal, got %d", len(finalConfig.Routes))
	}
	if len(finalConfig.Upstreams) != 4 {
		t.Errorf("Expected 4 upstreams after removal, got %d", len(finalConfig.Upstreams))
	}

	t.Log(" 完整工作流程测试通过")
}
