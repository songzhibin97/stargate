package middleware

import (
	"fmt"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/metrics/driver/prometheus"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// NewMetricsMiddlewareFromPrometheusConfig creates a MetricsMiddleware from PrometheusConfig
// This function provides backward compatibility for existing configurations
func NewMetricsMiddlewareFromPrometheusConfig(cfg *config.PrometheusConfig) (*MetricsMiddleware, error) {
	if cfg == nil {
		cfg = &config.PrometheusConfig{
			Enabled:   true,
			Namespace: "stargate",
			Subsystem: "node",
		}
	}

	// Create Prometheus provider
	provider, err := prometheus.NewProvider(prometheus.Options{
		Namespace: cfg.Namespace,
		Subsystem: cfg.Subsystem,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus provider: %w", err)
	}

	// Convert PrometheusConfig to MetricsConfig
	metricsConfig := &MetricsConfig{
		Enabled:   cfg.Enabled,
		Provider:  "prometheus",
		Namespace: cfg.Namespace,
		Subsystem: cfg.Subsystem,
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
	}

	return NewMetricsMiddleware(metricsConfig, provider)
}

// NewMetricsMiddlewareFromConfig creates a MetricsMiddleware from the new MetricsConfig
func NewMetricsMiddlewareFromConfig(cfg *MetricsConfig) (*MetricsMiddleware, error) {
	if cfg == nil {
		cfg = DefaultMetricsConfig()
	}

	// Create provider based on configuration
	var provider metrics.Provider
	var err error

	switch cfg.Provider {
	case "prometheus", "":
		provider, err = prometheus.NewProvider(prometheus.Options{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: cfg.ConstLabels,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Prometheus provider: %w", err)
		}
	default:
		// Try to get provider from factory
		factory, err := metrics.GetFactory(cfg.Provider)
		if err != nil {
			return nil, fmt.Errorf("unknown metrics provider %s: %w", cfg.Provider, err)
		}

		provider, err = factory.Create(metrics.ProviderOptions{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: cfg.ConstLabels,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create %s provider: %w", cfg.Provider, err)
		}
	}

	return NewMetricsMiddleware(cfg, provider)
}

// PrometheusMiddlewareAdapter provides backward compatibility
// This allows existing code to continue using PrometheusMiddleware interface
// while internally using the new MetricsMiddleware
type PrometheusMiddlewareAdapter struct {
	*MetricsMiddleware
	config *config.PrometheusConfig
}

// NewPrometheusMiddlewareAdapter creates a new adapter for backward compatibility
func NewPrometheusMiddlewareAdapter(cfg *config.PrometheusConfig) (*PrometheusMiddlewareAdapter, error) {
	metricsMiddleware, err := NewMetricsMiddlewareFromPrometheusConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &PrometheusMiddlewareAdapter{
		MetricsMiddleware: metricsMiddleware,
		config:           cfg,
	}, nil
}

// GetMetrics returns metrics in the format expected by the old PrometheusMiddleware
func (p *PrometheusMiddlewareAdapter) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"enabled":   p.config.Enabled,
		"namespace": p.config.Namespace,
		"subsystem": p.config.Subsystem,
	}
}

// MetricsProviderFactory creates metrics providers based on configuration
type MetricsProviderFactory struct{}

// CreateProvider creates a metrics provider based on the provider type
func (f *MetricsProviderFactory) CreateProvider(providerType string, opts metrics.ProviderOptions) (metrics.Provider, error) {
	switch providerType {
	case "prometheus", "":
		return prometheus.NewProvider(prometheus.Options{
			Namespace:   opts.Namespace,
			Subsystem:   opts.Subsystem,
			ConstLabels: opts.ConstLabels,
		})
	default:
		// Try to get provider from factory registry
		factory, err := metrics.GetFactory(providerType)
		if err != nil {
			return nil, fmt.Errorf("unknown metrics provider %s: %w", providerType, err)
		}
		return factory.Create(opts)
	}
}

// GetSupportedProviders returns a list of supported provider types
func (f *MetricsProviderFactory) GetSupportedProviders() []string {
	providers := []string{"prometheus"}
	
	// Add registered providers from factory
	registered := metrics.ListFactories()
	providers = append(providers, registered...)
	
	return providers
}

// ValidateConfig validates a MetricsConfig
func ValidateMetricsConfig(cfg *MetricsConfig) error {
	if cfg == nil {
		return fmt.Errorf("metrics config cannot be nil")
	}

	if cfg.Namespace != "" {
		if err := metrics.ValidateMetricName(cfg.Namespace); err != nil {
			return fmt.Errorf("invalid namespace: %w", err)
		}
	}

	if cfg.Subsystem != "" {
		if err := metrics.ValidateMetricName(cfg.Subsystem); err != nil {
			return fmt.Errorf("invalid subsystem: %w", err)
		}
	}

	if cfg.SampleRate < 0 || cfg.SampleRate > 1 {
		return fmt.Errorf("sample rate must be between 0 and 1, got %f", cfg.SampleRate)
	}

	if cfg.MaxLabelLength < 0 {
		return fmt.Errorf("max label length cannot be negative, got %d", cfg.MaxLabelLength)
	}

	if cfg.AsyncUpdates && cfg.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be positive when async updates are enabled, got %d", cfg.BufferSize)
	}

	// Validate const labels
	for name, value := range cfg.ConstLabels {
		if err := metrics.ValidateLabelName(name); err != nil {
			return fmt.Errorf("invalid const label name %s: %w", name, err)
		}
		if value == "" {
			return fmt.Errorf("const label %s cannot have empty value", name)
		}
	}

	return nil
}

// MigratePrometheusConfig converts old PrometheusConfig to new MetricsConfig
func MigratePrometheusConfig(oldConfig *config.PrometheusConfig) *MetricsConfig {
	if oldConfig == nil {
		return DefaultMetricsConfig()
	}

	return &MetricsConfig{
		Enabled:   oldConfig.Enabled,
		Provider:  "prometheus",
		Namespace: oldConfig.Namespace,
		Subsystem: oldConfig.Subsystem,
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
	}
}

// DefaultPrometheusConfig returns a default PrometheusConfig for backward compatibility
func DefaultPrometheusConfig() *config.PrometheusConfig {
	return &config.PrometheusConfig{
		Enabled:   true,
		Namespace: "stargate",
		Subsystem: "node",
	}
}
