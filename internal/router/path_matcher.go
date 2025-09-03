package router

import (
	"fmt"
	"regexp"
	"strings"
)

// PathMatcher 路径匹配器接口
type PathMatcher interface {
	Match(requestPath string) bool
	String() string
}

// ExactPathMatcher 精确路径匹配器
type ExactPathMatcher struct {
	path string
}

// NewExactPathMatcher 创建精确路径匹配器
func NewExactPathMatcher(path string) *ExactPathMatcher {
	return &ExactPathMatcher{path: path}
}

// Match 精确匹配路径
func (m *ExactPathMatcher) Match(requestPath string) bool {
	return m.path == requestPath
}

// String 返回匹配器描述
func (m *ExactPathMatcher) String() string {
	return "exact:" + m.path
}

// PrefixPathMatcher 前缀路径匹配器
type PrefixPathMatcher struct {
	prefix string
}

// NewPrefixPathMatcher 创建前缀路径匹配器
func NewPrefixPathMatcher(prefix string) *PrefixPathMatcher {
	return &PrefixPathMatcher{prefix: prefix}
}

// Match 前缀匹配路径
func (m *PrefixPathMatcher) Match(requestPath string) bool {
	return strings.HasPrefix(requestPath, m.prefix)
}

// String 返回匹配器描述
func (m *PrefixPathMatcher) String() string {
	return "prefix:" + m.prefix
}

// RegexPathMatcher 正则表达式路径匹配器
type RegexPathMatcher struct {
	pattern string
	regex   *regexp.Regexp
}

// NewRegexPathMatcher 创建正则表达式路径匹配器
func NewRegexPathMatcher(pattern string) (*RegexPathMatcher, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &RegexPathMatcher{
		pattern: pattern,
		regex:   regex,
	}, nil
}

// Match 正则表达式匹配路径
func (m *RegexPathMatcher) Match(requestPath string) bool {
	return m.regex.MatchString(requestPath)
}

// String 返回匹配器描述
func (m *RegexPathMatcher) String() string {
	return "regex:" + m.pattern
}

// PathMatcherFactory 路径匹配器工厂
type PathMatcherFactory struct{}

// NewPathMatcherFactory 创建路径匹配器工厂
func NewPathMatcherFactory() *PathMatcherFactory {
	return &PathMatcherFactory{}
}

// CreateMatcher 根据PathRule创建对应的路径匹配器
func (f *PathMatcherFactory) CreateMatcher(rule PathRule) (PathMatcher, error) {
	switch rule.Type {
	case MatchTypeExact:
		return NewExactPathMatcher(rule.Value), nil
	case MatchTypePrefix:
		return NewPrefixPathMatcher(rule.Value), nil
	case MatchTypeRegex:
		return NewRegexPathMatcher(rule.Value)
	default:
		return nil, ErrInvalidMatchType
	}
}

// CompiledPathRule 编译后的路径规则
type CompiledPathRule struct {
	Original PathRule
	Matcher  PathMatcher
}

// NewCompiledPathRule 创建编译后的路径规则
func NewCompiledPathRule(rule PathRule) (*CompiledPathRule, error) {
	factory := NewPathMatcherFactory()
	matcher, err := factory.CreateMatcher(rule)
	if err != nil {
		return nil, err
	}
	
	return &CompiledPathRule{
		Original: rule,
		Matcher:  matcher,
	}, nil
}

// Match 匹配请求路径
func (cpr *CompiledPathRule) Match(requestPath string) bool {
	return cpr.Matcher.Match(requestPath)
}

// String 返回规则描述
func (cpr *CompiledPathRule) String() string {
	return cpr.Matcher.String()
}

// PathRuleCompiler 路径规则编译器
type PathRuleCompiler struct {
	factory *PathMatcherFactory
}

// NewPathRuleCompiler 创建路径规则编译器
func NewPathRuleCompiler() *PathRuleCompiler {
	return &PathRuleCompiler{
		factory: NewPathMatcherFactory(),
	}
}

// CompileRules 编译路径规则列表
func (c *PathRuleCompiler) CompileRules(rules []PathRule) ([]*CompiledPathRule, error) {
	compiled := make([]*CompiledPathRule, 0, len(rules))
	
	for i, rule := range rules {
		compiledRule, err := NewCompiledPathRule(rule)
		if err != nil {
			return nil, &PathRuleCompileError{
				Index: i,
				Rule:  rule,
				Err:   err,
			}
		}
		compiled = append(compiled, compiledRule)
	}
	
	return compiled, nil
}

// PathRuleCompileError 路径规则编译错误
type PathRuleCompileError struct {
	Index int
	Rule  PathRule
	Err   error
}

// Error 实现error接口
func (e *PathRuleCompileError) Error() string {
	return fmt.Sprintf("failed to compile path rule at index %d (type=%s, value=%s): %v", 
		e.Index, e.Rule.Type, e.Rule.Value, e.Err)
}

// Unwrap 返回原始错误
func (e *PathRuleCompileError) Unwrap() error {
	return e.Err
}

// PathMatchResult 路径匹配结果
type PathMatchResult struct {
	Matched     bool
	MatchedRule *CompiledPathRule
	RequestPath string
}

// NewPathMatchResult 创建路径匹配结果
func NewPathMatchResult(matched bool, rule *CompiledPathRule, requestPath string) *PathMatchResult {
	return &PathMatchResult{
		Matched:     matched,
		MatchedRule: rule,
		RequestPath: requestPath,
	}
}

// PathMatcher 路径匹配引擎
type PathMatchEngine struct {
	rules []*CompiledPathRule
}

// NewPathMatchEngine 创建路径匹配引擎
func NewPathMatchEngine(rules []PathRule) (*PathMatchEngine, error) {
	compiler := NewPathRuleCompiler()
	compiled, err := compiler.CompileRules(rules)
	if err != nil {
		return nil, err
	}
	
	return &PathMatchEngine{
		rules: compiled,
	}, nil
}

// Match 匹配请求路径，返回第一个匹配的规则
func (e *PathMatchEngine) Match(requestPath string) *PathMatchResult {
	for _, rule := range e.rules {
		if rule.Match(requestPath) {
			return NewPathMatchResult(true, rule, requestPath)
		}
	}
	
	return NewPathMatchResult(false, nil, requestPath)
}

// MatchAll 匹配请求路径，返回所有匹配的规则
func (e *PathMatchEngine) MatchAll(requestPath string) []*PathMatchResult {
	results := make([]*PathMatchResult, 0)
	
	for _, rule := range e.rules {
		if rule.Match(requestPath) {
			results = append(results, NewPathMatchResult(true, rule, requestPath))
		}
	}
	
	return results
}

// GetRules 获取所有编译后的规则
func (e *PathMatchEngine) GetRules() []*CompiledPathRule {
	// 返回副本以防止外部修改
	rules := make([]*CompiledPathRule, len(e.rules))
	copy(rules, e.rules)
	return rules
}

// AddRule 添加新的路径规则
func (e *PathMatchEngine) AddRule(rule PathRule) error {
	compiled, err := NewCompiledPathRule(rule)
	if err != nil {
		return err
	}
	
	e.rules = append(e.rules, compiled)
	return nil
}

// RemoveRule 移除指定索引的路径规则
func (e *PathMatchEngine) RemoveRule(index int) error {
	if index < 0 || index >= len(e.rules) {
		return fmt.Errorf("rule index %d out of range [0, %d)", index, len(e.rules))
	}
	
	e.rules = append(e.rules[:index], e.rules[index+1:]...)
	return nil
}

// Clear 清空所有规则
func (e *PathMatchEngine) Clear() {
	e.rules = e.rules[:0]
}

// Size 返回规则数量
func (e *PathMatchEngine) Size() int {
	return len(e.rules)
}
