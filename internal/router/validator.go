package router

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Validator 配置验证器
type Validator struct {
	strictMode bool
}

// NewValidator 创建新的验证器
func NewValidator(strictMode bool) *Validator {
	return &Validator{
		strictMode: strictMode,
	}
}

// ValidateRoutingConfig 验证完整的路由配置
func (v *Validator) ValidateRoutingConfig(config *RoutingConfig) error {
	if config == nil {
		return fmt.Errorf("routing config cannot be nil")
	}

	// 验证路由规则
	routeIDs := make(map[string]bool)
	routeNames := make(map[string]bool)
	
	for i, route := range config.Routes {
		if err := v.ValidateRouteRule(&route); err != nil {
			return fmt.Errorf("route[%d] validation failed: %w", i, err)
		}
		
		// 检查路由ID唯一性
		if routeIDs[route.ID] {
			return fmt.Errorf("duplicate route ID: %s", route.ID)
		}
		routeIDs[route.ID] = true
		
		// 在严格模式下检查路由名称唯一性
		if v.strictMode {
			if routeNames[route.Name] {
				return fmt.Errorf("duplicate route name: %s", route.Name)
			}
			routeNames[route.Name] = true
		}
	}
	
	// 验证上游服务
	upstreamIDs := make(map[string]bool)
	upstreamNames := make(map[string]bool)
	
	for i, upstream := range config.Upstreams {
		if err := v.ValidateUpstream(&upstream); err != nil {
			return fmt.Errorf("upstream[%d] validation failed: %w", i, err)
		}
		
		// 检查上游服务ID唯一性
		if upstreamIDs[upstream.ID] {
			return fmt.Errorf("duplicate upstream ID: %s", upstream.ID)
		}
		upstreamIDs[upstream.ID] = true
		
		// 在严格模式下检查上游服务名称唯一性
		if v.strictMode {
			if upstreamNames[upstream.Name] {
				return fmt.Errorf("duplicate upstream name: %s", upstream.Name)
			}
			upstreamNames[upstream.Name] = true
		}
	}
	
	// 验证路由引用的上游服务存在
	for _, route := range config.Routes {
		if !upstreamIDs[route.UpstreamID] {
			return fmt.Errorf("route %s references non-existent upstream: %s", route.ID, route.UpstreamID)
		}
	}
	
	return nil
}

// ValidateRouteRule 验证路由规则
func (v *Validator) ValidateRouteRule(route *RouteRule) error {
	if route == nil {
		return fmt.Errorf("route rule cannot be nil")
	}
	
	// 基本字段验证
	if err := v.validateBasicFields(route); err != nil {
		return err
	}
	
	// 验证匹配规则
	if err := v.ValidateRule(&route.Rules); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}
	
	// 验证优先级
	if route.Priority < 0 {
		return fmt.Errorf("route priority cannot be negative")
	}
	
	// 在严格模式下进行额外验证
	if v.strictMode {
		if err := v.validateRouteRuleStrict(route); err != nil {
			return err
		}
	}
	
	return nil
}

// ValidateRule 验证匹配规则
func (v *Validator) ValidateRule(rule *Rule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}
	
	// 检查是否至少有一个匹配条件
	if len(rule.Hosts) == 0 && len(rule.Paths) == 0 &&
	   len(rule.Methods) == 0 && len(rule.Headers) == 0 &&
	   len(rule.Query) == 0 && len(rule.QueryParams) == 0 {
		return fmt.Errorf("rule must have at least one matching condition")
	}
	
	// 验证主机规则
	for i, host := range rule.Hosts {
		if err := v.validateHost(host); err != nil {
			return fmt.Errorf("hosts[%d] validation failed: %w", i, err)
		}
	}
	
	// 验证路径规则
	for i, path := range rule.Paths {
		if err := v.validatePathRule(&path); err != nil {
			return fmt.Errorf("paths[%d] validation failed: %w", i, err)
		}
	}
	
	// 验证HTTP方法
	for i, method := range rule.Methods {
		if err := v.validateMethod(method); err != nil {
			return fmt.Errorf("methods[%d] validation failed: %w", i, err)
		}
	}
	
	// 验证请求头规则
	for i, header := range rule.Headers {
		if err := v.validateHeaderRule(&header); err != nil {
			return fmt.Errorf("headers[%d] validation failed: %w", i, err)
		}
	}
	
	// 验证查询参数规则
	for _, queryRule := range rule.Query {
		if err := v.validateQueryRule(queryRule); err != nil {
			return fmt.Errorf("query parameter validation failed: %w", err)
		}
	}

	// 验证向后兼容的查询参数规则
	for key, value := range rule.QueryParams {
		if err := v.validateQueryParam(key, value); err != nil {
			return fmt.Errorf("query parameter validation failed: %w", err)
		}
	}
	
	return nil
}

// ValidateUpstream 验证上游服务
func (v *Validator) ValidateUpstream(upstream *Upstream) error {
	if upstream == nil {
		return fmt.Errorf("upstream cannot be nil")
	}
	
	// 基本字段验证
	if upstream.ID == "" {
		return fmt.Errorf("upstream ID cannot be empty")
	}
	if upstream.Name == "" {
		return fmt.Errorf("upstream name cannot be empty")
	}
	if len(upstream.Targets) == 0 {
		return fmt.Errorf("upstream must have at least one target")
	}
	
	// 验证目标
	for i, target := range upstream.Targets {
		if err := v.validateTarget(&target); err != nil {
			return fmt.Errorf("targets[%d] validation failed: %w", i, err)
		}
	}
	
	// 验证负载均衡算法
	if upstream.Algorithm != "" {
		if err := v.validateAlgorithm(upstream.Algorithm); err != nil {
			return err
		}
	}
	
	// 在严格模式下进行额外验证
	if v.strictMode {
		if err := v.validateUpstreamStrict(upstream); err != nil {
			return err
		}
	}
	
	return nil
}

// validateBasicFields 验证基本字段
func (v *Validator) validateBasicFields(route *RouteRule) error {
	if route.ID == "" {
		return fmt.Errorf("route ID cannot be empty")
	}
	if route.Name == "" {
		return fmt.Errorf("route name cannot be empty")
	}
	if route.UpstreamID == "" {
		return fmt.Errorf("upstream ID cannot be empty")
	}
	
	// 验证ID格式（只允许字母、数字、连字符和下划线）
	if !isValidID(route.ID) {
		return fmt.Errorf("invalid route ID format: %s", route.ID)
	}
	if !isValidID(route.UpstreamID) {
		return fmt.Errorf("invalid upstream ID format: %s", route.UpstreamID)
	}
	
	return nil
}

// validateHost 验证主机名
func (v *Validator) validateHost(host string) error {
	if host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	
	// 检查是否包含通配符
	if strings.Contains(host, "*") {
		// 验证通配符格式
		if !isValidWildcardHost(host) {
			return fmt.Errorf("invalid wildcard host format: %s", host)
		}
	} else {
		// 验证普通主机名格式
		if !isValidHostname(host) {
			return fmt.Errorf("invalid hostname format: %s", host)
		}
	}
	
	return nil
}

// validatePathRule 验证路径规则
func (v *Validator) validatePathRule(path *PathRule) error {
	if path.Value == "" {
		return fmt.Errorf("path value cannot be empty")
	}
	
	// 验证匹配类型
	switch path.Type {
	case MatchTypeExact, MatchTypePrefix:
		// 验证路径格式
		if !strings.HasPrefix(path.Value, "/") {
			return fmt.Errorf("path must start with '/': %s", path.Value)
		}
	case MatchTypeRegex:
		// 验证正则表达式
		if _, err := regexp.Compile(path.Value); err != nil {
			return fmt.Errorf("invalid regex pattern: %s, error: %w", path.Value, err)
		}
	default:
		return fmt.Errorf("invalid match type: %s", path.Type)
	}
	
	return nil
}

// validateMethod 验证HTTP方法
func (v *Validator) validateMethod(method string) error {
	if method == "" {
		return fmt.Errorf("method cannot be empty")
	}
	
	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true, "TRACE": true,
	}
	
	upperMethod := strings.ToUpper(method)
	if !validMethods[upperMethod] {
		return fmt.Errorf("invalid HTTP method: %s", method)
	}
	
	return nil
}

// validateHeaderRule 验证请求头规则
func (v *Validator) validateHeaderRule(header *HeaderRule) error {
	if header.Name == "" {
		return fmt.Errorf("header name cannot be empty")
	}
	
	// 验证请求头名称格式
	if !isValidHeaderName(header.Name) {
		return fmt.Errorf("invalid header name format: %s", header.Name)
	}
	
	return nil
}

// validateQueryParam 验证查询参数
func (v *Validator) validateQueryParam(key, value string) error {
	if key == "" {
		return fmt.Errorf("query parameter key cannot be empty")
	}
	
	// 验证参数名格式
	if !isValidQueryParamName(key) {
		return fmt.Errorf("invalid query parameter name: %s", key)
	}
	
	return nil
}

// validateTarget 验证目标服务器
func (v *Validator) validateTarget(target *Target) error {
	if target.URL == "" {
		return fmt.Errorf("target URL cannot be empty")
	}
	
	// 验证URL格式
	parsedURL, err := url.Parse(target.URL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %s, error: %w", target.URL, err)
	}
	
	// 检查协议
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("target URL must use http or https scheme: %s", target.URL)
	}
	
	// 检查主机
	if parsedURL.Host == "" {
		return fmt.Errorf("target URL must have a host: %s", target.URL)
	}
	
	// 验证权重
	if target.Weight < 0 {
		return fmt.Errorf("target weight cannot be negative: %d", target.Weight)
	}
	
	return nil
}

// validateAlgorithm 验证负载均衡算法
func (v *Validator) validateAlgorithm(algorithm string) error {
	validAlgorithms := map[string]bool{
		"round_robin": true,
		"weighted":    true,
		"ip_hash":     true,
		"least_conn":  true,
	}
	
	if !validAlgorithms[algorithm] {
		return fmt.Errorf("invalid load balancing algorithm: %s", algorithm)
	}
	
	return nil
}

// validateRouteRuleStrict 严格模式下的路由规则验证
func (v *Validator) validateRouteRuleStrict(route *RouteRule) error {
	// 检查名称长度
	if len(route.Name) > 100 {
		return fmt.Errorf("route name too long (max 100 characters): %s", route.Name)
	}
	
	// 检查优先级范围
	if route.Priority > 1000 {
		return fmt.Errorf("route priority too high (max 1000): %d", route.Priority)
	}
	
	return nil
}

// validateUpstreamStrict 严格模式下的上游服务验证
func (v *Validator) validateUpstreamStrict(upstream *Upstream) error {
	// 检查名称长度
	if len(upstream.Name) > 100 {
		return fmt.Errorf("upstream name too long (max 100 characters): %s", upstream.Name)
	}
	
	// 检查目标数量
	if len(upstream.Targets) > 50 {
		return fmt.Errorf("too many targets (max 50): %d", len(upstream.Targets))
	}
	
	return nil
}

// 辅助函数
func isValidID(id string) bool {
	if len(id) == 0 || len(id) > 50 {
		return false
	}
	
	for _, r := range id {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
			 (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	
	return true
}

func isValidWildcardHost(host string) bool {
	// 简单的通配符主机名验证
	return strings.HasPrefix(host, "*.") && len(host) > 2
}

func isValidHostname(host string) bool {
	// 简单的主机名验证
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	
	// 基本格式检查
	return !strings.Contains(host, " ") && !strings.HasPrefix(host, ".") && !strings.HasSuffix(host, ".")
}

func isValidHeaderName(name string) bool {
	// HTTP头名称验证
	if len(name) == 0 {
		return false
	}
	
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
			 (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	
	return true
}

func isValidQueryParamName(name string) bool {
	// 查询参数名称验证
	if len(name) == 0 {
		return false
	}
	
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
			 (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	
	return true
}

// validateQueryRule 验证查询参数规则
func (v *Validator) validateQueryRule(queryRule QueryRule) error {
	// 验证查询参数名称
	if queryRule.Name == "" {
		return fmt.Errorf("query parameter name cannot be empty")
	}

	if !isValidQueryParamName(queryRule.Name) {
		return fmt.Errorf("invalid query parameter name: %s", queryRule.Name)
	}

	// 验证匹配类型和值的组合
	switch queryRule.MatchType {
	case QueryMatchValue, QueryMatchRegex:
		if queryRule.Value == "" {
			return fmt.Errorf("query parameter value is required for match type %s", queryRule.MatchType)
		}
		// 如果是正则表达式，验证正则表达式的有效性
		if queryRule.MatchType == QueryMatchRegex {
			if _, err := regexp.Compile(queryRule.Value); err != nil {
				return fmt.Errorf("invalid regex pattern for query parameter %s: %w", queryRule.Name, err)
			}
		}
	case QueryMatchExists, QueryMatchNotExists:
		// 这些匹配类型不需要值
	case "":
		// 空匹配类型，使用默认逻辑
	default:
		return fmt.Errorf("invalid query match type: %s", queryRule.MatchType)
	}

	return nil
}
