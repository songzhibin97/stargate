package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from file with environment variable overrides
func Load(configFile string) (*Config, error) {
	// Set default configuration
	cfg := &Config{
		Server: ServerConfig{
			Address:        ":8080",
			Timeout:        30 * time.Second,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			IdleTimeout:    60 * time.Second,
			MaxHeaderBytes: 1048576,
		},
		Controller: ControllerConfig{
			Address:      ":9090",
			Timeout:      30 * time.Second,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		Portal: PortalConfig{
			Enabled: false,
			Port:    3000,
			JWT: PortalJWTConfig{
				Secret:    "",
				Algorithm: "HS256",
				ExpiresIn: 24 * time.Hour,
				Issuer:    "stargate-portal",
			},
			Repository: PortalRepositoryConfig{
				Type: "memory",
				Memory: PortalMemoryConfig{},
				Postgres: PortalPostgresConfig{
					DSN:             "postgres://postgres:password@localhost:5432/stargate?sslmode=disable",
					MaxOpenConns:    25,
					MaxIdleConns:    5,
					ConnMaxLifetime: 5 * time.Minute,
					MigrationPath:   "file://internal/portal/repository/postgres/migrations",
				},
			},
			CORS: PortalCORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "Authorization"},
				ExposedHeaders:   []string{},
				AllowCredentials: true,
				MaxAge:           86400,
			},
		},
		Gateway: GatewayConfig{
			DataPlaneURL: "http://localhost:8080",
			AdminAPIKey:  "",
		},
		Proxy: ProxyConfig{
			BufferSize:              32768,
			PoolSize:                100,
			ConnectTimeout:          5 * time.Second,
			ResponseHeaderTimeout:   10 * time.Second,
			KeepAliveTimeout:        30 * time.Second,
			MaxIdleConns:            100,
			MaxIdleConnsPerHost:     10,
			WebSocket: WebSocketConfig{
				Enabled:          true,
				BufferSize:       32768,
				ReadTimeout:      60 * time.Second,
				WriteTimeout:     60 * time.Second,
				PingInterval:     30 * time.Second,
				PongTimeout:      10 * time.Second,
				MaxConnections:   1000,
				CompressionLevel: 1,
			},
		},
		LoadBalancer: LoadBalancerConfig{
			DefaultAlgorithm: "round_robin",
			HealthCheck: HealthCheckConfig{
				Enabled:            true,
				Interval:           30 * time.Second,
				Timeout:            5 * time.Second,
				HealthyThreshold:   2,
				UnhealthyThreshold: 3,
				Path:               "/health",
			},
		},
		RateLimit: RateLimitConfig{
			Enabled:            false,
			DefaultRate:        1000,
			Burst:              100,
			Storage:            "memory",
			Strategy:           "fixed_window",
			IdentifierStrategy: "ip",
			WindowSize:         time.Minute,
			CleanupInterval:    5 * time.Minute,
			SkipSuccessful:     false,
			SkipFailed:         false,
			CustomHeaders:      make(map[string]string),
			ExcludedPaths:      []string{"/health", "/metrics"},
			ExcludedIPs:        []string{"127.0.0.1", "::1"},
			PerRoute:           make(map[string]RouteRateLimit),
		},
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:                  false,
			FailureThreshold:         5,
			RecoveryTimeout:          60 * time.Second,
			RequestVolumeThreshold:   20,
			ErrorPercentageThreshold: 50,
		},
		Auth: AuthConfig{
			Enabled: false,
			JWT: JWTConfig{
				Algorithm: "HS256",
				ExpiresIn: 24 * time.Hour,
			},
			APIKey: APIKeyConfig{
				Header: "X-API-Key",
				Query:  "api_key",
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
			AccessLog: AccessLogConfig{
				Enabled: true,
				Format:  "combined",
				Output:  "stdout",
			},
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Path:    "/metrics",
			Prometheus: PrometheusConfig{
				Enabled:   true,
				Namespace: "stargate",
				Subsystem: "node",
			},
			// New unified metrics configuration with defaults
			Provider:  "prometheus",
			Namespace: "stargate",
			Subsystem: "node",
			EnabledMetrics: map[string]bool{
				"requests_total":     true,
				"request_duration":   true,
				"request_size":       true,
				"response_size":      true,
				"active_connections": true,
				"errors_total":       true,
			},
			SampleRate:     1.0,
			MaxLabelLength: 256,
			AsyncUpdates:   false,
			BufferSize:     1000,
		},
		Tracing: TracingConfig{
			Enabled: false,
			Jaeger: JaegerConfig{
				Endpoint:    "http://localhost:14268/api/traces",
				ServiceName: "stargate-node",
				SampleRate:  0.1,
			},
		},
		Store: StoreConfig{
			Type:      "etcd",
			KeyPrefix: "/stargate/config",
			Watch:     true,
			Etcd: EtcdConfig{
				Endpoints: []string{"localhost:2379"},
				Timeout:   5 * time.Second,
			},
		},
		ConfigSource: ConfigSourceConfig{
			Source: SourceConfig{
				Driver:       "file",
				PollInterval: 1 * time.Second,
				File: FileSourceConfig{
					Path:         "routes.yaml",
					PollInterval: 1 * time.Second,
				},
				Etcd: EtcdSourceConfig{
					Endpoints: []string{"localhost:2379"},
					Key:       "/stargate/routes",
					Timeout:   5 * time.Second,
				},
			},
		},
		Sync: SyncConfig{
			Interval: 30 * time.Second,
			GitOps: GitOpsConfig{
				Enabled:      false,
				Branch:       "main",
				Path:         "configs",
				PollInterval: 5 * time.Minute,
			},
			Validation: ValidationConfig{
				Enabled: true,
				Strict:  true,
			},
		},
		AdminAPI: AdminAPIConfig{
			REST: RESTConfig{
				Enabled: true,
				Prefix:  "/api/v1",
			},
			GRPC: GRPCConfig{
				Enabled: true,
				Port:    9091,
			},
		},
		Routes: RoutesConfig{
			Defaults: RouteDefaults{
				Timeout:      30 * time.Second,
				Retries:      3,
				RetryTimeout: 5 * time.Second,
			},
		},
		Upstreams: UpstreamsConfig{
			Defaults: UpstreamDefaults{
				Algorithm: "round_robin",
				HealthCheck: HealthCheckConfig{
					Enabled:            true,
					Interval:           30 * time.Second,
					Timeout:            5 * time.Second,
					HealthyThreshold:   2,
					UnhealthyThreshold: 3,
					Path:               "/health",
					Passive: PassiveHealthCheckConfig{
						Enabled:              true,
						ConsecutiveFailures:  3,
						IsolationDuration:    30 * time.Second,
						RecoveryInterval:     10 * time.Second,
						ConsecutiveSuccesses: 2,
						FailureStatusCodes:   []int{500, 501, 502, 503, 504, 505},
						TimeoutAsFailure:     true,
					},
				},
			},
		},
		HeaderTransform: HeaderTransformConfig{
			Enabled: false,
			RequestHeaders: HeaderTransformRules{
				Add:     make(map[string]string),
				Remove:  []string{},
				Rename:  make(map[string]string),
				Replace: make(map[string]string),
			},
			ResponseHeaders: HeaderTransformRules{
				Add:     make(map[string]string),
				Remove:  []string{},
				Rename:  make(map[string]string),
				Replace: make(map[string]string),
			},
			PerRoute: make(map[string]HeaderTransformRule),
		},
		MockResponse: MockResponseConfig{
			Enabled:  false,
			Rules:    []MockRule{},
			PerRoute: make(map[string]MockRouteConfig),
		},
		GRPCWeb: GRPCWebConfig{
			Enabled:        false,
			AllowedOrigins: []string{"*"},
			DefaultTimeout: 30 * time.Second,
			Services:       make(map[string]GRPCServiceConfig),
			CORS: GRPCWebCORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"POST", "OPTIONS"},
				AllowedHeaders:   []string{"Content-Type", "X-Grpc-Web", "X-User-Agent"},
				ExposedHeaders:   []string{"Grpc-Status", "Grpc-Message"},
				AllowCredentials: false,
				MaxAge:           86400,
			},
		},
		Plugins: PluginsConfig{
			Directory: "./plugins",
			AutoLoad:  true,
			Config:    make(map[string]interface{}),
		},
	}

	// Load from file if exists
	if configFile != "" {
		if err := loadFromFile(cfg, configFile); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
	}

	// Override with environment variables
	if err := loadFromEnv(cfg); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from YAML file
func loadFromFile(cfg *Config, filename string) error {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("config file does not exist: %s", filename)
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML config: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(cfg *Config) error {
	// Server configuration
	if addr := os.Getenv("STARGATE_SERVER_ADDRESS"); addr != "" {
		cfg.Server.Address = addr
	}
	if httpsAddr := os.Getenv("STARGATE_SERVER_HTTPS_ADDRESS"); httpsAddr != "" {
		cfg.Server.HTTPSAddress = httpsAddr
	}

	// Controller configuration
	if addr := os.Getenv("STARGATE_CONTROLLER_ADDRESS"); addr != "" {
		cfg.Controller.Address = addr
	}

	// Store configuration
	if storeType := os.Getenv("STARGATE_STORE_TYPE"); storeType != "" {
		cfg.Store.Type = storeType
	}
	if endpoints := os.Getenv("STARGATE_ETCD_ENDPOINTS"); endpoints != "" {
		cfg.Store.Etcd.Endpoints = strings.Split(endpoints, ",")
	}
	if username := os.Getenv("STARGATE_ETCD_USERNAME"); username != "" {
		cfg.Store.Etcd.Username = username
	}
	if password := os.Getenv("STARGATE_ETCD_PASSWORD"); password != "" {
		cfg.Store.Etcd.Password = password
	}

	// Auth configuration
	if jwtSecret := os.Getenv("STARGATE_JWT_SECRET"); jwtSecret != "" {
		cfg.Auth.JWT.Secret = jwtSecret
	}

	// Logging configuration
	if logLevel := os.Getenv("STARGATE_LOG_LEVEL"); logLevel != "" {
		cfg.Logging.Level = logLevel
	}
	if logFormat := os.Getenv("STARGATE_LOG_FORMAT"); logFormat != "" {
		cfg.Logging.Format = logFormat
	}

	return nil
}

// validate validates the configuration
func validate(cfg *Config) error {
	// Validate server address
	if cfg.Server.Address == "" {
		return fmt.Errorf("server address cannot be empty")
	}

	// Validate store configuration
	if cfg.Store.Type == "" {
		return fmt.Errorf("store type cannot be empty")
	}

	if cfg.Store.Type == "etcd" {
		if len(cfg.Store.Etcd.Endpoints) == 0 {
			return fmt.Errorf("etcd endpoints cannot be empty")
		}
	}

	// Validate JWT secret if auth is enabled
	if cfg.Auth.Enabled && cfg.Auth.JWT.Secret == "" {
		return fmt.Errorf("JWT secret cannot be empty when auth is enabled")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
	}
	if !validLogLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", cfg.Logging.Level)
	}

	// Validate load balancer algorithm
	validAlgorithms := map[string]bool{
		"round_robin": true,
		"weighted":    true,
		"ip_hash":     true,
	}
	if !validAlgorithms[cfg.LoadBalancer.DefaultAlgorithm] {
		return fmt.Errorf("invalid load balancer algorithm: %s", cfg.LoadBalancer.DefaultAlgorithm)
	}

	return nil
}

// GetConfigDir returns the configuration directory
func GetConfigDir() string {
	if dir := os.Getenv("STARGATE_CONFIG_DIR"); dir != "" {
		return dir
	}
	
	// Default to current directory
	return "."
}

// GetDefaultConfigFile returns the default configuration file path
func GetDefaultConfigFile(component string) string {
	configDir := GetConfigDir()
	return filepath.Join(configDir, fmt.Sprintf("stargate-%s.defaults.yaml", component))
}
