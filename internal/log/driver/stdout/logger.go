package stdout

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/auth"
	"github.com/songzhibin97/stargate/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// StdoutLogger implements the log.Logger interface using zap for JSON output to stdout.
// It provides structured logging with high performance and configurable options.
type StdoutLogger struct {
	zapLogger *zap.Logger
	config    *Config
	fields    []log.Field
	mu        sync.RWMutex
}

// Config represents the configuration options for StdoutLogger.
type Config struct {
	// Level sets the minimum logging level
	Level log.Level `json:"level"`
	
	// TimeFormat specifies the time format for timestamps
	// Default: RFC3339
	TimeFormat string `json:"time_format,omitempty"`
	
	// EnableCaller adds caller information to log entries
	EnableCaller bool `json:"enable_caller"`
	
	// EnableStacktrace adds stack trace for error and fatal levels
	EnableStacktrace bool `json:"enable_stacktrace"`
	
	// DisableColors disables colored output (useful for production)
	DisableColors bool `json:"disable_colors"`
	
	// FieldNames allows customization of field names in JSON output
	FieldNames FieldNames `json:"field_names,omitempty"`
	
	// Development enables development mode with more human-readable output
	Development bool `json:"development"`
}

// FieldNames allows customization of standard field names in JSON output.
type FieldNames struct {
	Time    string `json:"time,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
	Caller  string `json:"caller,omitempty"`
}

// DefaultConfig returns a default configuration for StdoutLogger.
func DefaultConfig() *Config {
	return &Config{
		Level:            log.InfoLevel,
		TimeFormat:       time.RFC3339,
		EnableCaller:     false,
		EnableStacktrace: true,
		DisableColors:    false,
		Development:      false,
		FieldNames: FieldNames{
			Time:    "timestamp",
			Level:   "level",
			Message: "message",
			Caller:  "caller",
		},
	}
}

// New creates a new StdoutLogger with the given configuration.
func New(config *Config) (*StdoutLogger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Configure zap encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        config.FieldNames.Time,
		LevelKey:       config.FieldNames.Level,
		NameKey:        "logger",
		CallerKey:      config.FieldNames.Caller,
		MessageKey:     config.FieldNames.Message,
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     getTimeEncoder(config.TimeFormat),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create encoder (always JSON for stdout)
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Configure log level
	zapLevel := convertLogLevel(config.Level)
	
	// Create core
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		zapLevel,
	)

	// Configure logger options
	var options []zap.Option
	if config.EnableCaller {
		options = append(options, zap.AddCaller())
	}
	if config.EnableStacktrace {
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel))
	}
	if config.Development {
		options = append(options, zap.Development())
	}

	zapLogger := zap.New(core, options...)

	return &StdoutLogger{
		zapLogger: zapLogger,
		config:    config,
		fields:    make([]log.Field, 0),
	}, nil
}

// Debug logs a debug message with optional structured fields.
func (l *StdoutLogger) Debug(msg string, fields ...log.Field) {
	l.log(log.DebugLevel, msg, fields...)
}

// Info logs an informational message with optional structured fields.
func (l *StdoutLogger) Info(msg string, fields ...log.Field) {
	l.log(log.InfoLevel, msg, fields...)
}

// Warn logs a warning message with optional structured fields.
func (l *StdoutLogger) Warn(msg string, fields ...log.Field) {
	l.log(log.WarnLevel, msg, fields...)
}

// Error logs an error message with optional structured fields.
func (l *StdoutLogger) Error(msg string, fields ...log.Field) {
	l.log(log.ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message with optional structured fields and exits the program.
func (l *StdoutLogger) Fatal(msg string, fields ...log.Field) {
	l.log(log.FatalLevel, msg, fields...)
	os.Exit(1)
}

// With creates a new logger instance with additional structured fields.
func (l *StdoutLogger) With(fields ...log.Field) log.Logger {
	l.mu.RLock()
	existingFields := make([]log.Field, len(l.fields))
	copy(existingFields, l.fields)
	l.mu.RUnlock()

	newFields := append(existingFields, fields...)
	
	return &StdoutLogger{
		zapLogger: l.zapLogger,
		config:    l.config,
		fields:    newFields,
	}
}

// WithContext creates a new logger instance with context information.
func (l *StdoutLogger) WithContext(ctx context.Context) log.Logger {
	// Extract common context values
	var contextFields []log.Field

	// Extract consumer_id from authentication context
	if consumer, ok := auth.GetConsumerFromContext(ctx); ok && consumer != nil {
		contextFields = append(contextFields, log.String("consumer_id", consumer.ID))
	}

	// Extract trace ID if available
	if traceID := extractTraceID(ctx); traceID != "" {
		contextFields = append(contextFields, log.String("trace_id", traceID))
	}

	// Extract request ID if available
	if requestID := extractRequestID(ctx); requestID != "" {
		contextFields = append(contextFields, log.String("request_id", requestID))
	}

	if len(contextFields) == 0 {
		return l
	}

	return l.With(contextFields...)
}

// log is the internal logging method that handles the actual logging.
func (l *StdoutLogger) log(level log.Level, msg string, fields ...log.Field) {
	// Check if logging is enabled for this level
	if level < l.config.Level {
		return
	}

	// Combine existing fields with new fields
	l.mu.RLock()
	allFields := make([]log.Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)
	l.mu.RUnlock()

	// Convert to zap fields
	zapFields := convertToZapFields(allFields)

	// Log based on level
	switch level {
	case log.DebugLevel:
		l.zapLogger.Debug(msg, zapFields...)
	case log.InfoLevel:
		l.zapLogger.Info(msg, zapFields...)
	case log.WarnLevel:
		l.zapLogger.Warn(msg, zapFields...)
	case log.ErrorLevel:
		l.zapLogger.Error(msg, zapFields...)
	case log.FatalLevel:
		l.zapLogger.Fatal(msg, zapFields...)
	}
}

// convertLogLevel converts our log.Level to zap's zapcore.Level.
func convertLogLevel(level log.Level) zapcore.Level {
	switch level {
	case log.DebugLevel:
		return zapcore.DebugLevel
	case log.InfoLevel:
		return zapcore.InfoLevel
	case log.WarnLevel:
		return zapcore.WarnLevel
	case log.ErrorLevel:
		return zapcore.ErrorLevel
	case log.FatalLevel:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// convertToZapFields converts our log.Field slice to zap.Field slice.
func convertToZapFields(fields []log.Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))
	for i, field := range fields {
		zapFields[i] = convertToZapField(field)
	}
	return zapFields
}

// convertToZapField converts a single log.Field to zap.Field.
func convertToZapField(field log.Field) zap.Field {
	switch v := field.Value.(type) {
	case string:
		return zap.String(field.Key, v)
	case int:
		return zap.Int(field.Key, v)
	case int64:
		return zap.Int64(field.Key, v)
	case float64:
		return zap.Float64(field.Key, v)
	case bool:
		return zap.Bool(field.Key, v)
	case time.Time:
		return zap.Time(field.Key, v)
	case time.Duration:
		return zap.Duration(field.Key, v)
	case error:
		return zap.Error(v)
	default:
		return zap.Any(field.Key, v)
	}
}

// getTimeEncoder returns the appropriate time encoder based on the format.
func getTimeEncoder(format string) zapcore.TimeEncoder {
	switch format {
	case time.RFC3339:
		return zapcore.RFC3339TimeEncoder
	case time.RFC3339Nano:
		return zapcore.RFC3339NanoTimeEncoder
	default:
		return func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(format))
		}
	}
}

// extractTraceID extracts trace ID from context (placeholder implementation).
func extractTraceID(ctx context.Context) string {
	// This is a placeholder - in a real implementation, you would extract
	// the trace ID from your tracing system (e.g., OpenTelemetry)
	if traceID := ctx.Value("trace_id"); traceID != nil {
		if str, ok := traceID.(string); ok {
			return str
		}
	}
	return ""
}

// extractRequestID extracts request ID from context (placeholder implementation).
func extractRequestID(ctx context.Context) string {
	// This is a placeholder - in a real implementation, you would extract
	// the request ID from your context
	if requestID := ctx.Value("request_id"); requestID != nil {
		if str, ok := requestID.(string); ok {
			return str
		}
	}
	return ""
}
