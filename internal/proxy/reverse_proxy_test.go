package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/types"
)

// TestAcceptanceCriteria 验证反向代理功能
// 功能要求：请求被正确转发、后端响应被正确返回、代理头信息被正确添加
func TestReverseProxyAcceptanceCriteria(t *testing.T) {
	t.Run("反向代理处理器测试", func(t *testing.T) {
		testReverseProxyAcceptance(t)
	})

	t.Run("路由匹配和代理转发集成测试", func(t *testing.T) {
		testRouteMatchingAndProxyAcceptance(t)
	})

	t.Run("代理头处理测试", func(t *testing.T) {
		testProxyHeadersAcceptance(t)
	})
}

// testReverseProxyAcceptance 测试反向代理处理器功能
func testReverseProxyAcceptance(t *testing.T) {
	// 创建模拟后端服务器
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回请求信息，用于验证转发是否正确
		response := map[string]interface{}{
			"method":  r.Method,
			"path":    r.URL.Path,
			"query":   r.URL.RawQuery,
			"headers": r.Header,
			"host":    r.Host,
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Backend-Server", "test-backend")
		json.NewEncoder(w).Encode(response)
	}))
	defer backendServer.Close()

	// 创建配置
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			BufferSize:            32768,
		},
	}

	// 创建路由适配器
	routerAdapter := NewRouterAdapter()

	// 创建增强反向代理
	enhancedProxy, err := NewEnhancedReverseProxy(cfg, routerAdapter)
	if err != nil {
		t.Fatalf("Failed to create enhanced reverse proxy: %v", err)
	}

	// 添加测试路由
	testRoute := &router.RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: router.Rule{
			Paths: []router.PathRule{
				{Type: router.MatchTypePrefix, Value: "/api"},
			},
			Methods: []string{"GET", "POST"},
		},
		UpstreamID: "test-upstream",
		Priority:   100,
	}

	err = routerAdapter.AddRouteRule(testRoute)
	if err != nil {
		t.Fatalf("Failed to add test route: %v", err)
	}

	// 添加测试上游服务
	testUpstream := &types.Upstream{
		ID:        "test-upstream",
		Name:      "Test Upstream",
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{
				Host:    strings.TrimPrefix(backendServer.URL, "http://"),
				Port:    80, // httptest server uses a random port, but we'll handle this in the test
				Weight:  100,
				Healthy: true,
			},
		},
	}

	// 修正目标端口
	parts := strings.Split(strings.TrimPrefix(backendServer.URL, "http://"), ":")
	if len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &testUpstream.Targets[0].Port)
		testUpstream.Targets[0].Host = parts[0]
	}

	enhancedProxy.AddUpstream(testUpstream)

	// 测试用例
	testCases := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "GET请求被正确转发",
			method:         "GET",
			path:           "/api/users",
			expectedStatus: http.StatusOK,
			description:    "GET请求应该被正确转发到后端服务",
		},
		{
			name:           "POST请求被正确转发",
			method:         "POST",
			path:           "/api/users",
			expectedStatus: http.StatusOK,
			description:    "POST请求应该被正确转发到后端服务",
		},
		{
			name:           "不匹配的路径返回404",
			method:         "GET",
			path:           "/web/users",
			expectedStatus: http.StatusNotFound,
			description:    "不匹配的路径应该返回404",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试请求
			req := httptest.NewRequest(tc.method, "http://example.com"+tc.path, nil)
			req.Header.Set("X-Test-Header", "test-value")

			// 创建响应记录器
			rr := httptest.NewRecorder()

			// 执行请求
			enhancedProxy.ServeHTTP(rr, req)

			// 验证状态码
			if rr.Code != tc.expectedStatus {
				t.Errorf("%s: 期望状态码 %d，实际 %d - %s",
					tc.name, tc.expectedStatus, rr.Code, tc.description)
				return
			}

			if tc.expectedStatus == http.StatusOK {
				// 验证响应内容
				var response map[string]interface{}
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Errorf("%s: 无法解析响应JSON: %v", tc.name, err)
					return
				}

				// 验证请求被正确转发
				if response["method"] != tc.method {
					t.Errorf("%s: 期望方法 %s，实际 %s", tc.name, tc.method, response["method"])
				}

				if response["path"] != tc.path {
					t.Errorf("%s: 期望路径 %s，实际 %s", tc.name, tc.path, response["path"])
				}

				// 验证后端响应被正确返回
				if rr.Header().Get("X-Backend-Server") != "test-backend" {
					t.Errorf("%s: 缺少后端服务器响应头", tc.name)
				}

				t.Logf(" %s: 通过 - %s", tc.name, tc.description)
			} else {
				t.Logf(" %s: 通过 - %s", tc.name, tc.description)
			}
		})
	}
}

// testRouteMatchingAndProxyAcceptance 测试路由匹配和代理转发集成功能要求
func testRouteMatchingAndProxyAcceptance(t *testing.T) {
	// 创建多个模拟后端服务器
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Service", "api-service")
		json.NewEncoder(w).Encode(map[string]string{"service": "api", "path": r.URL.Path})
	}))
	defer apiServer.Close()

	webServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Service", "web-service")
		json.NewEncoder(w).Encode(map[string]string{"service": "web", "path": r.URL.Path})
	}))
	defer webServer.Close()

	// 创建配置
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			BufferSize:            32768,
		},
	}

	// 创建路由适配器
	routerAdapter := NewRouterAdapter()

	// 创建增强反向代理
	enhancedProxy, err := NewEnhancedReverseProxy(cfg, routerAdapter)
	if err != nil {
		t.Fatalf("Failed to create enhanced reverse proxy: %v", err)
	}

	// 添加多个路由规则
	routes := []router.RouteRule{
		{
			ID:   "api-route",
			Name: "API Route",
			Rules: router.Rule{
				Paths: []router.PathRule{
					{Type: router.MatchTypePrefix, Value: "/api"},
				},
			},
			UpstreamID: "api-upstream",
			Priority:   200,
		},
		{
			ID:   "web-route",
			Name: "Web Route",
			Rules: router.Rule{
				Paths: []router.PathRule{
					{Type: router.MatchTypePrefix, Value: "/web"},
				},
			},
			UpstreamID: "web-upstream",
			Priority:   100,
		},
	}

	for _, route := range routes {
		if err := routerAdapter.AddRouteRule(&route); err != nil {
			t.Fatalf("Failed to add route %s: %v", route.ID, err)
		}
	}

	// 添加上游服务
	enhancedProxy.AddUpstream(createUpstreamFromServer("api-upstream", "API Upstream", apiServer))
	enhancedProxy.AddUpstream(createUpstreamFromServer("web-upstream", "Web Upstream", webServer))

	// 测试用例
	testCases := []struct {
		name            string
		path            string
		expectedService string
		description     string
	}{
		{
			name:            "API路由匹配和转发",
			path:            "/api/users",
			expectedService: "api-service",
			description:     "API路由应该被匹配并转发到API服务",
		},
		{
			name:            "Web路由匹配和转发",
			path:            "/web/dashboard",
			expectedService: "web-service",
			description:     "Web路由应该被匹配并转发到Web服务",
		},
		{
			name:            "优先级高的路由被选中",
			path:            "/api/health",
			expectedService: "api-service",
			description:     "优先级高的API路由应该被选中",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建测试请求
			req := httptest.NewRequest("GET", "http://example.com"+tc.path, nil)
			rr := httptest.NewRecorder()

			// 执行请求
			enhancedProxy.ServeHTTP(rr, req)

			// 验证状态码
			if rr.Code != http.StatusOK {
				t.Errorf("%s: 期望状态码 200，实际 %d", tc.name, rr.Code)
				return
			}

			// 验证服务响应
			if rr.Header().Get("X-Service") != tc.expectedService {
				t.Errorf("%s: 期望服务 %s，实际 %s",
					tc.name, tc.expectedService, rr.Header().Get("X-Service"))
				return
			}

			t.Logf(" %s: 通过 - %s", tc.name, tc.description)
		})
	}
}

// testProxyHeadersAcceptance 测试代理头处理功能
func testProxyHeadersAcceptance(t *testing.T) {
	// 创建模拟后端服务器，返回接收到的请求头
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := make(map[string]string)
		for name, values := range r.Header {
			if len(values) > 0 {
				headers[name] = values[0]
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"headers": headers,
			"host":    r.Host,
		})
	}))
	defer backendServer.Close()

	// 创建配置
	cfg := &config.Config{
		Proxy: config.ProxyConfig{
			ConnectTimeout:        5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			KeepAliveTimeout:      30 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			BufferSize:            32768,
		},
	}

	// 创建路由适配器和增强反向代理
	routerAdapter := NewRouterAdapter()
	enhancedProxy, err := NewEnhancedReverseProxy(cfg, routerAdapter)
	if err != nil {
		t.Fatalf("Failed to create enhanced reverse proxy: %v", err)
	}

	// 添加测试路由和上游
	testRoute := &router.RouteRule{
		ID:   "header-test-route",
		Name: "Header Test Route",
		Rules: router.Rule{
			Paths: []router.PathRule{
				{Type: router.MatchTypePrefix, Value: "/"},
			},
		},
		UpstreamID: "header-test-upstream",
		Priority:   100,
	}

	routerAdapter.AddRouteRule(testRoute)
	enhancedProxy.AddUpstream(createUpstreamFromServer("header-test-upstream", "Header Test Upstream", backendServer))

	// 创建测试请求
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req.Header.Set("User-Agent", "Test-Client/1.0")
	req.RemoteAddr = "192.168.1.100:12345"

	rr := httptest.NewRecorder()

	// 执行请求
	enhancedProxy.ServeHTTP(rr, req)

	// 验证状态码
	if rr.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", rr.Code)
		return
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Errorf("无法解析响应JSON: %v", err)
		return
	}

	headers, ok := response["headers"].(map[string]interface{})
	if !ok {
		t.Errorf("响应中缺少headers字段")
		return
	}

	// 验证代理头信息被正确添加
	expectedHeaders := []string{
		"X-Forwarded-For",
		"X-Forwarded-Proto",
		"X-Forwarded-Host",
		"X-Real-Ip", // 注意：HTTP头名称在传输时会被规范化
	}

	allHeadersPresent := true
	for _, headerName := range expectedHeaders {
		found := false
		// 检查各种可能的头名称格式
		for actualHeader := range headers {
			if strings.EqualFold(actualHeader, headerName) {
				found = true
				t.Logf(" 代理头 %s 被正确添加，值为: %v", headerName, headers[actualHeader])
				break
			}
		}
		if !found {
			t.Errorf("缺少代理头 %s", headerName)
			allHeadersPresent = false
		}
	}

	if allHeadersPresent {
		t.Logf(" 代理头信息被正确添加")
	}
}

// createUpstreamFromServer 从httptest.Server创建Upstream
func createUpstreamFromServer(id, name string, server *httptest.Server) *types.Upstream {
	parts := strings.Split(strings.TrimPrefix(server.URL, "http://"), ":")
	host := parts[0]
	port := 80
	if len(parts) == 2 {
		fmt.Sscanf(parts[1], "%d", &port)
	}

	return &types.Upstream{
		ID:        id,
		Name:      name,
		Algorithm: "round_robin",
		Targets: []*types.Target{
			{
				Host:    host,
				Port:    port,
				Weight:  100,
				Healthy: true,
			},
		},
	}
}
