package router

import (
	"net/http"
	"testing"
)

// TestAcceptanceCriteria_Task133 验证功能的功能要求
// 功能要求：必须同时满足规则中定义的所有host, path, methods条件才能匹配成功
func TestAcceptanceCriteria_Task133(t *testing.T) {
	t.Run("Host和Method组合匹配功能要求", func(t *testing.T) {
		testHostMethodCombinationAcceptance(t)
	})

	t.Run("多条件与逻辑功能要求", func(t *testing.T) {
		testMultiConditionAndLogicAcceptance(t)
	})

	t.Run("Host通配符匹配功能要求", func(t *testing.T) {
		testHostWildcardAcceptance(t)
	})
}

// testHostMethodCombinationAcceptance 测试Host和Method组合匹配功能要求
func testHostMethodCombinationAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加测试路由规则 - 同时指定Host和Method
	route := RouteRule{
		ID:   "host-method-route",
		Name: "Host Method Route",
		Rules: Rule{
			Hosts: []string{"api.example.com"},
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/api"},
			},
			Methods: []string{"GET", "POST"},
		},
		UpstreamID: "api-upstream",
		Priority:   100,
	}

	err := router.AddRoute(&route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 功能要求测试用例
	testCases := []struct {
		name        string
		method      string
		host        string
		path        string
		shouldMatch bool
		description string
	}{
		{
			name:        "功能要求：Host+Method+Path全匹配",
			method:      "GET",
			host:        "api.example.com",
			path:        "/api/users",
			shouldMatch: true,
			description: "Host=api.example.com, Method=GET, Path=/api/users 应该匹配",
		},
		{
			name:        "功能要求：Host+Method+Path全匹配(POST)",
			method:      "POST",
			host:        "api.example.com",
			path:        "/api/users",
			shouldMatch: true,
			description: "Host=api.example.com, Method=POST, Path=/api/users 应该匹配",
		},
		{
			name:        "功能要求：Host不匹配应该失败",
			method:      "GET",
			host:        "web.example.com",
			path:        "/api/users",
			shouldMatch: false,
			description: "Host=web.example.com 不匹配，整体应该失败",
		},
		{
			name:        "功能要求：Method不匹配应该失败",
			method:      "DELETE",
			host:        "api.example.com",
			path:        "/api/users",
			shouldMatch: false,
			description: "Method=DELETE 不匹配，整体应该失败",
		},
		{
			name:        "功能要求：Path不匹配应该失败",
			method:      "GET",
			host:        "api.example.com",
			path:        "/web/users",
			shouldMatch: false,
			description: "Path=/web/users 不匹配，整体应该失败",
		},
		{
			name:        "功能要求：Host带端口号应该匹配",
			method:      "GET",
			host:        "api.example.com:8080",
			path:        "/api/users",
			shouldMatch: true,
			description: "Host带端口号应该能正确匹配",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建HTTP请求
			req, err := http.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// 执行匹配
			result := router.Match(req)

			if tc.shouldMatch {
				if !result.Matched {
					t.Errorf("%s: 期望匹配但未匹配 - %s", tc.name, tc.description)
					return
				}
				if result.Route == nil {
					t.Errorf("%s: 期望有匹配的路由但为空", tc.name)
					return
				}
				t.Logf(" %s: 通过，匹配路由 %s - %s",
					tc.name, result.Route.ID, tc.description)
			} else {
				if result.Matched {
					t.Errorf("%s: 期望不匹配但匹配了路由 %s - %s",
						tc.name, result.Route.ID, tc.description)
					return
				}
				t.Logf(" %s: 通过，正确未匹配 - %s", tc.name, tc.description)
			}
		})
	}
}

// testMultiConditionAndLogicAcceptance 测试多条件与逻辑功能要求
func testMultiConditionAndLogicAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加复杂的多条件路由规则
	route := RouteRule{
		ID:   "multi-condition-route",
		Name: "Multi Condition Route",
		Rules: Rule{
			Hosts: []string{"api.example.com", "api2.example.com"},
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/api/v1"},
				{Type: MatchTypeExact, Value: "/health"},
			},
			Methods: []string{"GET", "POST", "PUT"},
			Headers: []HeaderRule{
				{Name: "X-API-Version", Value: "v1"},
			},
			QueryParams: map[string]string{
				"format": "json",
			},
		},
		UpstreamID: "complex-upstream",
		Priority:   200,
	}

	err := router.AddRoute(&route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 多条件与逻辑测试用例
	testCases := []struct {
		name        string
		method      string
		host        string
		path        string
		headers     map[string]string
		query       string
		shouldMatch bool
		description string
	}{
		{
			name:   "功能要求：所有条件都满足应该匹配",
			method: "GET",
			host:   "api.example.com",
			path:   "/api/v1/users",
			headers: map[string]string{
				"X-API-Version": "v1",
			},
			query:       "?format=json",
			shouldMatch: true,
			description: "所有条件(Host+Path+Method+Header+Query)都满足",
		},
		{
			name:        "功能要求：缺少Header应该失败",
			method:      "GET",
			host:        "api.example.com",
			path:        "/api/v1/users",
			query:       "?format=json",
			shouldMatch: false,
			description: "缺少必需的Header，应该匹配失败",
		},
		{
			name:   "功能要求：缺少Query应该失败",
			method: "GET",
			host:   "api.example.com",
			path:   "/api/v1/users",
			headers: map[string]string{
				"X-API-Version": "v1",
			},
			shouldMatch: false,
			description: "缺少必需的Query参数，应该匹配失败",
		},
		{
			name:   "功能要求：使用第二个Host应该匹配",
			method: "POST",
			host:   "api2.example.com",
			path:   "/health",
			headers: map[string]string{
				"X-API-Version": "v1",
			},
			query:       "?format=json",
			shouldMatch: true,
			description: "使用第二个Host和精确路径匹配",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 构建完整URL
			url := "http://" + tc.host + tc.path + tc.query

			// 创建HTTP请求
			req, err := http.NewRequest(tc.method, url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// 添加请求头
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}

			// 执行匹配
			result := router.Match(req)

			if tc.shouldMatch {
				if !result.Matched {
					t.Errorf("%s: 期望匹配但未匹配 - %s", tc.name, tc.description)
					return
				}
				t.Logf(" %s: 通过，匹配路由 %s - %s",
					tc.name, result.Route.ID, tc.description)
			} else {
				if result.Matched {
					t.Errorf("%s: 期望不匹配但匹配了路由 %s - %s",
						tc.name, result.Route.ID, tc.description)
					return
				}
				t.Logf(" %s: 通过，正确未匹配 - %s", tc.name, tc.description)
			}
		})
	}
}

// testHostWildcardAcceptance 测试Host通配符匹配功能要求
func testHostWildcardAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加通配符Host路由规则
	route := RouteRule{
		ID:   "wildcard-host-route",
		Name: "Wildcard Host Route",
		Rules: Rule{
			Hosts: []string{"*.example.com"},
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/api"},
			},
			Methods: []string{"GET"},
		},
		UpstreamID: "wildcard-upstream",
		Priority:   100,
	}

	err := router.AddRoute(&route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 通配符Host测试用例
	testCases := []struct {
		name        string
		host        string
		shouldMatch bool
		description string
	}{
		{
			name:        "功能要求：子域名应该匹配通配符",
			host:        "api.example.com",
			shouldMatch: true,
			description: "api.example.com 应该匹配 *.example.com",
		},
		{
			name:        "功能要求：另一个子域名应该匹配",
			host:        "web.example.com",
			shouldMatch: true,
			description: "web.example.com 应该匹配 *.example.com",
		},
		{
			name:        "功能要求：不同域名不应该匹配",
			host:        "api.other.com",
			shouldMatch: false,
			description: "api.other.com 不应该匹配 *.example.com",
		},
		{
			name:        "功能要求：根域名不应该匹配通配符",
			host:        "example.com",
			shouldMatch: false,
			description: "example.com 不应该匹配 *.example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建HTTP请求
			req, err := http.NewRequest("GET", "http://"+tc.host+"/api/test", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// 执行匹配
			result := router.Match(req)

			if tc.shouldMatch {
				if !result.Matched {
					t.Errorf("%s: 期望匹配但未匹配 - %s", tc.name, tc.description)
					return
				}
				t.Logf(" %s: 通过，匹配路由 %s - %s",
					tc.name, result.Route.ID, tc.description)
			} else {
				if result.Matched {
					t.Errorf("%s: 期望不匹配但匹配了路由 %s - %s",
						tc.name, result.Route.ID, tc.description)
					return
				}
				t.Logf(" %s: 通过，正确未匹配 - %s", tc.name, tc.description)
			}
		})
	}
}
