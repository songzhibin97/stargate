package router

import "errors"

// 路由相关错误定义
var (
	// 路由规则错误
	ErrRouteIDEmpty       = errors.New("route ID cannot be empty")
	ErrRouteNameEmpty     = errors.New("route name cannot be empty")
	ErrUpstreamIDEmpty    = errors.New("upstream ID cannot be empty")
	ErrRuleEmpty          = errors.New("route rule must have at least one matching condition")
	ErrPathValueEmpty     = errors.New("path value cannot be empty")
	ErrInvalidMatchType   = errors.New("invalid match type, must be exact, prefix, or regex")
	ErrHeaderNameEmpty     = errors.New("header name cannot be empty")
	ErrHeaderValueRequired = errors.New("header value is required for value/regex match type")
	ErrQueryNameEmpty      = errors.New("query parameter name cannot be empty")
	ErrQueryValueRequired  = errors.New("query parameter value is required for value/regex match type")
	ErrDuplicateRouteID    = errors.New("duplicate route ID")
	
	// 上游服务错误
	ErrUpstreamNameEmpty    = errors.New("upstream name cannot be empty")
	ErrUpstreamTargetsEmpty = errors.New("upstream must have at least one target")
	ErrTargetURLEmpty       = errors.New("target URL cannot be empty")
	ErrInvalidWeight        = errors.New("target weight must be non-negative")
	ErrInvalidAlgorithm     = errors.New("invalid load balancing algorithm")
	ErrDuplicateUpstreamID  = errors.New("duplicate upstream ID")
	ErrUpstreamNotFound     = errors.New("referenced upstream not found")
	
	// 配置加载错误
	ErrConfigFileNotFound   = errors.New("configuration file not found")
	ErrInvalidYAMLFormat    = errors.New("invalid YAML format")
	ErrConfigValidation     = errors.New("configuration validation failed")
)
