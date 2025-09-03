package router

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	mu     sync.RWMutex
	config *RoutingConfig
}

// NewConfigManager 创建新的配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config: &RoutingConfig{
			Routes:    make([]RouteRule, 0),
			Upstreams: make([]Upstream, 0),
		},
	}
}

// LoadFromFile 从YAML文件加载配置
func (cm *ConfigManager) LoadFromFile(filename string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", ErrConfigFileNotFound, filename)
	}

	// 读取文件内容
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	// 解析YAML
	var config RoutingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidYAMLFormat, err.Error())
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrConfigValidation, err.Error())
	}

	// 设置时间戳
	for i := range config.Routes {
		config.Routes[i].SetTimestamps()
	}
	for i := range config.Upstreams {
		config.Upstreams[i].SetTimestamps()
	}

	// 更新配置
	cm.config = &config

	return nil
}

// LoadFromBytes 从字节数据加载配置
func (cm *ConfigManager) LoadFromBytes(data []byte) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 解析YAML
	var config RoutingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidYAMLFormat, err.Error())
	}

	// 验证配置
	if err := config.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrConfigValidation, err.Error())
	}

	// 设置时间戳
	for i := range config.Routes {
		config.Routes[i].SetTimestamps()
	}
	for i := range config.Upstreams {
		config.Upstreams[i].SetTimestamps()
	}

	// 更新配置
	cm.config = &config

	return nil
}

// SaveToFile 保存配置到YAML文件
func (cm *ConfigManager) SaveToFile(filename string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 创建目录（如果不存在）
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// 序列化为YAML
	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", filename, err)
	}

	return nil
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *RoutingConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 返回配置的深拷贝
	config := &RoutingConfig{
		Routes:    make([]RouteRule, len(cm.config.Routes)),
		Upstreams: make([]Upstream, len(cm.config.Upstreams)),
	}
	copy(config.Routes, cm.config.Routes)
	copy(config.Upstreams, cm.config.Upstreams)

	return config
}

// GetRoutes 获取所有路由规则
func (cm *ConfigManager) GetRoutes() []RouteRule {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	routes := make([]RouteRule, len(cm.config.Routes))
	copy(routes, cm.config.Routes)
	return routes
}

// GetUpstreams 获取所有上游服务
func (cm *ConfigManager) GetUpstreams() []Upstream {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	upstreams := make([]Upstream, len(cm.config.Upstreams))
	copy(upstreams, cm.config.Upstreams)
	return upstreams
}

// GetRoute 根据ID获取路由规则
func (cm *ConfigManager) GetRoute(id string) (*RouteRule, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, route := range cm.config.Routes {
		if route.ID == id {
			// 返回副本
			routeCopy := route
			return &routeCopy, nil
		}
	}

	return nil, fmt.Errorf("route with ID %s not found", id)
}

// GetUpstream 根据ID获取上游服务
func (cm *ConfigManager) GetUpstream(id string) (*Upstream, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, upstream := range cm.config.Upstreams {
		if upstream.ID == id {
			// 返回副本
			upstreamCopy := upstream
			return &upstreamCopy, nil
		}
	}

	return nil, fmt.Errorf("upstream with ID %s not found", id)
}

// AddRoute 添加路由规则
func (cm *ConfigManager) AddRoute(route RouteRule) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 验证路由规则
	if err := route.Validate(); err != nil {
		return err
	}

	// 检查ID是否已存在
	for _, existing := range cm.config.Routes {
		if existing.ID == route.ID {
			return ErrDuplicateRouteID
		}
	}

	// 检查引用的上游服务是否存在
	found := false
	for _, upstream := range cm.config.Upstreams {
		if upstream.ID == route.UpstreamID {
			found = true
			break
		}
	}
	if !found {
		return ErrUpstreamNotFound
	}

	// 设置时间戳
	route.SetTimestamps()

	// 添加路由规则
	cm.config.Routes = append(cm.config.Routes, route)

	return nil
}

// AddUpstream 添加上游服务
func (cm *ConfigManager) AddUpstream(upstream Upstream) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 验证上游服务
	if err := upstream.Validate(); err != nil {
		return err
	}

	// 检查ID是否已存在
	for _, existing := range cm.config.Upstreams {
		if existing.ID == upstream.ID {
			return ErrDuplicateUpstreamID
		}
	}

	// 设置时间戳
	upstream.SetTimestamps()

	// 添加上游服务
	cm.config.Upstreams = append(cm.config.Upstreams, upstream)

	return nil
}

// UpdateRoute 更新路由规则
func (cm *ConfigManager) UpdateRoute(route RouteRule) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 验证路由规则
	if err := route.Validate(); err != nil {
		return err
	}

	// 查找并更新路由规则
	for i, existing := range cm.config.Routes {
		if existing.ID == route.ID {
			// 保留创建时间
			route.CreatedAt = existing.CreatedAt
			route.SetTimestamps()
			cm.config.Routes[i] = route
			return nil
		}
	}

	return fmt.Errorf("route with ID %s not found", route.ID)
}

// UpdateUpstream 更新上游服务
func (cm *ConfigManager) UpdateUpstream(upstream Upstream) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 验证上游服务
	if err := upstream.Validate(); err != nil {
		return err
	}

	// 查找并更新上游服务
	for i, existing := range cm.config.Upstreams {
		if existing.ID == upstream.ID {
			// 保留创建时间
			upstream.CreatedAt = existing.CreatedAt
			upstream.SetTimestamps()
			cm.config.Upstreams[i] = upstream
			return nil
		}
	}

	return fmt.Errorf("upstream with ID %s not found", upstream.ID)
}

// RemoveRoute 删除路由规则
func (cm *ConfigManager) RemoveRoute(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, route := range cm.config.Routes {
		if route.ID == id {
			cm.config.Routes = append(cm.config.Routes[:i], cm.config.Routes[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("route with ID %s not found", id)
}

// RemoveUpstream 删除上游服务
func (cm *ConfigManager) RemoveUpstream(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检查是否有路由规则引用此上游服务
	for _, route := range cm.config.Routes {
		if route.UpstreamID == id {
			return fmt.Errorf("cannot remove upstream %s: it is referenced by route %s", id, route.ID)
		}
	}

	// 删除上游服务
	for i, upstream := range cm.config.Upstreams {
		if upstream.ID == id {
			cm.config.Upstreams = append(cm.config.Upstreams[:i], cm.config.Upstreams[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("upstream with ID %s not found", id)
}

// ValidateConfig 验证当前配置
func (cm *ConfigManager) ValidateConfig() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.config.Validate()
}

// LoadFromDirectory 从目录加载多个配置文件
func (cm *ConfigManager) LoadFromDirectory(dir string) error {
	// 查找目录中的所有YAML文件
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("failed to find YAML files in directory %s: %w", dir, err)
	}

	yamlFiles, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return fmt.Errorf("failed to find YML files in directory %s: %w", dir, err)
	}

	files = append(files, yamlFiles...)

	if len(files) == 0 {
		return fmt.Errorf("no YAML files found in directory %s", dir)
	}

	// 合并所有配置
	var mergedConfig RoutingConfig
	mergedConfig.Routes = make([]RouteRule, 0)
	mergedConfig.Upstreams = make([]Upstream, 0)

	for _, file := range files {
		// 跳过隐藏文件和备份文件
		basename := filepath.Base(file)
		if strings.HasPrefix(basename, ".") || strings.HasSuffix(basename, ".bak") {
			continue
		}

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}

		var config RoutingConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse YAML file %s: %w", file, err)
		}

		// 合并配置
		mergedConfig.Routes = append(mergedConfig.Routes, config.Routes...)
		mergedConfig.Upstreams = append(mergedConfig.Upstreams, config.Upstreams...)
	}

	// 验证合并后的配置
	if err := mergedConfig.Validate(); err != nil {
		return fmt.Errorf("merged configuration validation failed: %w", err)
	}

	// 设置时间戳
	for i := range mergedConfig.Routes {
		mergedConfig.Routes[i].SetTimestamps()
	}
	for i := range mergedConfig.Upstreams {
		mergedConfig.Upstreams[i].SetTimestamps()
	}

	cm.mu.Lock()
	cm.config = &mergedConfig
	cm.mu.Unlock()

	return nil
}
