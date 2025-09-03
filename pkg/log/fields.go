package log

import (
	"time"
)

// Standard field names for consistent logging across the application
const (
	// Core fields
	FieldTimestamp = "timestamp"
	FieldLevel     = "level"
	FieldMessage   = "message"
	FieldCaller    = "caller"
	FieldError     = "error"

	// Request/Response fields
	FieldRequestID     = "request_id"
	FieldUserID        = "user_id"
	FieldSessionID     = "session_id"
	FieldTraceID       = "trace_id"
	FieldSpanID        = "span_id"
	FieldMethod        = "method"
	FieldPath          = "path"
	FieldQuery         = "query"
	FieldStatusCode    = "status_code"
	FieldResponseTime  = "response_time"
	FieldResponseSize  = "response_size"
	FieldClientIP      = "client_ip"
	FieldUserAgent     = "user_agent"
	FieldReferer       = "referer"

	// Service fields
	FieldService     = "service"
	FieldVersion     = "version"
	FieldComponent   = "component"
	FieldEnvironment = "environment"
	FieldHost        = "host"
	FieldPID         = "pid"

	// Business logic fields
	FieldOperation   = "operation"
	FieldResource    = "resource"
	FieldAction      = "action"
	FieldEntity      = "entity"
	FieldEntityID    = "entity_id"
	FieldEventType   = "event_type"
	FieldEventID     = "event_id"

	// Performance fields
	FieldDuration     = "duration"
	FieldLatency      = "latency"
	FieldThroughput   = "throughput"
	FieldCPUUsage     = "cpu_usage"
	FieldMemoryUsage  = "memory_usage"
	FieldDiskUsage    = "disk_usage"
	FieldNetworkUsage = "network_usage"

	// Database fields
	FieldDatabase     = "database"
	FieldTable        = "table"
	FieldQuery_       = "query"
	FieldRowsAffected = "rows_affected"
	FieldConnectionID = "connection_id"

	// Cache fields
	FieldCacheKey    = "cache_key"
	FieldCacheHit    = "cache_hit"
	FieldCacheTTL    = "cache_ttl"
	FieldCacheSize   = "cache_size"

	// Security fields
	FieldAuthMethod   = "auth_method"
	FieldPermissions  = "permissions"
	FieldSecurityRole = "security_role"
	FieldIPAddress    = "ip_address"

	// Load balancer fields
	FieldUpstream     = "upstream"
	FieldBackend      = "backend"
	FieldAlgorithm    = "algorithm"
	FieldHealthCheck  = "health_check"
	FieldWeight       = "weight"

	// Circuit breaker fields
	FieldCircuitState = "circuit_state"
	FieldFailureRate  = "failure_rate"
	FieldThreshold    = "threshold"

	// Rate limiting fields
	FieldRateLimit    = "rate_limit"
	FieldTokens       = "tokens"
	FieldBucketSize   = "bucket_size"
	FieldRefillRate   = "refill_rate"
)

// Standard field helper functions for common logging patterns

// RequestFields creates standard request logging fields
func RequestFields(requestID, userID, method, path string) []Field {
	return []Field{
		String(FieldRequestID, requestID),
		String(FieldUserID, userID),
		String(FieldMethod, method),
		String(FieldPath, path),
	}
}

// ResponseFields creates standard response logging fields
func ResponseFields(statusCode int, responseSize int64, duration time.Duration) []Field {
	return []Field{
		Int(FieldStatusCode, statusCode),
		Int64(FieldResponseSize, responseSize),
		Duration(FieldResponseTime, duration),
	}
}

// ServiceFields creates standard service logging fields
func ServiceFields(service, version, component string) []Field {
	return []Field{
		String(FieldService, service),
		String(FieldVersion, version),
		String(FieldComponent, component),
	}
}

// ErrorFields creates standard error logging fields
func ErrorFields(err error, operation, resource string) []Field {
	fields := []Field{
		Error(err),
		String(FieldOperation, operation),
	}
	if resource != "" {
		fields = append(fields, String(FieldResource, resource))
	}
	return fields
}

// PerformanceFields creates standard performance logging fields
func PerformanceFields(duration time.Duration, cpuUsage, memoryUsage float64) []Field {
	return []Field{
		Duration(FieldDuration, duration),
		Float64(FieldCPUUsage, cpuUsage),
		Float64(FieldMemoryUsage, memoryUsage),
	}
}

// DatabaseFields creates standard database logging fields
func DatabaseFields(database, table, query string, rowsAffected int64) []Field {
	return []Field{
		String(FieldDatabase, database),
		String(FieldTable, table),
		String(FieldQuery_, query),
		Int64(FieldRowsAffected, rowsAffected),
	}
}

// CacheFields creates standard cache logging fields
func CacheFields(key string, hit bool, ttl time.Duration) []Field {
	return []Field{
		String(FieldCacheKey, key),
		Bool(FieldCacheHit, hit),
		Duration(FieldCacheTTL, ttl),
	}
}

// SecurityFields creates standard security logging fields
func SecurityFields(userID, authMethod, role, ipAddress string) []Field {
	return []Field{
		String(FieldUserID, userID),
		String(FieldAuthMethod, authMethod),
		String(FieldSecurityRole, role),
		String(FieldIPAddress, ipAddress),
	}
}

// LoadBalancerFields creates standard load balancer logging fields
func LoadBalancerFields(upstream, algorithm string, weight int) []Field {
	return []Field{
		String(FieldUpstream, upstream),
		String(FieldAlgorithm, algorithm),
		Int(FieldWeight, weight),
	}
}

// CircuitBreakerFields creates standard circuit breaker logging fields
func CircuitBreakerFields(state string, failureRate float64, threshold int) []Field {
	return []Field{
		String(FieldCircuitState, state),
		Float64(FieldFailureRate, failureRate),
		Int(FieldThreshold, threshold),
	}
}

// RateLimitFields creates standard rate limiting logging fields
func RateLimitFields(limit int, tokens int, bucketSize int) []Field {
	return []Field{
		Int(FieldRateLimit, limit),
		Int(FieldTokens, tokens),
		Int(FieldBucketSize, bucketSize),
	}
}

// TraceFields creates standard tracing logging fields
func TraceFields(traceID, spanID string) []Field {
	return []Field{
		String(FieldTraceID, traceID),
		String(FieldSpanID, spanID),
	}
}

// BusinessEventFields creates standard business event logging fields
func BusinessEventFields(eventType, eventID, entity, entityID string) []Field {
	return []Field{
		String(FieldEventType, eventType),
		String(FieldEventID, eventID),
		String(FieldEntity, entity),
		String(FieldEntityID, entityID),
	}
}

// HTTPClientFields creates standard HTTP client logging fields
func HTTPClientFields(method, url string, statusCode int, duration time.Duration) []Field {
	return []Field{
		String(FieldMethod, method),
		String("url", url),
		Int(FieldStatusCode, statusCode),
		Duration(FieldDuration, duration),
	}
}

// HealthCheckFields creates standard health check logging fields
func HealthCheckFields(service, endpoint string, healthy bool, duration time.Duration) []Field {
	return []Field{
		String(FieldService, service),
		String("endpoint", endpoint),
		Bool("healthy", healthy),
		Duration(FieldDuration, duration),
	}
}

// ConfigFields creates standard configuration logging fields
func ConfigFields(configType, key, source string) []Field {
	return []Field{
		String("config_type", configType),
		String("config_key", key),
		String("config_source", source),
	}
}

// MetricsFields creates standard metrics logging fields
func MetricsFields(metricName string, value float64, tags map[string]string) []Field {
	fields := []Field{
		String("metric_name", metricName),
		Float64("metric_value", value),
	}
	
	if tags != nil {
		fields = append(fields, Any("metric_tags", tags))
	}
	
	return fields
}

// StartupFields creates standard application startup logging fields
func StartupFields(appName, version, environment string, startTime time.Time) []Field {
	return []Field{
		String("app_name", appName),
		String(FieldVersion, version),
		String(FieldEnvironment, environment),
		Time("start_time", startTime),
	}
}

// ShutdownFields creates standard application shutdown logging fields
func ShutdownFields(reason string, uptime time.Duration) []Field {
	return []Field{
		String("shutdown_reason", reason),
		Duration("uptime", uptime),
	}
}
