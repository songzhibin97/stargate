package router

import (
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

// MatchType 定义匹配类型
type MatchType string

const (
	MatchTypeExact  MatchType = "exact"  // 精确匹配
	MatchTypePrefix MatchType = "prefix" // 前缀匹配
	MatchTypeRegex  MatchType = "regex"  // 正则匹配
)

// PathRule 路径匹配规则
type PathRule struct {
	Type  MatchType `yaml:"type" json:"type"`
	Value string    `yaml:"value" json:"value"`
}

// HeaderMatchType 请求头匹配类型
type HeaderMatchType string

const (
	HeaderMatchExists    HeaderMatchType = "exists"    // 检查请求头是否存在
	HeaderMatchNotExists HeaderMatchType = "notexists" // 检查请求头是否不存在
	HeaderMatchValue     HeaderMatchType = "value"     // 检查请求头的值
	HeaderMatchRegex     HeaderMatchType = "regex"     // 使用正则表达式匹配请求头的值
)

// HeaderRule 请求头匹配规则
type HeaderRule struct {
	Name      string          `yaml:"name" json:"name"`
	Value     string          `yaml:"value,omitempty" json:"value,omitempty"`
	MatchType HeaderMatchType `yaml:"match_type,omitempty" json:"match_type,omitempty"`
}

// QueryMatchType 查询参数匹配类型
type QueryMatchType string

const (
	QueryMatchExists    QueryMatchType = "exists"    // 检查查询参数是否存在
	QueryMatchNotExists QueryMatchType = "notexists" // 检查查询参数是否不存在
	QueryMatchValue     QueryMatchType = "value"     // 检查查询参数的值
	QueryMatchRegex     QueryMatchType = "regex"     // 使用正则表达式匹配查询参数的值
)

// QueryRule 查询参数匹配规则
type QueryRule struct {
	Name      string         `yaml:"name" json:"name"`
	Value     string         `yaml:"value,omitempty" json:"value,omitempty"`
	MatchType QueryMatchType `yaml:"match_type,omitempty" json:"match_type,omitempty"`
}

// Rule 匹配规则
type Rule struct {
	Hosts   []string            `yaml:"hosts,omitempty" json:"hosts,omitempty"`
	Paths   []PathRule          `yaml:"paths,omitempty" json:"paths,omitempty"`
	Methods []string            `yaml:"methods,omitempty" json:"methods,omitempty"`
	Headers []HeaderRule        `yaml:"headers,omitempty" json:"headers,omitempty"`
	Query   []QueryRule         `yaml:"query,omitempty" json:"query,omitempty"`
	// 保持向后兼容性的简单查询参数匹配
	QueryParams map[string]string `yaml:"query_params,omitempty" json:"query_params,omitempty"`
}

// Target 上游目标
type Target struct {
	URL    string `yaml:"url" json:"url"`
	Weight int    `yaml:"weight,omitempty" json:"weight,omitempty"`
}

// Upstream 上游服务
type Upstream struct {
	ID          string                     `yaml:"id" json:"id"`
	Name        string                     `yaml:"name" json:"name"`
	Targets     []Target                   `yaml:"targets" json:"targets"`
	Algorithm   string                     `yaml:"algorithm,omitempty" json:"algorithm,omitempty"`
	HealthCheck *config.HealthCheckConfig  `yaml:"health_check,omitempty" json:"health_check,omitempty"`
	Metadata    map[string]string          `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt   int64                      `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   int64                      `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// RouteRule 新的路由规则结构（扩展现有Route）
type RouteRule struct {
	ID         string            `yaml:"id" json:"id"`
	Name       string            `yaml:"name" json:"name"`
	Rules      Rule              `yaml:"rules" json:"rules"`
	UpstreamID string            `yaml:"upstream_id" json:"upstream_id"`
	Priority   int               `yaml:"priority,omitempty" json:"priority,omitempty"`
	Metadata   map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	// Developer Portal fields
	OpenAPISpec *OpenAPISpec      `yaml:"openapi_spec,omitempty" json:"openapi_spec,omitempty"`
	CreatedAt   int64             `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   int64             `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

// OpenAPISpec OpenAPI规范配置
type OpenAPISpec struct {
	URL         string            `yaml:"url,omitempty" json:"url,omitempty"`                 // OpenAPI规范文件URL
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`         // OpenAPI版本 (3.0.0, 3.1.0等)
	Title       string            `yaml:"title,omitempty" json:"title,omitempty"`             // API标题
	Description string            `yaml:"description,omitempty" json:"description,omitempty"` // API描述
	Contact     *ContactInfo      `yaml:"contact,omitempty" json:"contact,omitempty"`         // 联系信息
	License     *LicenseInfo      `yaml:"license,omitempty" json:"license,omitempty"`         // 许可证信息
	Tags        []string          `yaml:"tags,omitempty" json:"tags,omitempty"`               // API标签
	Metadata    map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`       // 额外元数据
	// 缓存和同步相关
	LastFetched int64             `yaml:"last_fetched,omitempty" json:"last_fetched,omitempty"` // 最后获取时间
	ETag        string            `yaml:"etag,omitempty" json:"etag,omitempty"`                 // HTTP ETag用于缓存
	Checksum    string            `yaml:"checksum,omitempty" json:"checksum,omitempty"`         // 内容校验和
}

// ContactInfo 联系信息
type ContactInfo struct {
	Name  string `yaml:"name,omitempty" json:"name,omitempty"`
	Email string `yaml:"email,omitempty" json:"email,omitempty"`
	URL   string `yaml:"url,omitempty" json:"url,omitempty"`
}

// LicenseInfo 许可证信息
type LicenseInfo struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	URL  string `yaml:"url,omitempty" json:"url,omitempty"`
}

// RoutingConfig 路由配置
type RoutingConfig struct {
	Routes    []RouteRule `yaml:"routes" json:"routes"`
	Upstreams []Upstream  `yaml:"upstreams" json:"upstreams"`
}

// Validate 验证路由规则
func (r *RouteRule) Validate() error {
	if r.ID == "" {
		return ErrRouteIDEmpty
	}
	if r.Name == "" {
		return ErrRouteNameEmpty
	}
	if r.UpstreamID == "" {
		return ErrUpstreamIDEmpty
	}
	
	// 验证规则至少有一个匹配条件
	if len(r.Rules.Hosts) == 0 && len(r.Rules.Paths) == 0 &&
	   len(r.Rules.Methods) == 0 && len(r.Rules.Headers) == 0 &&
	   len(r.Rules.Query) == 0 && len(r.Rules.QueryParams) == 0 {
		return ErrRuleEmpty
	}
	
	// 验证路径规则
	for _, path := range r.Rules.Paths {
		if path.Value == "" {
			return ErrPathValueEmpty
		}
		if path.Type != MatchTypeExact && path.Type != MatchTypePrefix && path.Type != MatchTypeRegex {
			return ErrInvalidMatchType
		}
	}
	
	// 验证请求头规则
	for _, header := range r.Rules.Headers {
		if header.Name == "" {
			return ErrHeaderNameEmpty
		}
		// 设置默认匹配类型
		if header.MatchType == "" {
			if header.Value == "" {
				header.MatchType = HeaderMatchExists
			} else {
				header.MatchType = HeaderMatchValue
			}
		}
		// 验证匹配类型和值的组合
		if header.MatchType == HeaderMatchValue || header.MatchType == HeaderMatchRegex {
			if header.Value == "" {
				return ErrHeaderValueRequired
			}
		}
	}

	// 验证查询参数规则
	for _, query := range r.Rules.Query {
		if query.Name == "" {
			return ErrQueryNameEmpty
		}
		// 设置默认匹配类型
		if query.MatchType == "" {
			if query.Value == "" {
				query.MatchType = QueryMatchExists
			} else {
				query.MatchType = QueryMatchValue
			}
		}
		// 验证匹配类型和值的组合
		if query.MatchType == QueryMatchValue || query.MatchType == QueryMatchRegex {
			if query.Value == "" {
				return ErrQueryValueRequired
			}
		}
	}
	
	return nil
}

// Validate 验证上游服务
func (u *Upstream) Validate() error {
	if u.ID == "" {
		return ErrUpstreamIDEmpty
	}
	if u.Name == "" {
		return ErrUpstreamNameEmpty
	}
	if len(u.Targets) == 0 {
		return ErrUpstreamTargetsEmpty
	}
	
	// 验证目标
	for _, target := range u.Targets {
		if target.URL == "" {
			return ErrTargetURLEmpty
		}
		if target.Weight < 0 {
			return ErrInvalidWeight
		}
	}
	
	// 验证负载均衡算法
	if u.Algorithm != "" {
		validAlgorithms := map[string]bool{
			"round_robin": true,
			"weighted":    true,
			"ip_hash":     true,
		}
		if !validAlgorithms[u.Algorithm] {
			return ErrInvalidAlgorithm
		}
	}
	
	return nil
}

// Validate 验证路由配置
func (rc *RoutingConfig) Validate() error {
	// 验证路由规则
	routeIDs := make(map[string]bool)
	for _, route := range rc.Routes {
		if err := route.Validate(); err != nil {
			return err
		}
		
		// 检查路由ID唯一性
		if routeIDs[route.ID] {
			return ErrDuplicateRouteID
		}
		routeIDs[route.ID] = true
	}
	
	// 验证上游服务
	upstreamIDs := make(map[string]bool)
	for _, upstream := range rc.Upstreams {
		if err := upstream.Validate(); err != nil {
			return err
		}
		
		// 检查上游服务ID唯一性
		if upstreamIDs[upstream.ID] {
			return ErrDuplicateUpstreamID
		}
		upstreamIDs[upstream.ID] = true
	}
	
	// 验证路由引用的上游服务存在
	for _, route := range rc.Routes {
		if !upstreamIDs[route.UpstreamID] {
			return ErrUpstreamNotFound
		}
	}
	
	return nil
}

// ToLegacyRoute 转换为现有的Route结构（兼容性）
func (r *RouteRule) ToLegacyRoute() *Route {
	// 提取hosts和paths
	var hosts []string
	var paths []string
	var methods []string
	
	hosts = r.Rules.Hosts
	methods = r.Rules.Methods
	
	// 将PathRule转换为简单的字符串路径
	for _, pathRule := range r.Rules.Paths {
		paths = append(paths, pathRule.Value)
	}
	
	return &Route{
		ID:         r.ID,
		Name:       r.Name,
		Hosts:      hosts,
		Paths:      paths,
		Methods:    methods,
		UpstreamID: r.UpstreamID,
		Priority:   r.Priority,
		Metadata:   r.Metadata,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

// SetTimestamps 设置时间戳
func (r *RouteRule) SetTimestamps() {
	now := time.Now().Unix()
	if r.CreatedAt == 0 {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
}

// SetTimestamps 设置上游服务时间戳
func (u *Upstream) SetTimestamps() {
	now := time.Now().Unix()
	if u.CreatedAt == 0 {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
}


