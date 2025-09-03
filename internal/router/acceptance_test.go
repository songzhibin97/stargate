package router

import (
	"net/http"
	"testing"
)

// TestAcceptanceCriteria 验证路径匹配功能
func TestAcceptanceCriteria_Task132(t *testing.T) {
	t.Run("前缀匹配测试", func(t *testing.T) {
		testPrefixMatchingAcceptance(t)
	})

	t.Run("精确匹配测试", func(t *testing.T) {
		testExactMatchingAcceptance(t)
	})

	t.Run("集成测试", func(t *testing.T) {
		testIntegratedMatchingAcceptance(t)
	})
}

// testPrefixMatchingAcceptance 测试前缀匹配功能
// 测试要求：type: prefix, path: /api/ 能匹配 /api/users
func testPrefixMatchingAcceptance(t *testing.T) {
	// 创建前缀匹配规则
	rule := PathRule{
		Type:  MatchTypePrefix,
		Value: "/api/",
	}

	// 创建路径匹配器
	factory := NewPathMatcherFactory()
	matcher, err := factory.CreateMatcher(rule)
	if err != nil {
		t.Fatalf("Failed to create prefix matcher: %v", err)
	}

	// 测试用例
	testCases := []struct {
		name        string
		requestPath string
		expected    bool
		description string
	}{
		{
			name:        "前缀匹配：/api/ 匹配 /api/users",
			requestPath: "/api/users",
			expected:    true,
			description: "前缀 /api/ 应该能匹配 /api/users",
		},
		{
			name:        "前缀匹配：/api/ 匹配 /api/users/123",
			requestPath: "/api/users/123",
			expected:    true,
			description: "前缀 /api/ 应该能匹配 /api/users/123",
		},
		{
			name:        "前缀匹配：/api/ 匹配 /api/",
			requestPath: "/api/",
			expected:    true,
			description: "前缀 /api/ 应该能匹配自身",
		},
		{
			name:        "前缀不匹配：/api/ 不匹配 /web/users",
			requestPath: "/web/users",
			expected:    false,
			description: "前缀 /api/ 不应该匹配 /web/users",
		},
		{
			name:        "前缀不匹配：/api/ 不匹配 /api",
			requestPath: "/api",
			expected:    false,
			description: "前缀 /api/ 不应该匹配 /api（缺少尾部斜杠）",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matcher.Match(tc.requestPath)
			if result != tc.expected {
				t.Errorf("%s: 期望 %v，实际 %v - %s",
					tc.name, tc.expected, result, tc.description)
			} else {
				t.Logf(" %s: 通过 - %s", tc.name, tc.description)
			}
		})
	}
}

// testExactMatchingAcceptance 测试精确匹配功能
// 测试要求：type: exact, path: /login 不能匹配 /login/now
func testExactMatchingAcceptance(t *testing.T) {
	// 创建精确匹配规则
	rule := PathRule{
		Type:  MatchTypeExact,
		Value: "/login",
	}

	// 创建路径匹配器
	factory := NewPathMatcherFactory()
	matcher, err := factory.CreateMatcher(rule)
	if err != nil {
		t.Fatalf("Failed to create exact matcher: %v", err)
	}

	// 测试用例
	testCases := []struct {
		name        string
		requestPath string
		expected    bool
		description string
	}{
		{
			name:        "精确匹配：/login 不能匹配 /login/now",
			requestPath: "/login/now",
			expected:    false,
			description: "精确匹配 /login 不应该匹配 /login/now",
		},
		{
			name:        "精确匹配：/login 匹配 /login",
			requestPath: "/login",
			expected:    true,
			description: "精确匹配 /login 应该匹配自身",
		},
		{
			name:        "精确不匹配：/login 不匹配 /login/",
			requestPath: "/login/",
			expected:    false,
			description: "精确匹配 /login 不应该匹配 /login/",
		},
		{
			name:        "精确不匹配：/login 不匹配 /logout",
			requestPath: "/logout",
			expected:    false,
			description: "精确匹配 /login 不应该匹配 /logout",
		},
		{
			name:        "精确不匹配：/login 不匹配 /log",
			requestPath: "/log",
			expected:    false,
			description: "精确匹配 /login 不应该匹配 /log",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matcher.Match(tc.requestPath)
			if result != tc.expected {
				t.Errorf("%s: 期望 %v，实际 %v - %s",
					tc.name, tc.expected, result, tc.description)
			} else {
				t.Logf(" %s: 通过 - %s", tc.name, tc.description)
			}
		})
	}
}

// testIntegratedMatchingAcceptance 测试集成匹配功能
func testIntegratedMatchingAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加测试路由规则
	routes := []RouteRule{
		{
			ID:   "prefix-api-route",
			Name: "API Prefix Route",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/api/"},
				},
			},
			UpstreamID: "api-upstream",
			Priority:   100,
		},
		{
			ID:   "exact-login-route",
			Name: "Login Exact Route",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypeExact, Value: "/login"},
				},
			},
			UpstreamID: "auth-upstream",
			Priority:   200,
		},
		{
			ID:   "regex-user-route",
			Name: "User Regex Route",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypeRegex, Value: "^/users/[0-9]+$"},
				},
			},
			UpstreamID: "user-upstream",
			Priority:   150,
		},
	}

	// 添加路由到路由器
	for _, route := range routes {
		if err := router.AddRoute(&route); err != nil {
			t.Fatalf("Failed to add route %s: %v", route.ID, err)
		}
	}

	// 集成测试用例
	testCases := []struct {
		name        string
		method      string
		url         string
		expectedID  string
		shouldMatch bool
		description string
	}{
		{
			name:        "前缀匹配：/api/ 能匹配 /api/users",
			method:      "GET",
			url:         "http://example.com/api/users",
			expectedID:  "prefix-api-route",
			shouldMatch: true,
			description: "前缀路由应该匹配 /api/users",
		},
		{
			name:        "精确匹配：/login 不能匹配 /login/now",
			method:      "GET",
			url:         "http://example.com/login/now",
			shouldMatch: false,
			description: "精确路由不应该匹配 /login/now",
		},
		{
			name:        "精确匹配：/login 匹配 /login",
			method:      "GET",
			url:         "http://example.com/login",
			expectedID:  "exact-login-route",
			shouldMatch: true,
			description: "精确路由应该匹配 /login",
		},
		{
			name:        "正则匹配：用户ID路径",
			method:      "GET",
			url:         "http://example.com/users/123",
			expectedID:  "regex-user-route",
			shouldMatch: true,
			description: "正则路由应该匹配 /users/123",
		},
		{
			name:        "正则不匹配：用户非数字ID",
			method:      "GET",
			url:         "http://example.com/users/abc",
			shouldMatch: false,
			description: "正则路由不应该匹配 /users/abc",
		},
		{
			name:        "前缀匹配：深层API路径",
			method:      "POST",
			url:         "http://example.com/api/v1/users/create",
			expectedID:  "prefix-api-route",
			shouldMatch: true,
			description: "前缀路由应该匹配深层API路径",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建HTTP请求
			req, err := http.NewRequest(tc.method, tc.url, nil)
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
				if tc.expectedID != "" && result.Route.ID != tc.expectedID {
					t.Errorf("%s: 期望匹配路由ID %s，实际 %s",
						tc.name, tc.expectedID, result.Route.ID)
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

// TestSpecificAcceptanceCriteria 测试具体的功能要求
func TestSpecificAcceptanceCriteria(t *testing.T) {
	t.Log("开始验证路径匹配的具体功能要求...")

	// 功能要求1：type: prefix, path: /api/ 能匹配 /api/users
	t.Run("功能要求1", func(t *testing.T) {
		rule := PathRule{Type: MatchTypePrefix, Value: "/api/"}
		matcher := NewPrefixPathMatcher(rule.Value)

		result := matcher.Match("/api/users")
		if !result {
			t.Errorf(" 功能要求1失败：type: prefix, path: /api/ 应该能匹配 /api/users")
		} else {
			t.Logf(" 功能要求1通过：type: prefix, path: /api/ 能匹配 /api/users")
		}
	})

	// 功能要求2：type: exact, path: /login 不能匹配 /login/now
	t.Run("功能要求2", func(t *testing.T) {
		rule := PathRule{Type: MatchTypeExact, Value: "/login"}
		matcher := NewExactPathMatcher(rule.Value)

		result := matcher.Match("/login/now")
		if result {
			t.Errorf(" 功能要求2失败：type: exact, path: /login 不应该匹配 /login/now")
		} else {
			t.Logf(" 功能要求2通过：type: exact, path: /login 不能匹配 /login/now")
		}
	})

	// 额外验证：确保精确匹配能匹配自身
	t.Run("精确匹配自身验证", func(t *testing.T) {
		rule := PathRule{Type: MatchTypeExact, Value: "/login"}
		matcher := NewExactPathMatcher(rule.Value)

		result := matcher.Match("/login")
		if !result {
			t.Errorf(" 精确匹配应该能匹配自身：/login")
		} else {
			t.Logf(" 精确匹配能正确匹配自身：/login")
		}
	})

	t.Log(" 所有功能要求验证完成")
}
