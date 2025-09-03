package router

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigManager_LoadFromBytes(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errType error
	}{
		{
			name: "valid config",
			yaml: `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
      paths:
        - type: "prefix"
          value: "/api"
      methods: ["GET", "POST"]
    upstream_id: "test-upstream"
    priority: 100

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend1.example.com"
        weight: 100
    algorithm: "round_robin"
`,
			wantErr: false,
		},
		{
			name: "invalid yaml format",
			yaml: `
routes:
  - id: "test-route"
    name: "Test Route"
    invalid_yaml: [
`,
			wantErr: true,
			errType: ErrInvalidYAMLFormat,
		},
		{
			name: "missing upstream",
			yaml: `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
    upstream_id: "non-existent-upstream"

upstreams: []
`,
			wantErr: true,
			errType: ErrConfigValidation,
		},
		{
			name: "duplicate route id",
			yaml: `
routes:
  - id: "duplicate-id"
    name: "Route 1"
    rules:
      hosts: ["example1.com"]
    upstream_id: "test-upstream"
  - id: "duplicate-id"
    name: "Route 2"
    rules:
      hosts: ["example2.com"]
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
`,
			wantErr: true,
			errType: ErrConfigValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewConfigManager()
			err := cm.LoadFromBytes([]byte(tt.yaml))

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadFromBytes() expected error but got none")
					return
				}
				if tt.errType != nil && !isErrorType(err, tt.errType) {
					t.Errorf("LoadFromBytes() error = %v, want error type %v", err, tt.errType)
				}
			} else {
				if err != nil {
					t.Errorf("LoadFromBytes() error = %v, want nil", err)
					return
				}

				// 验证配置是否正确加载
				config := cm.GetConfig()
				if len(config.Routes) == 0 {
					t.Errorf("LoadFromBytes() no routes loaded")
				}
				if len(config.Upstreams) == 0 {
					t.Errorf("LoadFromBytes() no upstreams loaded")
				}
			}
		})
	}
}

func TestConfigManager_LoadFromFile(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "stargate-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 测试用例
	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
		errType  error
	}{
		{
			name:     "valid config file",
			filename: "valid.yaml",
			content: `
routes:
  - id: "api-route"
    name: "API Route"
    rules:
      paths:
        - type: "prefix"
          value: "/api"
    upstream_id: "api-upstream"

upstreams:
  - id: "api-upstream"
    name: "API Upstream"
    targets:
      - url: "http://api.example.com"
`,
			wantErr: false,
		},
		{
			name:     "non-existent file",
			filename: "non-existent.yaml",
			wantErr:  true,
			errType:  ErrConfigFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.filename)

			// 创建测试文件（如果有内容）
			if tt.content != "" {
				if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			cm := NewConfigManager()
			err := cm.LoadFromFile(filePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadFromFile() expected error but got none")
					return
				}
				if tt.errType != nil && !isErrorType(err, tt.errType) {
					t.Errorf("LoadFromFile() error = %v, want error type %v", err, tt.errType)
				}
			} else {
				if err != nil {
					t.Errorf("LoadFromFile() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestConfigManager_AddRoute(t *testing.T) {
	cm := NewConfigManager()

	// 先添加上游服务
	upstream := Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []Target{
			{URL: "http://backend.example.com", Weight: 100},
		},
	}
	if err := cm.AddUpstream(upstream); err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	tests := []struct {
		name    string
		route   RouteRule
		wantErr bool
	}{
		{
			name: "valid route",
			route: RouteRule{
				ID:   "test-route",
				Name: "Test Route",
				Rules: Rule{
					Hosts: []string{"example.com"},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: false,
		},
		{
			name: "duplicate route id",
			route: RouteRule{
				ID:   "test-route", // 重复ID
				Name: "Another Route",
				Rules: Rule{
					Hosts: []string{"another.com"},
				},
				UpstreamID: "test-upstream",
			},
			wantErr: true,
		},
		{
			name: "non-existent upstream",
			route: RouteRule{
				ID:   "route-2",
				Name: "Route 2",
				Rules: Rule{
					Hosts: []string{"example2.com"},
				},
				UpstreamID: "non-existent",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.AddRoute(tt.route)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AddRoute() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("AddRoute() error = %v, want nil", err)
				}

				// 验证路由是否添加成功
				route, err := cm.GetRoute(tt.route.ID)
				if err != nil {
					t.Errorf("GetRoute() error = %v", err)
				}
				if route.ID != tt.route.ID {
					t.Errorf("GetRoute() ID = %v, want %v", route.ID, tt.route.ID)
				}
			}
		})
	}
}

func TestConfigManager_AddUpstream(t *testing.T) {
	cm := NewConfigManager()

	tests := []struct {
		name     string
		upstream Upstream
		wantErr  bool
	}{
		{
			name: "valid upstream",
			upstream: Upstream{
				ID:   "test-upstream",
				Name: "Test Upstream",
				Targets: []Target{
					{URL: "http://backend.example.com", Weight: 100},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate upstream id",
			upstream: Upstream{
				ID:   "test-upstream", // 重复ID
				Name: "Another Upstream",
				Targets: []Target{
					{URL: "http://another.example.com", Weight: 100},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid upstream - no targets",
			upstream: Upstream{
				ID:      "upstream-2",
				Name:    "Upstream 2",
				Targets: []Target{}, // 空目标列表
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cm.AddUpstream(tt.upstream)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AddUpstream() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("AddUpstream() error = %v, want nil", err)
				}

				// 验证上游服务是否添加成功
				upstream, err := cm.GetUpstream(tt.upstream.ID)
				if err != nil {
					t.Errorf("GetUpstream() error = %v", err)
				}
				if upstream.ID != tt.upstream.ID {
					t.Errorf("GetUpstream() ID = %v, want %v", upstream.ID, tt.upstream.ID)
				}
			}
		})
	}
}

func TestConfigManager_SaveToFile(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "stargate-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm := NewConfigManager()

	// 添加测试数据
	upstream := Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []Target{
			{URL: "http://backend.example.com", Weight: 100},
		},
	}
	if err := cm.AddUpstream(upstream); err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	route := RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: Rule{
			Hosts: []string{"example.com"},
		},
		UpstreamID: "test-upstream",
	}
	if err := cm.AddRoute(route); err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 保存到文件
	filePath := filepath.Join(tempDir, "config.yaml")
	if err := cm.SaveToFile(filePath); err != nil {
		t.Errorf("SaveToFile() error = %v, want nil", err)
	}

	// 验证文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("SaveToFile() file not created")
	}

	// 验证文件内容是否可以重新加载
	cm2 := NewConfigManager()
	if err := cm2.LoadFromFile(filePath); err != nil {
		t.Errorf("LoadFromFile() after SaveToFile() error = %v", err)
	}

	// 验证数据一致性
	config := cm2.GetConfig()
	if len(config.Routes) != 1 {
		t.Errorf("LoadFromFile() routes count = %d, want 1", len(config.Routes))
	}
	if len(config.Upstreams) != 1 {
		t.Errorf("LoadFromFile() upstreams count = %d, want 1", len(config.Upstreams))
	}
}

func TestConfigManager_RemoveUpstream(t *testing.T) {
	cm := NewConfigManager()

	// 添加上游服务
	upstream := Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []Target{
			{URL: "http://backend.example.com", Weight: 100},
		},
	}
	if err := cm.AddUpstream(upstream); err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 添加引用该上游服务的路由
	route := RouteRule{
		ID:   "test-route",
		Name: "Test Route",
		Rules: Rule{
			Hosts: []string{"example.com"},
		},
		UpstreamID: "test-upstream",
	}
	if err := cm.AddRoute(route); err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// 尝试删除被引用的上游服务（应该失败）
	if err := cm.RemoveUpstream("test-upstream"); err == nil {
		t.Errorf("RemoveUpstream() expected error when upstream is referenced")
	}

	// 先删除路由
	if err := cm.RemoveRoute("test-route"); err != nil {
		t.Errorf("RemoveRoute() error = %v", err)
	}

	// 再删除上游服务（应该成功）
	if err := cm.RemoveUpstream("test-upstream"); err != nil {
		t.Errorf("RemoveUpstream() error = %v", err)
	}

	// 验证上游服务已被删除
	if _, err := cm.GetUpstream("test-upstream"); err == nil {
		t.Errorf("GetUpstream() expected error after removal")
	}
}

func TestConfigManager_Concurrency(t *testing.T) {
	cm := NewConfigManager()

	// 添加基础上游服务
	upstream := Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []Target{
			{URL: "http://backend.example.com", Weight: 100},
		},
	}
	if err := cm.AddUpstream(upstream); err != nil {
		t.Fatalf("Failed to add upstream: %v", err)
	}

	// 并发读写测试
	done := make(chan bool, 10)

	// 启动多个goroutine进行并发操作
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			route := RouteRule{
				ID:   fmt.Sprintf("route-%d", id),
				Name: fmt.Sprintf("Route %d", id),
				Rules: Rule{
					Hosts: []string{fmt.Sprintf("example%d.com", id)},
				},
				UpstreamID: "test-upstream",
			}

			if err := cm.AddRoute(route); err != nil {
				t.Errorf("Concurrent AddRoute() error = %v", err)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		go func() {
			defer func() { done <- true }()

			// 并发读取配置
			config := cm.GetConfig()
			if config == nil {
				t.Errorf("Concurrent GetConfig() returned nil")
			}
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证最终状态
	config := cm.GetConfig()
	if len(config.Routes) != 5 {
		t.Errorf("Concurrent operations resulted in %d routes, want 5", len(config.Routes))
	}
}

// 辅助函数
func isErrorType(err error, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return strings.Contains(err.Error(), target.Error())
}

// 基准测试
func BenchmarkConfigManager_LoadFromBytes(b *testing.B) {
	yaml := `
routes:
  - id: "test-route"
    name: "Test Route"
    rules:
      hosts: ["example.com"]
      paths:
        - type: "prefix"
          value: "/api"
    upstream_id: "test-upstream"

upstreams:
  - id: "test-upstream"
    name: "Test Upstream"
    targets:
      - url: "http://backend.example.com"
`

	data := []byte(yaml)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cm := NewConfigManager()
		if err := cm.LoadFromBytes(data); err != nil {
			b.Fatalf("LoadFromBytes() error = %v", err)
		}
	}
}

func BenchmarkConfigManager_GetConfig(b *testing.B) {
	cm := NewConfigManager()

	// 添加测试数据
	upstream := Upstream{
		ID:   "test-upstream",
		Name: "Test Upstream",
		Targets: []Target{
			{URL: "http://backend.example.com", Weight: 100},
		},
	}
	cm.AddUpstream(upstream)

	for i := 0; i < 100; i++ {
		route := RouteRule{
			ID:   fmt.Sprintf("route-%d", i),
			Name: fmt.Sprintf("Route %d", i),
			Rules: Rule{
				Hosts: []string{fmt.Sprintf("example%d.com", i)},
			},
			UpstreamID: "test-upstream",
		}
		cm.AddRoute(route)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		config := cm.GetConfig()
		if config == nil {
			b.Fatalf("GetConfig() returned nil")
		}
	}
}
