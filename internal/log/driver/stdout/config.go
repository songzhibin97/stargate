package stdout

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/songzhibin97/stargate/pkg/log"
)

// ConfigBuilder provides a fluent interface for building StdoutLogger configurations.
type ConfigBuilder struct {
	config *Config
}

// NewConfigBuilder creates a new ConfigBuilder with default settings.
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: DefaultConfig(),
	}
}

// WithLevel sets the minimum logging level.
func (b *ConfigBuilder) WithLevel(level log.Level) *ConfigBuilder {
	b.config.Level = level
	return b
}

// WithTimeFormat sets the time format for timestamps.
func (b *ConfigBuilder) WithTimeFormat(format string) *ConfigBuilder {
	b.config.TimeFormat = format
	return b
}

// WithCaller enables or disables caller information in log entries.
func (b *ConfigBuilder) WithCaller(enabled bool) *ConfigBuilder {
	b.config.EnableCaller = enabled
	return b
}

// WithStacktrace enables or disables stack traces for error and fatal levels.
func (b *ConfigBuilder) WithStacktrace(enabled bool) *ConfigBuilder {
	b.config.EnableStacktrace = enabled
	return b
}

// WithColors enables or disables colored output.
func (b *ConfigBuilder) WithColors(enabled bool) *ConfigBuilder {
	b.config.DisableColors = !enabled
	return b
}

// WithDevelopment enables or disables development mode.
func (b *ConfigBuilder) WithDevelopment(enabled bool) *ConfigBuilder {
	b.config.Development = enabled
	return b
}

// WithFieldNames sets custom field names for JSON output.
func (b *ConfigBuilder) WithFieldNames(fieldNames FieldNames) *ConfigBuilder {
	b.config.FieldNames = fieldNames
	return b
}

// WithTimeFieldName sets the custom field name for timestamps.
func (b *ConfigBuilder) WithTimeFieldName(name string) *ConfigBuilder {
	b.config.FieldNames.Time = name
	return b
}

// WithLevelFieldName sets the custom field name for log levels.
func (b *ConfigBuilder) WithLevelFieldName(name string) *ConfigBuilder {
	b.config.FieldNames.Level = name
	return b
}

// WithMessageFieldName sets the custom field name for messages.
func (b *ConfigBuilder) WithMessageFieldName(name string) *ConfigBuilder {
	b.config.FieldNames.Message = name
	return b
}

// WithCallerFieldName sets the custom field name for caller information.
func (b *ConfigBuilder) WithCallerFieldName(name string) *ConfigBuilder {
	b.config.FieldNames.Caller = name
	return b
}

// Build returns the configured Config.
func (b *ConfigBuilder) Build() *Config {
	// Validate configuration
	if err := b.config.Validate(); err != nil {
		// Return default config if validation fails
		return DefaultConfig()
	}
	return b.config
}

// Validate validates the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	// Validate log level
	if c.Level < log.DebugLevel || c.Level > log.FatalLevel {
		return fmt.Errorf("invalid log level: %d", c.Level)
	}

	// Validate time format
	if c.TimeFormat == "" {
		c.TimeFormat = time.RFC3339
	}

	// Test time format
	_, err := time.Parse(c.TimeFormat, time.Now().Format(c.TimeFormat))
	if err != nil {
		return fmt.Errorf("invalid time format: %s", c.TimeFormat)
	}

	// Validate field names
	if c.FieldNames.Time == "" {
		c.FieldNames.Time = "timestamp"
	}
	if c.FieldNames.Level == "" {
		c.FieldNames.Level = "level"
	}
	if c.FieldNames.Message == "" {
		c.FieldNames.Message = "message"
	}
	if c.FieldNames.Caller == "" {
		c.FieldNames.Caller = "caller"
	}

	return nil
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	return &Config{
		Level:            c.Level,
		TimeFormat:       c.TimeFormat,
		EnableCaller:     c.EnableCaller,
		EnableStacktrace: c.EnableStacktrace,
		DisableColors:    c.DisableColors,
		Development:      c.Development,
		FieldNames: FieldNames{
			Time:    c.FieldNames.Time,
			Level:   c.FieldNames.Level,
			Message: c.FieldNames.Message,
			Caller:  c.FieldNames.Caller,
		},
	}
}

// ToJSON converts the configuration to JSON format.
func (c *Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// FromJSON loads configuration from JSON data.
func (c *Config) FromJSON(data []byte) error {
	if err := json.Unmarshal(data, c); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return c.Validate()
}

// String returns a string representation of the configuration.
func (c *Config) String() string {
	data, err := c.ToJSON()
	if err != nil {
		return fmt.Sprintf("Config{Level: %s, TimeFormat: %s}", c.Level.String(), c.TimeFormat)
	}
	return string(data)
}

// PresetConfigs provides common configuration presets.
var PresetConfigs = struct {
	Development *Config
	Production  *Config
	Debug       *Config
}{
	Development: &Config{
		Level:            log.DebugLevel,
		TimeFormat:       time.RFC3339,
		EnableCaller:     true,
		EnableStacktrace: true,
		DisableColors:    false,
		Development:      true,
		FieldNames: FieldNames{
			Time:    "time",
			Level:   "level",
			Message: "msg",
			Caller:  "caller",
		},
	},
	Production: &Config{
		Level:            log.InfoLevel,
		TimeFormat:       time.RFC3339,
		EnableCaller:     false,
		EnableStacktrace: true,
		DisableColors:    true,
		Development:      false,
		FieldNames: FieldNames{
			Time:    "timestamp",
			Level:   "level",
			Message: "message",
			Caller:  "caller",
		},
	},
	Debug: &Config{
		Level:            log.DebugLevel,
		TimeFormat:       time.RFC3339Nano,
		EnableCaller:     true,
		EnableStacktrace: true,
		DisableColors:    false,
		Development:      true,
		FieldNames: FieldNames{
			Time:    "timestamp",
			Level:   "level",
			Message: "message",
			Caller:  "caller",
		},
	},
}

// GetPresetConfig returns a preset configuration by name.
func GetPresetConfig(name string) *Config {
	switch name {
	case "development", "dev":
		return PresetConfigs.Development.Clone()
	case "production", "prod":
		return PresetConfigs.Production.Clone()
	case "debug":
		return PresetConfigs.Debug.Clone()
	default:
		return DefaultConfig()
	}
}

// ConfigOption represents a functional option for configuring StdoutLogger.
type ConfigOption func(*Config)

// WithLevelOption returns a ConfigOption that sets the log level.
func WithLevelOption(level log.Level) ConfigOption {
	return func(c *Config) {
		c.Level = level
	}
}

// WithTimeFormatOption returns a ConfigOption that sets the time format.
func WithTimeFormatOption(format string) ConfigOption {
	return func(c *Config) {
		c.TimeFormat = format
	}
}

// WithCallerOption returns a ConfigOption that enables/disables caller info.
func WithCallerOption(enabled bool) ConfigOption {
	return func(c *Config) {
		c.EnableCaller = enabled
	}
}

// WithStacktraceOption returns a ConfigOption that enables/disables stack traces.
func WithStacktraceOption(enabled bool) ConfigOption {
	return func(c *Config) {
		c.EnableStacktrace = enabled
	}
}

// WithColorsOption returns a ConfigOption that enables/disables colors.
func WithColorsOption(enabled bool) ConfigOption {
	return func(c *Config) {
		c.DisableColors = !enabled
	}
}

// WithDevelopmentOption returns a ConfigOption that enables/disables development mode.
func WithDevelopmentOption(enabled bool) ConfigOption {
	return func(c *Config) {
		c.Development = enabled
	}
}

// WithFieldNamesOption returns a ConfigOption that sets custom field names.
func WithFieldNamesOption(fieldNames FieldNames) ConfigOption {
	return func(c *Config) {
		c.FieldNames = fieldNames
	}
}

// NewWithOptions creates a new StdoutLogger with functional options.
func NewWithOptions(options ...ConfigOption) (*StdoutLogger, error) {
	config := DefaultConfig()
	for _, option := range options {
		option(config)
	}
	return New(config)
}
