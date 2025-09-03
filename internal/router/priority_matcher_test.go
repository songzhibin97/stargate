package router

import (
	"net/http"
	"testing"
)

// TestAcceptanceCriteria 验证路由优先级功能
// 功能要求：当多个路由都匹配同一个请求时，优先级高的路由被选中
func TestPriorityMatcherAcceptanceCriteria(t *testing.T) {
	t.Run("路由优先级功能功能要求", func(t *testing.T) {
		testRoutePriorityAcceptance(t)
	})

	t.Run("优先级排序功能要求", func(t *testing.T) {
		testPrioritySortingAcceptance(t)
	})

	t.Run("相同优先级处理功能要求", func(t *testing.T) {
		testSamePriorityAcceptance(t)
	})
}

// testRoutePriorityAcceptance 测试路由优先级功能要求
func testRoutePriorityAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加多个可能匹配同一请求的路由规则，设置不同优先级
	routes := []RouteRule{
		{
			ID:   "low-priority-route",
			Name: "Low Priority Route",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/api"},
				},
			},
			UpstreamID: "low-upstream",
			Priority:   100, // 低优先级
		},
		{
			ID:   "high-priority-route",
			Name: "High Priority Route",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/api"},
				},
			},
			UpstreamID: "high-upstream",
			Priority:   500, // 高优先级
		},
		{
			ID:   "medium-priority-route",
			Name: "Medium Priority Route",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/api"},
				},
			},
			UpstreamID: "medium-upstream",
			Priority:   300, // 中等优先级
		},
	}

	// 添加路由规则
	for _, route := range routes {
		err := router.AddRoute(&route)
		if err != nil {
			t.Fatalf("Failed to add route %s: %v", route.ID, err)
		}
	}

	// 功能要求测试用例
	testCases := []struct {
		name            string
		method          string
		url             string
		expectedRouteID string
		description     string
	}{
		{
			name:            "功能要求：多个路由匹配时选择最高优先级",
			method:          "GET",
			url:             "http://example.com/api/users",
			expectedRouteID: "high-priority-route",
			description:     "所有三个路由都匹配/api/users，应该选择优先级最高的(500)",
		},
		{
			name:            "功能要求：不同路径但都匹配的情况",
			method:          "POST",
			url:             "http://example.com/api/orders",
			expectedRouteID: "high-priority-route",
			description:     "所有三个路由都匹配/api/orders，应该选择优先级最高的(500)",
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

			if !result.Matched {
				t.Errorf("%s: 期望匹配但未匹配 - %s", tc.name, tc.description)
				return
			}

			if result.Route.ID != tc.expectedRouteID {
				t.Errorf("%s: 期望匹配路由 %s，但匹配了 %s - %s",
					tc.name, tc.expectedRouteID, result.Route.ID, tc.description)
				return
			}

			t.Logf(" %s: 通过，匹配了优先级最高的路由 %s (优先级: %d) - %s",
				tc.name, result.Route.ID, result.Route.Priority, tc.description)
		})
	}
}

// testPrioritySortingAcceptance 测试优先级排序功能要求
func testPrioritySortingAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 按随机顺序添加不同优先级的路由
	routes := []RouteRule{
		{
			ID:   "priority-50",
			Name: "Priority 50",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/test"},
				},
			},
			UpstreamID: "upstream-50",
			Priority:   50,
		},
		{
			ID:   "priority-1000",
			Name: "Priority 1000",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/test"},
				},
			},
			UpstreamID: "upstream-1000",
			Priority:   1000,
		},
		{
			ID:   "priority-200",
			Name: "Priority 200",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/test"},
				},
			},
			UpstreamID: "upstream-200",
			Priority:   200,
		},
		{
			ID:   "priority-0",
			Name: "Priority 0",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/test"},
				},
			},
			UpstreamID: "upstream-0",
			Priority:   0,
		},
	}

	// 添加路由规则
	for _, route := range routes {
		err := router.AddRoute(&route)
		if err != nil {
			t.Fatalf("Failed to add route %s: %v", route.ID, err)
		}
	}

	// 验证排序：应该按优先级从高到低排序
	allRoutes := router.GetRoutes()
	expectedOrder := []string{"priority-1000", "priority-200", "priority-50", "priority-0"}

	if len(allRoutes) != len(expectedOrder) {
		t.Fatalf("Expected %d routes, got %d", len(expectedOrder), len(allRoutes))
	}

	for i, expectedID := range expectedOrder {
		if allRoutes[i].ID != expectedID {
			t.Errorf("功能要求：路由排序错误，位置 %d 期望 %s，实际 %s",
				i, expectedID, allRoutes[i].ID)
		}
	}

	// 验证匹配结果：应该总是返回优先级最高的路由
	req, err := http.NewRequest("GET", "http://example.com/test/anything", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	result := router.Match(req)
	if !result.Matched {
		t.Errorf("功能要求：应该匹配到路由")
		return
	}

	if result.Route.ID != "priority-1000" {
		t.Errorf("功能要求：应该匹配优先级最高的路由 priority-1000，实际匹配 %s",
			result.Route.ID)
		return
	}

	t.Logf(" 功能要求：路由按优先级正确排序，匹配时选择优先级最高的路由")
}

// testSamePriorityAcceptance 测试相同优先级处理功能要求
func testSamePriorityAcceptance(t *testing.T) {
	// 创建增强路由器
	router := NewEnhancedRouter()

	// 添加相同优先级的路由
	routes := []RouteRule{
		{
			ID:   "same-priority-1",
			Name: "Same Priority 1",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/same"},
				},
			},
			UpstreamID: "upstream-1",
			Priority:   100,
		},
		{
			ID:   "same-priority-2",
			Name: "Same Priority 2",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/same"},
				},
			},
			UpstreamID: "upstream-2",
			Priority:   100,
		},
		{
			ID:   "higher-priority",
			Name: "Higher Priority",
			Rules: Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/same"},
				},
			},
			UpstreamID: "upstream-high",
			Priority:   200,
		},
	}

	// 添加路由规则
	for _, route := range routes {
		err := router.AddRoute(&route)
		if err != nil {
			t.Fatalf("Failed to add route %s: %v", route.ID, err)
		}
	}

	// 验证相同优先级的处理：应该选择更高优先级的路由
	req, err := http.NewRequest("GET", "http://example.com/same/test", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	result := router.Match(req)
	if !result.Matched {
		t.Errorf("功能要求：应该匹配到路由")
		return
	}

	if result.Route.ID != "higher-priority" {
		t.Errorf("功能要求：存在更高优先级路由时，应该匹配 higher-priority，实际匹配 %s",
			result.Route.ID)
		return
	}

	t.Logf(" 功能要求：存在相同优先级路由时，正确选择了更高优先级的路由")

	// 测试只有相同优先级路由的情况
	router2 := NewEnhancedRouter()

	// 只添加相同优先级的前两个路由
	for i := 0; i < 2; i++ {
		err := router2.AddRoute(&routes[i])
		if err != nil {
			t.Fatalf("Failed to add route %s: %v", routes[i].ID, err)
		}
	}

	result2 := router2.Match(req)
	if !result2.Matched {
		t.Errorf("功能要求：应该匹配到路由")
		return
	}

	// 相同优先级时，应该返回第一个匹配的（按添加顺序）
	expectedID := "same-priority-1" // 第一个添加的
	if result2.Route.ID != expectedID {
		t.Errorf("功能要求：相同优先级时应该匹配第一个添加的路由 %s，实际匹配 %s",
			expectedID, result2.Route.ID)
		return
	}

	t.Logf(" 功能要求：相同优先级时正确选择了第一个匹配的路由")
}
