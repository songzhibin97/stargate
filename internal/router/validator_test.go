package router

import (
	"testing"
)

func TestValidator_ValidateRouteRule(t *testing.T) {
	validator := NewValidator(false)

	tests := []struct {
		name    string
		route   *RouteRule
		wantErr bool
	}{
		{
			name: "valid route",
			route: &RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Hosts: []string{"example.com"},
					Paths: []PathRule{
						{Type: MatchTypePrefix, Value: "/api"},
					},
					Methods: []string{"GET", "POST"},
				},
				UpstreamID: "test-upstream",
				Priority:   100,
			},
			wantErr: false,
		},
		{
			name: "empty route ID",
			route: &RouteRule{
				ID:   "",
				Name: "Test Route",
				Rules: Rule{
					Hosts: []string{"example.com"},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
		{
			name: "empty route name",
			route: &RouteRule{
				ID:   "test-route",
				Name: "",
				Rules: Rule{
					Hosts: []string{"example.com"},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
		{
			name: "empty upstream ID",
			route: &RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Hosts: []string{"example.com"},
				},
				UpstreamID: "",
			},
			wantErr: true,
		},
		{
			name: "empty rules",
			route: &RouteRule{
				ID:         "test-route",
				Name:       "Test Route",
				Rules:      Rule{}, // 空规则
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
		{
			name: "negative priority",
			route: &RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Hosts: []string{"example.com"},
				},
				UpstreamID: "test-upstream",
				Priority:   -1,
			},
			wantErr: true,
		},
		{
			name: "invalid path rule - empty value",
			route: &RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Paths: []PathRule{
						{Type: MatchTypePrefix, Value: ""}, // 空路径值
					},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
		{
			name: "invalid path rule - invalid match type",
			route: &RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Paths: []PathRule{
						{Type: "invalid", Value: "/api"},
					},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
		{
			name: "invalid header rule - empty name",
			route: &RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Headers: []HeaderRule{
						{Name: "", Value: "test"}, // 空请求头名称
					},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRouteRule(tt.route)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRouteRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateUpstream(t *testing.T) {
	validator := NewValidator(false)

	tests := []struct {
		name     string
		upstream *Upstream
		wantErr  bool
	}{
		{
			name: "valid upstream",
			upstream: &Upstream{
				ID:   "test-upstream",
				Name: "Test Upstream",
				Targets: []Target{
					{URL: "http://backend1.example.com", Weight: 100},
					{URL: "https://backend2.example.com", Weight: 50},
				},
				Algorithm: "round_robin",
			},
			wantErr: false,
		},
		{
			name: "empty upstream ID",
			upstream: &Upstream{
				ID:   "",
				Name: "Test Upstream",
				Targets: []Target{
					{URL: "http://backend.example.com", Weight: 100},
				},
			},
			wantErr: true,
		},
		{
			name: "empty upstream name",
			upstream: &Upstream{
				ID:   "test-upstream",
				Name: "",
				Targets: []Target{
					{URL: "http://backend.example.com", Weight: 100},
				},
			},
			wantErr: true,
		},
		{
			name: "no targets",
			upstream: &Upstream{
				ID:      "test-upstream",
				Name:    "Test Upstream",
				Targets: []Target{}, // 空目标列表
			},
			wantErr: true,
		},
		{
			name: "invalid target URL",
			upstream: &Upstream{
				ID:   "test-upstream",
				Name: "Test Upstream",
				Targets: []Target{
					{URL: "", Weight: 100}, // 空URL
				},
			},
			wantErr: true,
		},
		{
			name: "negative weight",
			upstream: &Upstream{
				ID:   "test-upstream",
				Name: "Test Upstream",
				Targets: []Target{
					{URL: "http://backend.example.com", Weight: -1}, // 负权重
				},
			},
			wantErr: true,
		},
		{
			name: "invalid algorithm",
			upstream: &Upstream{
				ID:   "test-upstream",
				Name: "Test Upstream",
				Targets: []Target{
					{URL: "http://backend.example.com", Weight: 100},
				},
				Algorithm: "invalid_algorithm",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateUpstream(tt.upstream)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpstream() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateRule(t *testing.T) {
	validator := NewValidator(false)

	tests := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "valid rule with hosts",
			rule: &Rule{
				Hosts: []string{"example.com", "*.example.com"},
			},
			wantErr: false,
		},
		{
			name: "valid rule with paths",
			rule: &Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "/api"},
					{Type: MatchTypeExact, Value: "/health"},
					{Type: MatchTypeRegex, Value: "^/users/[0-9]+$"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid rule with methods",
			rule: &Rule{
				Methods: []string{"GET", "POST", "PUT", "DELETE"},
			},
			wantErr: false,
		},
		{
			name: "valid rule with headers",
			rule: &Rule{
				Headers: []HeaderRule{
					{Name: "X-API-Key", Value: "secret"},
					{Name: "Content-Type", Value: "application/json"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid rule with query params",
			rule: &Rule{
				QueryParams: map[string]string{
					"version": "v1",
					"format":  "json",
				},
			},
			wantErr: false,
		},
		{
			name: "empty rule",
			rule: &Rule{}, // 没有任何匹配条件
			wantErr: true,
		},
		{
			name: "invalid host",
			rule: &Rule{
				Hosts: []string{"invalid host with spaces"},
			},
			wantErr: true,
		},
		{
			name: "invalid path - doesn't start with /",
			rule: &Rule{
				Paths: []PathRule{
					{Type: MatchTypePrefix, Value: "api"}, // 不以/开头
				},
			},
			wantErr: true,
		},
		{
			name: "invalid regex path",
			rule: &Rule{
				Paths: []PathRule{
					{Type: MatchTypeRegex, Value: "[invalid regex"}, // 无效正则表达式
				},
			},
			wantErr: true,
		},
		{
			name: "invalid HTTP method",
			rule: &Rule{
				Methods: []string{"INVALID_METHOD"},
			},
			wantErr: true,
		},
		{
			name: "invalid header name",
			rule: &Rule{
				Headers: []HeaderRule{
					{Name: "Invalid Header Name!", Value: "test"}, // 包含非法字符
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_ValidateRoutingConfig(t *testing.T) {
	validator := NewValidator(false)

	tests := []struct {
		name    string
		config  *RoutingConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &RoutingConfig{
				Routes: []RouteRule{
					{
						ID:   "route1",
						Name: "Route 1",
						Rules: Rule{
							Hosts: []string{"example.com"},
						},
						UpstreamID: "upstream1",
					},
				},
				Upstreams: []Upstream{
					{
						ID:   "upstream1",
						Name: "Upstream 1",
						Targets: []Target{
							{URL: "http://backend.example.com", Weight: 100},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "duplicate route IDs",
			config: &RoutingConfig{
				Routes: []RouteRule{
					{
						ID:   "duplicate",
						Name: "Route 1",
						Rules: Rule{
							Hosts: []string{"example1.com"},
						},
						UpstreamID: "upstream1",
					},
					{
						ID:   "duplicate", // 重复ID
						Name: "Route 2",
						Rules: Rule{
							Hosts: []string{"example2.com"},
						},
						UpstreamID: "upstream1",
					},
				},
				Upstreams: []Upstream{
					{
						ID:   "upstream1",
						Name: "Upstream 1",
						Targets: []Target{
							{URL: "http://backend.example.com", Weight: 100},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate upstream IDs",
			config: &RoutingConfig{
				Routes: []RouteRule{
					{
						ID:   "route1",
						Name: "Route 1",
						Rules: Rule{
							Hosts: []string{"example.com"},
						},
						UpstreamID: "upstream1",
					},
				},
				Upstreams: []Upstream{
					{
						ID:   "duplicate",
						Name: "Upstream 1",
						Targets: []Target{
							{URL: "http://backend1.example.com", Weight: 100},
						},
					},
					{
						ID:   "duplicate", // 重复ID
						Name: "Upstream 2",
						Targets: []Target{
							{URL: "http://backend2.example.com", Weight: 100},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "route references non-existent upstream",
			config: &RoutingConfig{
				Routes: []RouteRule{
					{
						ID:   "route1",
						Name: "Route 1",
						Rules: Rule{
							Hosts: []string{"example.com"},
						},
						UpstreamID: "non-existent", // 不存在的上游服务
					},
				},
				Upstreams: []Upstream{
					{
						ID:   "upstream1",
						Name: "Upstream 1",
						Targets: []Target{
							{URL: "http://backend.example.com", Weight: 100},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRoutingConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoutingConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidator_StrictMode(t *testing.T) {
	strictValidator := NewValidator(true)
	normalValidator := NewValidator(false)

	// 测试严格模式下的额外验证
	config := &RoutingConfig{
		Routes: []RouteRule{
			{
				ID:   "route1",
				Name: "Route 1",
				Rules: Rule{
					Hosts: []string{"example.com"},
				},
				UpstreamID: "upstream1",
				Priority:   1001, // 超过严格模式限制
			},
			{
				ID:   "route2",
				Name: "Route 1", // 重复名称
				Rules: Rule{
					Hosts: []string{"example2.com"},
				},
				UpstreamID: "upstream1",
			},
		},
		Upstreams: []Upstream{
			{
				ID:   "upstream1",
				Name: "Upstream 1",
				Targets: []Target{
					{URL: "http://backend.example.com", Weight: 100},
				},
			},
		},
	}

	// 普通模式应该通过
	if err := normalValidator.ValidateRoutingConfig(config); err != nil {
		t.Errorf("Normal validator should pass, but got error: %v", err)
	}

	// 严格模式应该失败
	if err := strictValidator.ValidateRoutingConfig(config); err == nil {
		t.Errorf("Strict validator should fail, but passed")
	}
}
