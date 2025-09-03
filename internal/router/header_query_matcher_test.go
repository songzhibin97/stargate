package router

import (
	"net/http"
	"testing"
)

// TestAcceptanceCriteria 验证请求头和查询参数匹配功能
// 功能要求：支持请求头和查询参数的存在、缺失或值匹配，包括多值处理
func TestHeaderQueryMatcherAcceptanceCriteria(t *testing.T) {
	t.Run("请求头匹配功能要求", func(t *testing.T) {
		testHeaderMatchingAcceptance(t)
	})

	t.Run("查询参数匹配功能要求", func(t *testing.T) {
		testQueryMatchingAcceptance(t)
	})

	t.Run("请求头和查询参数组合匹配功能要求", func(t *testing.T) {
		testHeaderQueryCombinationAcceptance(t)
	})
}

// testHeaderMatchingAcceptance 测试请求头匹配功能
func testHeaderMatchingAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加测试路由规则 - 包含各种请求头匹配类型
	route := RouteRule{
		ID:   "header-match-route",
		Name: "Header Match Route",
		Rules: Rule{
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/api"},
			},
			Headers: []HeaderRule{
				{Name: "X-API-Key", MatchType: HeaderMatchExists},
				{Name: "Authorization", Value: "Bearer token123", MatchType: HeaderMatchValue},
				{Name: "X-Version", Value: "v[12]", MatchType: HeaderMatchRegex},
				{Name: "X-Debug", MatchType: HeaderMatchNotExists},
			},
		},
		UpstreamID: "header-upstream",
		Priority:   100,
	}

	err := router.AddRoute(&route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 功能要求测试用例
	testCases := []struct {
		name        string
		path        string
		headers     map[string]string
		shouldMatch bool
		description string
	}{
		{
			name: "功能要求：所有请求头条件满足",
			path: "/api/users",
			headers: map[string]string{
				"X-API-Key":     "abc123",
				"Authorization": "Bearer token123",
				"X-Version":     "v1",
				// X-Debug 不存在，满足 NotExists 条件
			},
			shouldMatch: true,
			description: "所有请求头匹配条件都满足",
		},
		{
			name: "功能要求：缺少必需的请求头",
			path: "/api/users",
			headers: map[string]string{
				"Authorization": "Bearer token123",
				"X-Version":     "v2",
			},
			shouldMatch: false,
			description: "缺少X-API-Key请求头，应该匹配失败",
		},
		{
			name: "功能要求：请求头值不匹配",
			path: "/api/users",
			headers: map[string]string{
				"X-API-Key":     "abc123",
				"Authorization": "Bearer wrong-token",
				"X-Version":     "v1",
			},
			shouldMatch: false,
			description: "Authorization值不匹配，应该匹配失败",
		},
		{
			name: "功能要求：正则表达式匹配成功",
			path: "/api/users",
			headers: map[string]string{
				"X-API-Key":     "abc123",
				"Authorization": "Bearer token123",
				"X-Version":     "v2",
			},
			shouldMatch: true,
			description: "X-Version=v2匹配正则表达式v[12]",
		},
		{
			name: "功能要求：正则表达式匹配失败",
			path: "/api/users",
			headers: map[string]string{
				"X-API-Key":     "abc123",
				"Authorization": "Bearer token123",
				"X-Version":     "v3",
			},
			shouldMatch: false,
			description: "X-Version=v3不匹配正则表达式v[12]",
		},
		{
			name: "功能要求：不应存在的请求头存在",
			path: "/api/users",
			headers: map[string]string{
				"X-API-Key":     "abc123",
				"Authorization": "Bearer token123",
				"X-Version":     "v1",
				"X-Debug":       "true",
			},
			shouldMatch: false,
			description: "X-Debug不应该存在但存在了，应该匹配失败",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建HTTP请求
			req, err := http.NewRequest("GET", "http://example.com"+tc.path, nil)
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

// testQueryMatchingAcceptance 测试查询参数匹配功能
func testQueryMatchingAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加测试路由规则 - 包含各种查询参数匹配类型
	route := RouteRule{
		ID:   "query-match-route",
		Name: "Query Match Route",
		Rules: Rule{
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/search"},
			},
			Query: []QueryRule{
				{Name: "q", MatchType: QueryMatchExists},
				{Name: "format", Value: "json", MatchType: QueryMatchValue},
				{Name: "page", Value: "[1-9][0-9]*", MatchType: QueryMatchRegex},
				{Name: "debug", MatchType: QueryMatchNotExists},
			},
		},
		UpstreamID: "query-upstream",
		Priority:   100,
	}

	err := router.AddRoute(&route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 功能要求测试用例
	testCases := []struct {
		name        string
		path        string
		query       string
		shouldMatch bool
		description string
	}{
		{
			name:        "功能要求：所有查询参数条件满足",
			path:        "/search",
			query:       "?q=golang&format=json&page=1",
			shouldMatch: true,
			description: "所有查询参数匹配条件都满足",
		},
		{
			name:        "功能要求：缺少必需的查询参数",
			path:        "/search",
			query:       "?format=json&page=2",
			shouldMatch: false,
			description: "缺少q查询参数，应该匹配失败",
		},
		{
			name:        "功能要求：查询参数值不匹配",
			path:        "/search",
			query:       "?q=golang&format=xml&page=1",
			shouldMatch: false,
			description: "format=xml不匹配期望的json值",
		},
		{
			name:        "功能要求：正则表达式匹配成功",
			path:        "/search",
			query:       "?q=golang&format=json&page=123",
			shouldMatch: true,
			description: "page=123匹配正则表达式[1-9][0-9]*",
		},
		{
			name:        "功能要求：正则表达式匹配失败",
			path:        "/search",
			query:       "?q=golang&format=json&page=0",
			shouldMatch: false,
			description: "page=0不匹配正则表达式[1-9][0-9]*",
		},
		{
			name:        "功能要求：不应存在的查询参数存在",
			path:        "/search",
			query:       "?q=golang&format=json&page=1&debug=true",
			shouldMatch: false,
			description: "debug参数不应该存在但存在了，应该匹配失败",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建HTTP请求
			fullURL := "http://example.com" + tc.path + tc.query
			req, err := http.NewRequest("GET", fullURL, nil)
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

// testHeaderQueryCombinationAcceptance 测试请求头和查询参数组合匹配功能要求
func testHeaderQueryCombinationAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加复杂的组合匹配路由规则
	route := RouteRule{
		ID:   "combination-match-route",
		Name: "Combination Match Route",
		Rules: Rule{
			Hosts: []string{"api.example.com"},
			Paths: []PathRule{
				{Type: MatchTypePrefix, Value: "/api/v1"},
			},
			Methods: []string{"POST"},
			Headers: []HeaderRule{
				{Name: "Content-Type", Value: "application/json", MatchType: HeaderMatchValue},
				{Name: "X-Client-Version", Value: "^[0-9]+\\.[0-9]+", MatchType: HeaderMatchRegex},
			},
			Query: []QueryRule{
				{Name: "validate", Value: "true", MatchType: QueryMatchValue},
				{Name: "timeout", MatchType: QueryMatchExists},
			},
		},
		UpstreamID: "combination-upstream",
		Priority:   200,
	}

	err := router.AddRoute(&route)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 组合匹配测试用例
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
			name:   "功能要求：所有条件都满足",
			method: "POST",
			host:   "api.example.com",
			path:   "/api/v1/users",
			headers: map[string]string{
				"Content-Type":     "application/json",
				"X-Client-Version": "1.2",
			},
			query:       "?validate=true&timeout=30",
			shouldMatch: true,
			description: "Host+Path+Method+Headers+Query所有条件都满足",
		},
		{
			name:   "功能要求：请求头不满足",
			method: "POST",
			host:   "api.example.com",
			path:   "/api/v1/users",
			headers: map[string]string{
				"Content-Type":     "application/xml",
				"X-Client-Version": "1.2",
			},
			query:       "?validate=true&timeout=30",
			shouldMatch: false,
			description: "Content-Type不匹配，应该匹配失败",
		},
		{
			name:   "功能要求：查询参数不满足",
			method: "POST",
			host:   "api.example.com",
			path:   "/api/v1/users",
			headers: map[string]string{
				"Content-Type":     "application/json",
				"X-Client-Version": "1.2",
			},
			query:       "?validate=false&timeout=30",
			shouldMatch: false,
			description: "validate=false不匹配期望的true值",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 构建完整URL
			fullURL := "http://" + tc.host + tc.path + tc.query

			// 创建HTTP请求
			req, err := http.NewRequest(tc.method, fullURL, nil)
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
