package router

import (
	"net/http"
	"regexp"
	"sort"
	"strings"
)

// EnhancedRoute 增强的路由结构，支持新的PathRule
type EnhancedRoute struct {
	*RouteRule
	compiledPaths []*CompiledPathRule
	hostRegexes   []*regexp.Regexp
	methods       map[string]bool
}

// NewEnhancedRoute 创建增强路由
func NewEnhancedRoute(rule *RouteRule) (*EnhancedRoute, error) {
	enhanced := &EnhancedRoute{
		RouteRule: rule,
		methods:   make(map[string]bool),
	}

	// 编译路径规则
	if len(rule.Rules.Paths) > 0 {
		compiler := NewPathRuleCompiler()
		compiled, err := compiler.CompileRules(rule.Rules.Paths)
		if err != nil {
			return nil, err
		}
		enhanced.compiledPaths = compiled
	}

	// 编译主机正则表达式
	for _, host := range rule.Rules.Hosts {
		regex, err := compileHostPattern(host)
		if err != nil {
			return nil, err
		}
		enhanced.hostRegexes = append(enhanced.hostRegexes, regex)
	}

	// 预处理HTTP方法
	for _, method := range rule.Rules.Methods {
		enhanced.methods[strings.ToUpper(method)] = true
	}

	return enhanced, nil
}

// MatchRequest 匹配HTTP请求
func (er *EnhancedRoute) MatchRequest(req *http.Request) bool {
	// 检查主机匹配
	if len(er.Rules.Hosts) > 0 && !er.matchHost(req.Host) {
		return false
	}

	// 检查路径匹配
	if len(er.Rules.Paths) > 0 && !er.matchPath(req.URL.Path) {
		return false
	}

	// 检查HTTP方法匹配
	if len(er.Rules.Methods) > 0 && !er.matchMethod(req.Method) {
		return false
	}

	// 检查请求头匹配
	if len(er.Rules.Headers) > 0 && !er.matchHeaders(req.Header) {
		return false
	}

	// 检查查询参数匹配
	if (len(er.Rules.Query) > 0 || len(er.Rules.QueryParams) > 0) && !er.matchQuery(req.URL.Query()) {
		return false
	}

	return true
}

// matchHost 匹配主机名
func (er *EnhancedRoute) matchHost(host string) bool {
	// 移除端口号
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	for _, regex := range er.hostRegexes {
		if regex.MatchString(host) {
			return true
		}
	}
	return false
}

// matchPath 匹配路径
func (er *EnhancedRoute) matchPath(path string) bool {
	for _, compiledPath := range er.compiledPaths {
		if compiledPath.Match(path) {
			return true
		}
	}
	return false
}

// matchMethod 匹配HTTP方法
func (er *EnhancedRoute) matchMethod(method string) bool {
	return er.methods[strings.ToUpper(method)]
}

// matchHeaders 匹配请求头
func (er *EnhancedRoute) matchHeaders(headers http.Header) bool {
	for _, headerRule := range er.Rules.Headers {
		if !er.matchSingleHeader(headers, headerRule) {
			return false
		}
	}
	return true
}

// matchSingleHeader 匹配单个请求头规则
func (er *EnhancedRoute) matchSingleHeader(headers http.Header, headerRule HeaderRule) bool {
	headerValues := headers.Values(headerRule.Name)

	switch headerRule.MatchType {
	case HeaderMatchExists:
		// 检查请求头是否存在
		return len(headerValues) > 0

	case HeaderMatchNotExists:
		// 检查请求头是否不存在
		return len(headerValues) == 0

	case HeaderMatchValue:
		// 检查请求头的值是否完全匹配
		if len(headerValues) == 0 {
			return false
		}
		for _, value := range headerValues {
			if value == headerRule.Value {
				return true
			}
		}
		return false

	case HeaderMatchRegex:
		// 使用正则表达式匹配请求头的值
		if headerRule.Value == "" || len(headerValues) == 0 {
			return false
		}
		regex, err := regexp.Compile(headerRule.Value)
		if err != nil {
			// 正则表达式编译失败，记录错误并返回false
			return false
		}
		for _, value := range headerValues {
			if regex.MatchString(value) {
				return true
			}
		}
		return false

	default:
		// 向后兼容：如果没有指定匹配类型，使用旧逻辑
		if len(headerValues) == 0 {
			return false
		}
		// 如果规则中的值为空，只检查请求头是否存在
		if headerRule.Value == "" {
			return true
		}
		// 检查是否有任何值匹配
		for _, value := range headerValues {
			if value == headerRule.Value {
				return true
			}
		}
		return false
	}
}

// matchQuery 匹配查询参数
func (er *EnhancedRoute) matchQuery(query map[string][]string) bool {
	// 匹配新的QueryRule格式
	for _, queryRule := range er.Rules.Query {
		if !er.matchSingleQuery(query, queryRule) {
			return false
		}
	}

	// 向后兼容：匹配旧的QueryParams格式
	for key, expectedValue := range er.Rules.QueryParams {
		actualValues, exists := query[key]
		if !exists {
			return false
		}

		// 检查是否有任何值匹配
		matched := false
		for _, actualValue := range actualValues {
			if expectedValue == "" || actualValue == expectedValue {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// matchSingleQuery 匹配单个查询参数规则
func (er *EnhancedRoute) matchSingleQuery(query map[string][]string, queryRule QueryRule) bool {
	actualValues, exists := query[queryRule.Name]

	switch queryRule.MatchType {
	case QueryMatchExists:
		// 检查查询参数是否存在
		return exists && len(actualValues) > 0

	case QueryMatchNotExists:
		// 检查查询参数是否不存在
		return !exists || len(actualValues) == 0

	case QueryMatchValue:
		// 检查查询参数的值是否完全匹配
		if !exists || len(actualValues) == 0 {
			return false
		}
		for _, actualValue := range actualValues {
			if actualValue == queryRule.Value {
				return true
			}
		}
		return false

	case QueryMatchRegex:
		// 使用正则表达式匹配查询参数的值
		if queryRule.Value == "" || !exists || len(actualValues) == 0 {
			return false
		}
		regex, err := regexp.Compile(queryRule.Value)
		if err != nil {
			// 正则表达式编译失败，记录错误并返回false
			return false
		}
		for _, actualValue := range actualValues {
			if regex.MatchString(actualValue) {
				return true
			}
		}
		return false

	default:
		// 向后兼容：如果没有指定匹配类型，使用旧逻辑
		if !exists || len(actualValues) == 0 {
			return false
		}
		// 如果规则中的值为空，只检查查询参数是否存在
		if queryRule.Value == "" {
			return true
		}
		// 检查是否有任何值匹配
		for _, actualValue := range actualValues {
			if actualValue == queryRule.Value {
				return true
			}
		}
		return false
	}
}

// EnhancedMatchResult 增强的匹配结果
type EnhancedMatchResult struct {
	Route       *EnhancedRoute
	Matched     bool
	MatchedPath *CompiledPathRule
	Params      map[string]string
	Metadata    map[string]string
}

// NewEnhancedMatchResult 创建增强匹配结果
func NewEnhancedMatchResult(route *EnhancedRoute, matched bool) *EnhancedMatchResult {
	result := &EnhancedMatchResult{
		Route:    route,
		Matched:  matched,
		Params:   make(map[string]string),
		Metadata: make(map[string]string),
	}

	if matched && route != nil {
		// 复制元数据
		for k, v := range route.Metadata {
			result.Metadata[k] = v
		}
	}

	return result
}

// EnhancedRouter 增强的路由器
type EnhancedRouter struct {
	routes []*EnhancedRoute
}

// NewEnhancedRouter 创建增强路由器
func NewEnhancedRouter() *EnhancedRouter {
	return &EnhancedRouter{
		routes: make([]*EnhancedRoute, 0),
	}
}

// AddRoute 添加路由规则
func (er *EnhancedRouter) AddRoute(rule *RouteRule) error {
	enhanced, err := NewEnhancedRoute(rule)
	if err != nil {
		return err
	}

	er.routes = append(er.routes, enhanced)
	
	// 按优先级排序（优先级高的在前）
	er.sortRoutes()
	
	return nil
}

// AddRoutes 批量添加路由规则
func (er *EnhancedRouter) AddRoutes(rules []RouteRule) error {
	for _, rule := range rules {
		if err := er.AddRoute(&rule); err != nil {
			return err
		}
	}
	return nil
}

// RemoveRoute 移除路由规则
func (er *EnhancedRouter) RemoveRoute(routeID string) bool {
	for i, route := range er.routes {
		if route.ID == routeID {
			er.routes = append(er.routes[:i], er.routes[i+1:]...)
			return true
		}
	}
	return false
}

// Match 匹配HTTP请求
func (er *EnhancedRouter) Match(req *http.Request) *EnhancedMatchResult {
	for _, route := range er.routes {
		if route.MatchRequest(req) {
			result := NewEnhancedMatchResult(route, true)
			
			// 找到匹配的路径规则
			for _, compiledPath := range route.compiledPaths {
				if compiledPath.Match(req.URL.Path) {
					result.MatchedPath = compiledPath
					break
				}
			}
			
			return result
		}
	}
	
	return NewEnhancedMatchResult(nil, false)
}

// MatchAll 匹配所有符合条件的路由
func (er *EnhancedRouter) MatchAll(req *http.Request) []*EnhancedMatchResult {
	results := make([]*EnhancedMatchResult, 0)
	
	for _, route := range er.routes {
		if route.MatchRequest(req) {
			result := NewEnhancedMatchResult(route, true)
			
			// 找到匹配的路径规则
			for _, compiledPath := range route.compiledPaths {
				if compiledPath.Match(req.URL.Path) {
					result.MatchedPath = compiledPath
					break
				}
			}
			
			results = append(results, result)
		}
	}
	
	return results
}

// GetRoutes 获取所有路由
func (er *EnhancedRouter) GetRoutes() []*EnhancedRoute {
	routes := make([]*EnhancedRoute, len(er.routes))
	copy(routes, er.routes)
	return routes
}

// Clear 清空所有路由
func (er *EnhancedRouter) Clear() {
	er.routes = er.routes[:0]
}

// Size 返回路由数量
func (er *EnhancedRouter) Size() int {
	return len(er.routes)
}

// sortRoutes 按优先级排序路由（优先级高的在前）
func (er *EnhancedRouter) sortRoutes() {
	sort.Slice(er.routes, func(i, j int) bool {
		return er.routes[i].Priority > er.routes[j].Priority
	})
}

// compileHostPattern 编译主机模式为正则表达式
func compileHostPattern(pattern string) (*regexp.Regexp, error) {
	// 处理通配符主机名
	if strings.HasPrefix(pattern, "*.") {
		// *.example.com -> ^[^.]+\.example\.com$
		// 通配符必须匹配至少一个子域名，不能匹配根域名
		domain := regexp.QuoteMeta(pattern[2:])
		regexPattern := "^[^.]+\\." + domain + "$"
		return regexp.Compile(regexPattern)
	}

	// 精确匹配
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	return regexp.Compile(regexPattern)
}
