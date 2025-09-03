package config

import "time"

// Config represents the complete configuration structure
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Controller     ControllerConfig     `yaml:"controller"`
	Portal         PortalConfig         `yaml:"portal"`
	Gateway        GatewayConfig        `yaml:"gateway"`
	Proxy          ProxyConfig          `yaml:"proxy"`
	LoadBalancer   LoadBalancerConfig   `yaml:"load_balancer"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	TrafficMirror  TrafficMirrorConfig  `yaml:"traffic_mirror"`
	Auth           AuthConfig           `yaml:"auth"`
	IPACL          IPACLConfig          `yaml:"ip_acl"`
	CORS           CORSConfig           `yaml:"cors"`
	HeaderTransform HeaderTransformConfig `yaml:"header_transform"`
	MockResponse   MockResponseConfig   `yaml:"mock_response"`
	GRPCWeb        GRPCWebConfig        `yaml:"grpc_web"`
	Logging        LoggingConfig        `yaml:"logging"`
	Metrics        MetricsConfig        `yaml:"metrics"`
	Tracing        TracingConfig        `yaml:"tracing"`
	Store          StoreConfig          `yaml:"store"`
	ConfigSource   ConfigSourceConfig   `yaml:"config"`
	Sync           SyncConfig           `yaml:"sync"`
	AdminAPI       AdminAPIConfig       `yaml:"admin_api"`
	Routes         RoutesConfig         `yaml:"routes"`
	Upstreams      UpstreamsConfig      `yaml:"upstreams"`
	Plugins        PluginsConfig        `yaml:"plugins"`
	Webhooks       WebhooksConfig       `yaml:"webhooks"`
	Aggregator     AggregatorConfig     `yaml:"aggregator"`
	Serverless     ServerlessConfig     `yaml:"serverless"`
	WASM           WASMConfig           `yaml:"wasm"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Address        string        `yaml:"address"`
	HTTPSAddress   string        `yaml:"https_address"`
	TLS            TLSConfig     `yaml:"tls"`
	Timeout        time.Duration `yaml:"timeout"`
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
}

// ControllerConfig represents controller server configuration
type ControllerConfig struct {
	Address      string        `yaml:"address"`
	HTTPSAddress string        `yaml:"https_address"`
	TLS          TLSConfig     `yaml:"tls"`
	Timeout      time.Duration `yaml:"timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool       `yaml:"enabled"`
	CertFile string     `yaml:"cert_file"`
	KeyFile  string     `yaml:"key_file"`
	CAFile   string     `yaml:"ca_file"`
	ACME     ACMEConfig `yaml:"acme"`
}

// ACMEConfig represents ACME (Let's Encrypt) configuration
type ACMEConfig struct {
	Enabled     bool     `yaml:"enabled"`
	Domains     []string `yaml:"domains"`
	Email       string   `yaml:"email"`
	CacheDir    string   `yaml:"cache_dir"`
	DirectoryURL string  `yaml:"directory_url"`
	AcceptTOS   bool     `yaml:"accept_tos"`
}

// ProxyConfig represents proxy configuration
type ProxyConfig struct {
	BufferSize               int           `yaml:"buffer_size"`
	PoolSize                 int           `yaml:"pool_size"`
	ConnectTimeout           time.Duration `yaml:"connect_timeout"`
	ResponseHeaderTimeout    time.Duration `yaml:"response_header_timeout"`
	KeepAliveTimeout         time.Duration `yaml:"keep_alive_timeout"`
	MaxIdleConns             int           `yaml:"max_idle_conns"`
	MaxIdleConnsPerHost      int           `yaml:"max_idle_conns_per_host"`
	WebSocket                WebSocketConfig `yaml:"websocket"`
}

// WebSocketConfig represents WebSocket proxy configuration
type WebSocketConfig struct {
	Enabled           bool          `yaml:"enabled"`
	BufferSize        int           `yaml:"buffer_size"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	PingInterval      time.Duration `yaml:"ping_interval"`
	PongTimeout       time.Duration `yaml:"pong_timeout"`
	MaxConnections    int           `yaml:"max_connections"`
	CompressionLevel  int           `yaml:"compression_level"`
}

// LoadBalancerConfig represents load balancer configuration
type LoadBalancerConfig struct {
	DefaultAlgorithm string            `yaml:"default_algorithm"`
	HealthCheck      HealthCheckConfig `yaml:"health_check"`
	Canary           CanaryConfig      `yaml:"canary"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled             bool                    `yaml:"enabled"`
	Interval            time.Duration           `yaml:"interval"`
	Timeout             time.Duration           `yaml:"timeout"`
	HealthyThreshold    int                     `yaml:"healthy_threshold"`
	UnhealthyThreshold  int                     `yaml:"unhealthy_threshold"`
	Path                string                  `yaml:"path"`
	Passive             PassiveHealthCheckConfig `yaml:"passive"`
}

// PassiveHealthCheckConfig represents passive health check configuration
type PassiveHealthCheckConfig struct {
	Enabled              bool          `yaml:"enabled"`
	ConsecutiveFailures  int           `yaml:"consecutive_failures"`
	IsolationDuration    time.Duration `yaml:"isolation_duration"`
	RecoveryInterval     time.Duration `yaml:"recovery_interval"`
	ConsecutiveSuccesses int           `yaml:"consecutive_successes"`
	FailureStatusCodes   []int         `yaml:"failure_status_codes"`
	TimeoutAsFailure     bool          `yaml:"timeout_as_failure"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled            bool                    `yaml:"enabled"`
	DefaultRate        int                     `yaml:"default_rate"`
	Burst              int                     `yaml:"burst"`
	Storage            string                  `yaml:"storage"`
	Redis              RedisConfig             `yaml:"redis"`
	Strategy           string                  `yaml:"strategy"`           // fixed_window, sliding_window, token_bucket, leaky_bucket
	IdentifierStrategy string                  `yaml:"identifier_strategy"` // ip, user, api_key, combined
	WindowSize         time.Duration           `yaml:"window_size"`
	CleanupInterval    time.Duration           `yaml:"cleanup_interval"`
	SkipSuccessful     bool                    `yaml:"skip_successful_requests"`
	SkipFailed         bool                    `yaml:"skip_failed_requests"`
	CustomHeaders      map[string]string       `yaml:"custom_headers"`
	ExcludedPaths      []string                `yaml:"excluded_paths"`
	ExcludedIPs        []string                `yaml:"excluded_ips"`
	PerRoute           map[string]RouteRateLimit `yaml:"per_route"`
}

// RouteRateLimit represents per-route rate limiting configuration
type RouteRateLimit struct {
	Enabled     bool          `yaml:"enabled"`
	MaxRequests int           `yaml:"max_requests"`
	WindowSize  time.Duration `yaml:"window_size"`
	Strategy    string        `yaml:"strategy"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// CanaryConfig represents canary deployment configuration
type CanaryConfig struct {
	Enabled bool                    `yaml:"enabled"`
	Groups  map[string]*CanaryGroup `yaml:"groups"`
}

// CanaryGroup represents a canary deployment group
type CanaryGroup struct {
	GroupID   string                 `yaml:"group_id"`
	Strategy  string                 `yaml:"strategy"` // "weighted", "percentage", "header_based"
	Versions  []*CanaryVersionConfig `yaml:"versions"`
	Rules     []*CanaryRule          `yaml:"rules,omitempty"`
}

// CanaryVersionConfig represents canary version configuration
type CanaryVersionConfig struct {
	Version    string            `yaml:"version"`
	UpstreamID string            `yaml:"upstream_id"`
	Weight     int               `yaml:"weight"`
	Percentage float64           `yaml:"percentage"`
	Metadata   map[string]string `yaml:"metadata,omitempty"`
}

// CanaryRule represents canary routing rule
type CanaryRule struct {
	Type     string            `yaml:"type"`     // "header", "cookie", "query", "ip"
	Key      string            `yaml:"key"`      // rule key name
	Value    string            `yaml:"value"`    // rule value
	Version  string            `yaml:"version"`  // target version
	Metadata map[string]string `yaml:"metadata,omitempty"`
}

// TrafficMirrorConfig represents traffic mirroring configuration
type TrafficMirrorConfig struct {
	Enabled            bool                    `yaml:"enabled"`
	LogMirrorRequests  bool                    `yaml:"log_mirror_requests"`
	Mirrors            []*MirrorTargetConfig   `yaml:"mirrors"`
}

// MirrorTargetConfig represents mirror target configuration
type MirrorTargetConfig struct {
	ID         string            `yaml:"id"`
	Name       string            `yaml:"name"`
	URL        string            `yaml:"url"`
	SampleRate float64           `yaml:"sample_rate"` // 0.0 - 1.0
	Timeout    time.Duration     `yaml:"timeout"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	Enabled    bool              `yaml:"enabled"`
	Metadata   map[string]string `yaml:"metadata,omitempty"`
}

// CircuitBreakerConfig represents circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled                    bool          `yaml:"enabled"`
	FailureThreshold           int           `yaml:"failure_threshold"`
	RecoveryTimeout            time.Duration `yaml:"recovery_timeout"`
	RequestVolumeThreshold     int           `yaml:"request_volume_threshold"`
	ErrorPercentageThreshold   int           `yaml:"error_percentage_threshold"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Enabled bool           `yaml:"enabled"`
	JWT     JWTConfig      `yaml:"jwt"`
	APIKey  APIKeyConfig   `yaml:"api_key"`
	OAuth2  OAuth2Config   `yaml:"oauth2"`
}

// JWTConfig represents JWT configuration
type JWTConfig struct {
	Secret    string        `yaml:"secret"`
	PublicKey string        `yaml:"public_key"`
	Algorithm string        `yaml:"algorithm"`
	ExpiresIn time.Duration `yaml:"expires_in"`
	Issuer    string        `yaml:"issuer"`
	Audience  string        `yaml:"audience"`
	JWKSURL   string        `yaml:"jwks_url"`
}

// APIKeyConfig represents API key configuration
type APIKeyConfig struct {
	Header string   `yaml:"header"`
	Query  string   `yaml:"query"`
	Keys   []string `yaml:"keys"`
}

// OAuth2Config represents OAuth 2.0 configuration
type OAuth2Config struct {
	IntrospectionURL string            `yaml:"introspection_url"`
	ClientID         string            `yaml:"client_id"`
	ClientSecret     string            `yaml:"client_secret"`
	TokenTypeHint    string            `yaml:"token_type_hint"`
	Timeout          time.Duration     `yaml:"timeout"`
	MaxRetries       int               `yaml:"max_retries"`
	RetryDelay       time.Duration     `yaml:"retry_delay"`
	CacheEnabled     bool              `yaml:"cache_enabled"`
	CacheTTL         time.Duration     `yaml:"cache_ttl"`
	Headers          map[string]string `yaml:"headers"`
}

// IPACLConfig represents IP access control configuration
type IPACLConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Whitelist []string `yaml:"whitelist"`
	Blacklist []string `yaml:"blacklist"`
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled          bool          `yaml:"enabled"`
	AllowAllOrigins  bool          `yaml:"allow_all_origins"`
	AllowedOrigins   []string      `yaml:"allowed_origins"`
	AllowedMethods   []string      `yaml:"allowed_methods"`
	AllowedHeaders   []string      `yaml:"allowed_headers"`
	ExposedHeaders   []string      `yaml:"exposed_headers"`
	AllowCredentials bool          `yaml:"allow_credentials"`
	MaxAge           time.Duration `yaml:"max_age"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level     string        `yaml:"level"`
	Format    string        `yaml:"format"`
	Output    string        `yaml:"output"`
	AccessLog AccessLogConfig `yaml:"access_log"`
	AuditLog  AuditLogConfig  `yaml:"audit_log"`
}

// AccessLogConfig represents access log configuration
type AccessLogConfig struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format"`
	Output  string `yaml:"output"`
}

// AuditLogConfig represents audit log configuration
type AuditLogConfig struct {
	Enabled bool   `yaml:"enabled"`
	Output  string `yaml:"output"`
}

// MetricsConfig represents metrics configuration
type MetricsConfig struct {
	Enabled    bool             `yaml:"enabled"`
	Path       string           `yaml:"path"`
	Prometheus PrometheusConfig `yaml:"prometheus"`

	// New unified metrics configuration
	Provider          string                  `yaml:"provider"`                     // Provider type (prometheus, statsd, etc.)
	Namespace         string                  `yaml:"namespace"`
	Subsystem         string                  `yaml:"subsystem"`
	ConstLabels       map[string]string       `yaml:"const_labels"`
	EnabledMetrics    map[string]bool         `yaml:"enabled_metrics"`             // Enable/disable specific metrics
	CustomLabels      map[string]string       `yaml:"custom_labels"`               // Custom labels to add
	SampleRate        float64                 `yaml:"sample_rate"`                 // Sampling rate for high traffic
	LabelExtractors   map[string]string       `yaml:"label_extractors"`            // Custom label extraction rules
	SensitiveLabels   []string                `yaml:"sensitive_labels"`            // Labels to filter out
	MaxLabelLength    int                     `yaml:"max_label_length"`            // Maximum label value length
	AsyncUpdates      bool                    `yaml:"async_updates"`               // Enable async metric updates
	BufferSize        int                     `yaml:"buffer_size"`                 // Buffer size for async updates
}

// PrometheusConfig represents Prometheus configuration (kept for backward compatibility)
type PrometheusConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Namespace string `yaml:"namespace"`
	Subsystem string `yaml:"subsystem"`
}

// TracingConfig represents tracing configuration
type TracingConfig struct {
	Enabled bool          `yaml:"enabled"`
	Jaeger  JaegerConfig  `yaml:"jaeger"`
}

// JaegerConfig represents Jaeger configuration
type JaegerConfig struct {
	Endpoint    string  `yaml:"endpoint"`
	ServiceName string  `yaml:"service_name"`
	SampleRate  float64 `yaml:"sample_rate"`
}

// StoreConfig represents configuration store settings
type StoreConfig struct {
	Type      string      `yaml:"type"`
	Etcd      EtcdConfig  `yaml:"etcd"`
	KeyPrefix string      `yaml:"key_prefix"`
	Watch     bool        `yaml:"watch"`
}

// EtcdConfig represents etcd configuration
type EtcdConfig struct {
	Endpoints []string      `yaml:"endpoints"`
	Timeout   time.Duration `yaml:"timeout"`
	TLS       TLSConfig     `yaml:"tls"`
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
}

// ConfigSourceConfig represents configuration source settings for data plane
type ConfigSourceConfig struct {
	Source SourceConfig `yaml:"source"`
}

// SourceConfig represents the configuration source driver settings
type SourceConfig struct {
	Driver       string                 `yaml:"driver"`       // "file" or "etcd"
	File         FileSourceConfig       `yaml:"file"`         // File source configuration
	Etcd         EtcdSourceConfig       `yaml:"etcd"`         // Etcd source configuration
	PollInterval time.Duration          `yaml:"poll_interval"` // Polling interval for file source
}

// FileSourceConfig represents file-based configuration source settings
type FileSourceConfig struct {
	Path         string        `yaml:"path"`          // Path to the configuration file
	PollInterval time.Duration `yaml:"poll_interval"` // How often to check for file changes
}

// EtcdSourceConfig represents etcd-based configuration source settings
type EtcdSourceConfig struct {
	Endpoints []string      `yaml:"endpoints"`     // Etcd endpoints
	Key       string        `yaml:"key"`           // Etcd key to watch
	Timeout   time.Duration `yaml:"timeout"`       // Connection timeout
	TLS       TLSConfig     `yaml:"tls"`           // TLS configuration
	Username  string        `yaml:"username"`      // Authentication username
	Password  string        `yaml:"password"`      // Authentication password
}

// SyncConfig represents synchronization configuration
type SyncConfig struct {
	Interval   time.Duration    `yaml:"interval"`
	GitOps     GitOpsConfig     `yaml:"gitops"`
	Validation ValidationConfig `yaml:"validation"`
}

// GitOpsConfig represents GitOps configuration
type GitOpsConfig struct {
	Enabled      bool          `yaml:"enabled"`
	Repository   string        `yaml:"repository"`
	Branch       string        `yaml:"branch"`
	Path         string        `yaml:"path"`
	SSHKeyFile   string        `yaml:"ssh_key_file"`
	PollInterval time.Duration `yaml:"poll_interval"`
}

// ValidationConfig represents validation configuration
type ValidationConfig struct {
	Enabled bool `yaml:"enabled"`
	Strict  bool `yaml:"strict"`
}

// AdminAPIConfig represents Admin API configuration
type AdminAPIConfig struct {
	REST RESTConfig `yaml:"rest"`
	GRPC GRPCConfig `yaml:"grpc"`
	Auth AuthConfig `yaml:"auth"`
}

// RESTConfig represents REST API configuration
type RESTConfig struct {
	Enabled bool   `yaml:"enabled"`
	Prefix  string `yaml:"prefix"`
}

// GRPCConfig represents gRPC API configuration
type GRPCConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

// RoutesConfig represents routes configuration
type RoutesConfig struct {
	Defaults RouteDefaults `yaml:"defaults"`
}

// RouteDefaults represents default route settings
type RouteDefaults struct {
	Timeout      time.Duration `yaml:"timeout"`
	Retries      int           `yaml:"retries"`
	RetryTimeout time.Duration `yaml:"retry_timeout"`
}

// UpstreamsConfig represents upstreams configuration
type UpstreamsConfig struct {
	Defaults UpstreamDefaults `yaml:"defaults"`
}

// UpstreamDefaults represents default upstream settings
type UpstreamDefaults struct {
	Algorithm   string            `yaml:"algorithm"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// PluginsConfig represents plugins configuration
type PluginsConfig struct {
	Directory string                 `yaml:"directory"`
	AutoLoad  bool                   `yaml:"auto_load"`
	Config    map[string]interface{} `yaml:"config"`
}

// WebhooksConfig represents webhooks configuration
type WebhooksConfig struct {
	ConfigChange WebhookConfig `yaml:"config_change"`
	HealthStatus WebhookConfig `yaml:"health_status"`
}

// WebhookConfig represents individual webhook configuration
type WebhookConfig struct {
	Enabled    bool          `yaml:"enabled"`
	URL        string        `yaml:"url"`
	Timeout    time.Duration `yaml:"timeout"`
	RetryCount int           `yaml:"retry_count"`
}

// HeaderTransformConfig represents header transformation middleware configuration
type HeaderTransformConfig struct {
	Enabled         bool                           `yaml:"enabled"`
	RequestHeaders  HeaderTransformRules           `yaml:"request_headers"`
	ResponseHeaders HeaderTransformRules           `yaml:"response_headers"`
	PerRoute        map[string]HeaderTransformRule `yaml:"per_route"`
}

// HeaderTransformRules represents header transformation rules
type HeaderTransformRules struct {
	Add     map[string]string `yaml:"add"`
	Remove  []string          `yaml:"remove"`
	Rename  map[string]string `yaml:"rename"`
	Replace map[string]string `yaml:"replace"`
}

// HeaderTransformRule represents per-route header transformation configuration
type HeaderTransformRule struct {
	Enabled         bool                 `yaml:"enabled"`
	RequestHeaders  HeaderTransformRules `yaml:"request_headers"`
	ResponseHeaders HeaderTransformRules `yaml:"response_headers"`
}

// MockResponseConfig represents mock response middleware configuration
type MockResponseConfig struct {
	Enabled  bool                       `yaml:"enabled"`
	Rules    []MockRule                 `yaml:"rules"`
	PerRoute map[string]MockRouteConfig `yaml:"per_route"`
}

// MockRule represents a mock response rule
type MockRule struct {
	ID         string         `yaml:"id"`
	Name       string         `yaml:"name"`
	Conditions MockConditions `yaml:"conditions"`
	Response   MockResponse   `yaml:"response"`
	Priority   int            `yaml:"priority"`
	Enabled    bool           `yaml:"enabled"`
}

// MockConditions represents matching conditions for mock rules
type MockConditions struct {
	Methods     []string            `yaml:"methods"`
	Paths       []MockPathMatcher   `yaml:"paths"`
	Headers     map[string]string   `yaml:"headers"`
	QueryParams map[string]string   `yaml:"query_params"`
	Body        string              `yaml:"body"`
}

// MockPathMatcher represents path matching configuration
type MockPathMatcher struct {
	Type  string `yaml:"type"`  // exact, prefix, regex
	Value string `yaml:"value"`
}

// MockResponse represents mock response configuration
type MockResponse struct {
	StatusCode int               `yaml:"status_code"`
	Headers    map[string]string `yaml:"headers"`
	Body       string            `yaml:"body"`
	BodyFile   string            `yaml:"body_file"`
	Delay      time.Duration     `yaml:"delay"`
}

// MockRouteConfig represents per-route mock configuration
type MockRouteConfig struct {
	Enabled bool       `yaml:"enabled"`
	Rules   []MockRule `yaml:"rules"`
}

// GRPCWebConfig represents gRPC-Web proxy middleware configuration
type GRPCWebConfig struct {
	Enabled        bool                       `yaml:"enabled"`
	AllowedOrigins []string                   `yaml:"allowed_origins"`
	DefaultTimeout time.Duration              `yaml:"default_timeout"`
	Services       map[string]GRPCServiceConfig `yaml:"services"`
	CORS           GRPCWebCORSConfig          `yaml:"cors"`
}

// GRPCServiceConfig represents configuration for a specific gRPC service
type GRPCServiceConfig struct {
	Backend     string            `yaml:"backend"`
	Timeout     time.Duration     `yaml:"timeout"`
	Metadata    map[string]string `yaml:"metadata"`
	TLS         GRPCTLSConfig     `yaml:"tls"`
	Enabled     bool              `yaml:"enabled"`
}

// GRPCTLSConfig represents TLS configuration for gRPC backend
type GRPCTLSConfig struct {
	Enabled            bool   `yaml:"enabled"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	CAFile             string `yaml:"ca_file"`
}

// GRPCWebCORSConfig represents CORS configuration for gRPC-Web
type GRPCWebCORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// AggregatorConfig represents API aggregator configuration
type AggregatorConfig struct {
	Enabled        bool            `yaml:"enabled"`
	Routes         []AggregateRoute `yaml:"routes"`
	DefaultTimeout time.Duration   `yaml:"default_timeout"`
	MaxConcurrency int             `yaml:"max_concurrency"`
}

// AggregateRoute represents a single aggregate route configuration
type AggregateRoute struct {
	ID               string            `yaml:"id" json:"id"`
	Path             string            `yaml:"path" json:"path"`
	Method           string            `yaml:"method" json:"method"`
	UpstreamRequests []UpstreamRequest `yaml:"upstream_requests" json:"upstream_requests"`
	ResponseTemplate string            `yaml:"response_template,omitempty" json:"response_template,omitempty"`
	Timeout          time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// UpstreamRequest represents a single upstream request configuration
type UpstreamRequest struct {
	Name     string            `yaml:"name" json:"name"`
	URL      string            `yaml:"url" json:"url"`
	Method   string            `yaml:"method" json:"method"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Timeout  time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Required bool              `yaml:"required,omitempty" json:"required,omitempty"`
}

// ServerlessConfig represents serverless function integration configuration
type ServerlessConfig struct {
	Enabled        bool              `yaml:"enabled"`
	Rules          []ServerlessRule  `yaml:"rules"`
	DefaultTimeout time.Duration     `yaml:"default_timeout"`
}

// ServerlessRule represents a rule for when to execute serverless functions
type ServerlessRule struct {
	ID          string                   `yaml:"id" json:"id"`
	Path        string                   `yaml:"path" json:"path"`
	Method      string                   `yaml:"method" json:"method"`
	Headers     map[string]string        `yaml:"headers,omitempty" json:"headers,omitempty"`
	PreProcess  []ServerlessFunction     `yaml:"pre_process,omitempty" json:"pre_process,omitempty"`
	PostProcess []ServerlessFunction     `yaml:"post_process,omitempty" json:"post_process,omitempty"`
}

// ServerlessFunction represents a single serverless function configuration
type ServerlessFunction struct {
	ID         string            `yaml:"id" json:"id"`
	Name       string            `yaml:"name" json:"name"`
	URL        string            `yaml:"url" json:"url"`
	Method     string            `yaml:"method" json:"method"`
	Headers    map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Timeout    time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	RetryCount int               `yaml:"retry_count,omitempty" json:"retry_count,omitempty"`
	OnError    string            `yaml:"on_error,omitempty" json:"on_error,omitempty"` // continue, abort
}

// WASMConfig represents WASM plugin configuration
type WASMConfig struct {
	Enabled bool         `yaml:"enabled"`
	Plugins []WASMPlugin `yaml:"plugins"`
	Rules   []WASMRule   `yaml:"rules"`
}

// WASMPlugin represents a single WASM plugin configuration
type WASMPlugin struct {
	ID       string `yaml:"id" json:"id"`
	Name     string `yaml:"name" json:"name"`
	Path     string `yaml:"path" json:"path"`
	Required bool   `yaml:"required,omitempty" json:"required,omitempty"`
}

// WASMRule represents a rule for when to execute WASM plugins
type WASMRule struct {
	ID      string            `yaml:"id" json:"id"`
	Path    string            `yaml:"path" json:"path"`
	Method  string            `yaml:"method" json:"method"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Plugins []string          `yaml:"plugins" json:"plugins"`
}

// PortalConfig represents developer portal configuration
type PortalConfig struct {
	Enabled    bool                 `yaml:"enabled"`
	Port       int                  `yaml:"port"`
	JWT        PortalJWTConfig      `yaml:"jwt"`
	Repository PortalRepositoryConfig `yaml:"repository"`
	CORS       PortalCORSConfig     `yaml:"cors"`
}

// PortalJWTConfig represents JWT configuration for portal
type PortalJWTConfig struct {
	Secret    string        `yaml:"secret"`
	Algorithm string        `yaml:"algorithm"`
	ExpiresIn time.Duration `yaml:"expires_in"`
	Issuer    string        `yaml:"issuer"`
}

// PortalRepositoryConfig represents repository configuration for portal
type PortalRepositoryConfig struct {
	Type     string                    `yaml:"type"` // "memory" or "postgres"
	Memory   PortalMemoryConfig        `yaml:"memory"`
	Postgres PortalPostgresConfig      `yaml:"postgres"`
}

// PortalMemoryConfig represents in-memory repository configuration
type PortalMemoryConfig struct {
	// No specific configuration needed for memory repository
}

// PortalPostgresConfig represents PostgreSQL repository configuration
type PortalPostgresConfig struct {
	DSN             string        `yaml:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	MigrationPath   string        `yaml:"migration_path"`
}

// PortalCORSConfig represents CORS configuration for portal
type PortalCORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// GatewayConfig represents gateway integration configuration
type GatewayConfig struct {
	DataPlaneURL string `yaml:"data_plane_url"`
	AdminAPIKey  string `yaml:"admin_api_key"`
}
