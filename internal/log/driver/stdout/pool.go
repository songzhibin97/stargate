package stdout

import (
	"sync"

	"github.com/songzhibin97/stargate/pkg/log"
	"go.uber.org/zap"
)

// FieldPool manages a pool of field slices to reduce memory allocations.
type FieldPool struct {
	pool sync.Pool
}

// NewFieldPool creates a new FieldPool.
func NewFieldPool() *FieldPool {
	return &FieldPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate slice with capacity for common use cases
				return make([]log.Field, 0, 8)
			},
		},
	}
}

// Get retrieves a field slice from the pool.
func (p *FieldPool) Get() []log.Field {
	return p.pool.Get().([]log.Field)
}

// Put returns a field slice to the pool after clearing it.
func (p *FieldPool) Put(fields []log.Field) {
	// Clear the slice but keep the underlying array
	fields = fields[:0]
	p.pool.Put(fields)
}

// ZapFieldPool manages a pool of zap field slices to reduce memory allocations.
type ZapFieldPool struct {
	pool sync.Pool
}

// NewZapFieldPool creates a new ZapFieldPool.
func NewZapFieldPool() *ZapFieldPool {
	return &ZapFieldPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate slice with capacity for common use cases
				return make([]zap.Field, 0, 8)
			},
		},
	}
}

// Get retrieves a zap field slice from the pool.
func (p *ZapFieldPool) Get() []zap.Field {
	return p.pool.Get().([]zap.Field)
}

// Put returns a zap field slice to the pool after clearing it.
func (p *ZapFieldPool) Put(fields []zap.Field) {
	// Clear the slice but keep the underlying array
	fields = fields[:0]
	p.pool.Put(fields)
}

// BufferPool manages a pool of byte buffers for JSON serialization.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new BufferPool.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				// Pre-allocate buffer with reasonable initial capacity
				return make([]byte, 0, 256)
			},
		},
	}
}

// Get retrieves a byte buffer from the pool.
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a byte buffer to the pool after clearing it.
func (p *BufferPool) Put(buf []byte) {
	// Clear the buffer but keep the underlying array
	buf = buf[:0]
	p.pool.Put(buf)
}

// PerformanceOptimizedLogger wraps StdoutLogger with performance optimizations.
type PerformanceOptimizedLogger struct {
	*StdoutLogger
	fieldPool    *FieldPool
	zapFieldPool *ZapFieldPool
	bufferPool   *BufferPool
}

// NewPerformanceOptimized creates a new performance-optimized StdoutLogger.
func NewPerformanceOptimized(config *Config) (*PerformanceOptimizedLogger, error) {
	baseLogger, err := New(config)
	if err != nil {
		return nil, err
	}

	return &PerformanceOptimizedLogger{
		StdoutLogger: baseLogger,
		fieldPool:    NewFieldPool(),
		zapFieldPool: NewZapFieldPool(),
		bufferPool:   NewBufferPool(),
	}, nil
}

// log overrides the base log method with performance optimizations.
func (l *PerformanceOptimizedLogger) log(level log.Level, msg string, fields ...log.Field) {
	// Check if logging is enabled for this level (early return)
	if level < l.config.Level {
		return
	}

	// Use pooled slices to reduce allocations
	allFields := l.fieldPool.Get()
	defer l.fieldPool.Put(allFields)

	// Combine existing fields with new fields
	l.mu.RLock()
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)
	l.mu.RUnlock()

	// Use pooled zap fields
	zapFields := l.zapFieldPool.Get()
	defer l.zapFieldPool.Put(zapFields)

	// Convert to zap fields efficiently
	zapFields = convertToZapFieldsPooled(allFields, zapFields)

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

// convertToZapFieldsPooled converts log fields to zap fields using a pooled slice.
func convertToZapFieldsPooled(fields []log.Field, zapFields []zap.Field) []zap.Field {
	// Ensure capacity
	if cap(zapFields) < len(fields) {
		zapFields = make([]zap.Field, 0, len(fields))
	}

	for _, field := range fields {
		zapFields = append(zapFields, convertToZapFieldOptimized(field))
	}
	return zapFields
}

// convertToZapFieldOptimized is an optimized version of field conversion.
func convertToZapFieldOptimized(field log.Field) zap.Field {
	// Use type assertions for better performance
	switch v := field.Value.(type) {
	case string:
		return zap.String(field.Key, v)
	case int:
		return zap.Int(field.Key, v)
	case int32:
		return zap.Int32(field.Key, v)
	case int64:
		return zap.Int64(field.Key, v)
	case uint:
		return zap.Uint(field.Key, v)
	case uint32:
		return zap.Uint32(field.Key, v)
	case uint64:
		return zap.Uint64(field.Key, v)
	case float32:
		return zap.Float32(field.Key, v)
	case float64:
		return zap.Float64(field.Key, v)
	case bool:
		return zap.Bool(field.Key, v)
	case []byte:
		return zap.ByteString(field.Key, v)
	case error:
		if field.Key == "error" {
			return zap.Error(v)
		}
		return zap.NamedError(field.Key, v)
	default:
		// Fallback to Any for complex types
		return zap.Any(field.Key, v)
	}
}

// Debug logs a debug message with performance optimizations.
func (l *PerformanceOptimizedLogger) Debug(msg string, fields ...log.Field) {
	l.log(log.DebugLevel, msg, fields...)
}

// Info logs an info message with performance optimizations.
func (l *PerformanceOptimizedLogger) Info(msg string, fields ...log.Field) {
	l.log(log.InfoLevel, msg, fields...)
}

// Warn logs a warning message with performance optimizations.
func (l *PerformanceOptimizedLogger) Warn(msg string, fields ...log.Field) {
	l.log(log.WarnLevel, msg, fields...)
}

// Error logs an error message with performance optimizations.
func (l *PerformanceOptimizedLogger) Error(msg string, fields ...log.Field) {
	l.log(log.ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message with performance optimizations.
func (l *PerformanceOptimizedLogger) Fatal(msg string, fields ...log.Field) {
	l.log(log.FatalLevel, msg, fields...)
}

// With creates a new logger with additional fields, maintaining performance optimizations.
func (l *PerformanceOptimizedLogger) With(fields ...log.Field) log.Logger {
	baseLogger := l.StdoutLogger.With(fields...)
	
	return &PerformanceOptimizedLogger{
		StdoutLogger: baseLogger.(*StdoutLogger),
		fieldPool:    l.fieldPool,    // Share pools
		zapFieldPool: l.zapFieldPool, // Share pools
		bufferPool:   l.bufferPool,   // Share pools
	}
}

// AsyncLogger provides asynchronous logging capabilities.
type AsyncLogger struct {
	*PerformanceOptimizedLogger
	logChan   chan logEntry
	done      chan struct{}
	wg        sync.WaitGroup
	bufferSize int
}

// logEntry represents a log entry for async processing.
type logEntry struct {
	level  log.Level
	msg    string
	fields []log.Field
}

// NewAsync creates a new asynchronous logger.
func NewAsync(config *Config, bufferSize int) (*AsyncLogger, error) {
	if bufferSize <= 0 {
		bufferSize = 1000 // Default buffer size
	}

	baseLogger, err := NewPerformanceOptimized(config)
	if err != nil {
		return nil, err
	}

	asyncLogger := &AsyncLogger{
		PerformanceOptimizedLogger: baseLogger,
		logChan:                   make(chan logEntry, bufferSize),
		done:                      make(chan struct{}),
		bufferSize:                bufferSize,
	}

	// Start background goroutine
	asyncLogger.wg.Add(1)
	go asyncLogger.processLogs()

	return asyncLogger, nil
}

// processLogs processes log entries in the background.
func (l *AsyncLogger) processLogs() {
	defer l.wg.Done()
	
	for {
		select {
		case entry := <-l.logChan:
			l.PerformanceOptimizedLogger.log(entry.level, entry.msg, entry.fields...)
		case <-l.done:
			// Process remaining entries
			for {
				select {
				case entry := <-l.logChan:
					l.PerformanceOptimizedLogger.log(entry.level, entry.msg, entry.fields...)
				default:
					return
				}
			}
		}
	}
}

// log queues a log entry for async processing.
func (l *AsyncLogger) log(level log.Level, msg string, fields ...log.Field) {
	// Check if logging is enabled for this level (early return)
	if level < l.config.Level {
		return
	}

	// Copy fields to avoid race conditions
	fieldsCopy := make([]log.Field, len(fields))
	copy(fieldsCopy, fields)

	entry := logEntry{
		level:  level,
		msg:    msg,
		fields: fieldsCopy,
	}

	select {
	case l.logChan <- entry:
		// Successfully queued
	default:
		// Channel is full, fall back to synchronous logging
		l.PerformanceOptimizedLogger.log(level, msg, fields...)
	}
}

// Close shuts down the async logger gracefully.
func (l *AsyncLogger) Close() error {
	close(l.done)
	l.wg.Wait()
	return nil
}

// Debug logs a debug message asynchronously.
func (l *AsyncLogger) Debug(msg string, fields ...log.Field) {
	l.log(log.DebugLevel, msg, fields...)
}

// Info logs an info message asynchronously.
func (l *AsyncLogger) Info(msg string, fields ...log.Field) {
	l.log(log.InfoLevel, msg, fields...)
}

// Warn logs a warning message asynchronously.
func (l *AsyncLogger) Warn(msg string, fields ...log.Field) {
	l.log(log.WarnLevel, msg, fields...)
}

// Error logs an error message asynchronously.
func (l *AsyncLogger) Error(msg string, fields ...log.Field) {
	l.log(log.ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message synchronously (cannot be async due to os.Exit).
func (l *AsyncLogger) Fatal(msg string, fields ...log.Field) {
	l.PerformanceOptimizedLogger.Fatal(msg, fields...)
}
