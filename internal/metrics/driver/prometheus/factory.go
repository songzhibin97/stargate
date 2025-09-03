package prometheus

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/songzhibin97/stargate/pkg/metrics"
)

// Factory implements metrics.Factory for Prometheus
type Factory struct{}

// Create creates a new PrometheusProvider with the given options
func (f *Factory) Create(opts metrics.ProviderOptions) (metrics.Provider, error) {
	promOpts := Options{
		Namespace:   opts.Namespace,
		Subsystem:   opts.Subsystem,
		ConstLabels: opts.ConstLabels,
	}
	
	// Use provided registry if available
	if opts.Registerer != nil {
		// Try to extract prometheus.Registry from our registry wrapper
		// In practice, this would need a more sophisticated approach
		// For now, we'll create a new registry
		promOpts.Registry = prometheus.NewRegistry()
	}
	
	// Use provided gatherer if available
	if opts.Gatherer != nil {
		// We need to wrap our gatherer to make it compatible with prometheus.Gatherer
		promOpts.Gatherer = &gathererAdapter{gatherer: opts.Gatherer}
	}
	
	return NewProvider(promOpts)
}

// Name returns the name of the factory
func (f *Factory) Name() string {
	return "prometheus"
}

// Description returns a description of the factory
func (f *Factory) Description() string {
	return "Prometheus metrics provider using github.com/prometheus/client_golang"
}

// NewFactory creates a new Prometheus factory
func NewFactory() metrics.Factory {
	return &Factory{}
}

// DefaultProvider creates a default Prometheus provider
func DefaultProvider() (metrics.Provider, error) {
	return NewProvider(Options{})
}

// DefaultProviderWithOptions creates a default Prometheus provider with options
func DefaultProviderWithOptions(namespace, subsystem string, constLabels map[string]string) (metrics.Provider, error) {
	return NewProvider(Options{
		Namespace:   namespace,
		Subsystem:   subsystem,
		ConstLabels: constLabels,
	})
}

// MustDefaultProvider creates a default Prometheus provider and panics on error
func MustDefaultProvider() metrics.Provider {
	provider, err := DefaultProvider()
	if err != nil {
		panic(err)
	}
	return provider
}

// MustDefaultProviderWithOptions creates a default Prometheus provider with options and panics on error
func MustDefaultProviderWithOptions(namespace, subsystem string, constLabels map[string]string) metrics.Provider {
	provider, err := DefaultProviderWithOptions(namespace, subsystem, constLabels)
	if err != nil {
		panic(err)
	}
	return provider
}

// RegisterFactory registers the Prometheus factory with the global registry
func RegisterFactory() error {
	return metrics.RegisterFactory("prometheus", NewFactory())
}

// init automatically registers the Prometheus factory
func init() {
	if err := RegisterFactory(); err != nil {
		// Log error but don't panic during init
		// In a real implementation, you might want to use a logger here
	}
}

// NewProviderFromConfig creates a provider from configuration
func NewProviderFromConfig(config PrometheusConfig) (metrics.Provider, error) {
	opts := Options{
		Namespace:   config.Namespace,
		Subsystem:   config.Subsystem,
		ConstLabels: config.ConstLabels,
	}
	
	if config.Registry != nil {
		opts.Registry = config.Registry
	}
	
	if config.Gatherer != nil {
		opts.Gatherer = config.Gatherer
	}
	
	return NewProvider(opts)
}

// PrometheusConfig represents configuration for Prometheus provider
type PrometheusConfig struct {
	Namespace   string                 `yaml:"namespace" json:"namespace"`
	Subsystem   string                 `yaml:"subsystem" json:"subsystem"`
	ConstLabels map[string]string      `yaml:"const_labels" json:"const_labels"`
	Registry    *prometheus.Registry   `yaml:"-" json:"-"`
	Gatherer    prometheus.Gatherer    `yaml:"-" json:"-"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() PrometheusConfig {
	return PrometheusConfig{
		Namespace:   "stargate",
		Subsystem:   "metrics",
		ConstLabels: make(map[string]string),
	}
}

// Validate validates the configuration
func (c *PrometheusConfig) Validate() error {
	if c.Namespace != "" {
		if err := metrics.ValidateMetricName(c.Namespace); err != nil {
			return err
		}
	}
	
	if c.Subsystem != "" {
		if err := metrics.ValidateMetricName(c.Subsystem); err != nil {
			return err
		}
	}
	
	for name, value := range c.ConstLabels {
		if err := metrics.ValidateLabelName(name); err != nil {
			return err
		}
		if value == "" {
			return metrics.NewMetricError("validate", name, nil, metrics.ErrInvalidLabel)
		}
	}
	
	return nil
}

// WithNamespace sets the namespace
func (c *PrometheusConfig) WithNamespace(namespace string) *PrometheusConfig {
	c.Namespace = namespace
	return c
}

// WithSubsystem sets the subsystem
func (c *PrometheusConfig) WithSubsystem(subsystem string) *PrometheusConfig {
	c.Subsystem = subsystem
	return c
}

// WithConstLabels sets the constant labels
func (c *PrometheusConfig) WithConstLabels(labels map[string]string) *PrometheusConfig {
	c.ConstLabels = labels
	return c
}

// WithRegistry sets the registry
func (c *PrometheusConfig) WithRegistry(registry *prometheus.Registry) *PrometheusConfig {
	c.Registry = registry
	return c
}

// WithGatherer sets the gatherer
func (c *PrometheusConfig) WithGatherer(gatherer prometheus.Gatherer) *PrometheusConfig {
	c.Gatherer = gatherer
	return c
}

// Builder provides a fluent API for building Prometheus providers
type Builder struct {
	config PrometheusConfig
}

// NewBuilder creates a new builder
func NewBuilder() *Builder {
	return &Builder{
		config: DefaultConfig(),
	}
}

// WithNamespace sets the namespace
func (b *Builder) WithNamespace(namespace string) *Builder {
	b.config.Namespace = namespace
	return b
}

// WithSubsystem sets the subsystem
func (b *Builder) WithSubsystem(subsystem string) *Builder {
	b.config.Subsystem = subsystem
	return b
}

// WithConstLabels sets the constant labels
func (b *Builder) WithConstLabels(labels map[string]string) *Builder {
	b.config.ConstLabels = labels
	return b
}

// WithRegistry sets the registry
func (b *Builder) WithRegistry(registry *prometheus.Registry) *Builder {
	b.config.Registry = registry
	return b
}

// WithGatherer sets the gatherer
func (b *Builder) WithGatherer(gatherer prometheus.Gatherer) *Builder {
	b.config.Gatherer = gatherer
	return b
}

// Build creates the provider
func (b *Builder) Build() (metrics.Provider, error) {
	if err := b.config.Validate(); err != nil {
		return nil, err
	}
	return NewProviderFromConfig(b.config)
}

// MustBuild creates the provider and panics on error
func (b *Builder) MustBuild() metrics.Provider {
	provider, err := b.Build()
	if err != nil {
		panic(err)
	}
	return provider
}
