package stdout

import (
	"context"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/pkg/log"
)

// BenchmarkStdoutLogger_Info benchmarks the Info method.
func BenchmarkStdoutLogger_Info(b *testing.B) {
	logger, err := New(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				log.String("key1", "value1"),
				log.Int("key2", 42),
				log.Bool("key3", true),
			)
		}
	})
}

// BenchmarkStdoutLogger_InfoWithFields benchmarks logging with many fields.
func BenchmarkStdoutLogger_InfoWithFields(b *testing.B) {
	logger, err := New(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	fields := []log.Field{
		log.String("service", "test-service"),
		log.String("version", "1.0.0"),
		log.String("environment", "production"),
		log.Int("user_id", 12345),
		log.String("request_id", "req-123-456"),
		log.String("trace_id", "trace-789-012"),
		log.Float64("duration", 123.45),
		log.Bool("success", true),
		log.Time("timestamp", time.Now()),
		log.String("method", "GET"),
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message with many fields", fields...)
		}
	})
}

// BenchmarkStdoutLogger_With benchmarks the With method.
func BenchmarkStdoutLogger_With(b *testing.B) {
	logger, err := New(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			childLogger := logger.With(
				log.String("service", "test"),
				log.Int("version", 1),
			)
			childLogger.Info("benchmark message")
		}
	})
}

// BenchmarkStdoutLogger_WithContext benchmarks the WithContext method.
func BenchmarkStdoutLogger_WithContext(b *testing.B) {
	logger, err := New(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
	ctx = context.WithValue(ctx, "request_id", "req-456")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			contextLogger := logger.WithContext(ctx)
			contextLogger.Info("benchmark message")
		}
	})
}

// BenchmarkPerformanceOptimizedLogger_Info benchmarks the performance-optimized logger.
func BenchmarkPerformanceOptimizedLogger_Info(b *testing.B) {
	logger, err := NewPerformanceOptimized(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create performance-optimized logger: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				log.String("key1", "value1"),
				log.Int("key2", 42),
				log.Bool("key3", true),
			)
		}
	})
}

// BenchmarkAsyncLogger_Info benchmarks the async logger.
func BenchmarkAsyncLogger_Info(b *testing.B) {
	logger, err := NewAsync(DefaultConfig(), 10000)
	if err != nil {
		b.Fatalf("Failed to create async logger: %v", err)
	}
	defer logger.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark message",
				log.String("key1", "value1"),
				log.Int("key2", 42),
				log.Bool("key3", true),
			)
		}
	})
}

// BenchmarkFieldConversion benchmarks field conversion performance.
func BenchmarkFieldConversion(b *testing.B) {
	fields := []log.Field{
		log.String("string", "value"),
		log.Int("int", 42),
		log.Int64("int64", 123456789),
		log.Float64("float64", 3.14),
		log.Bool("bool", true),
		log.Time("time", time.Now()),
		log.Duration("duration", 5*time.Second),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		convertToZapFields(fields)
	}
}

// BenchmarkFieldPool benchmarks the field pool performance.
func BenchmarkFieldPool(b *testing.B) {
	pool := NewFieldPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fields := pool.Get()
			fields = append(fields,
				log.String("key1", "value1"),
				log.Int("key2", 42),
				log.Bool("key3", true),
			)
			pool.Put(fields)
		}
	})
}

// BenchmarkZapFieldPool benchmarks the zap field pool performance.
func BenchmarkZapFieldPool(b *testing.B) {
	pool := NewZapFieldPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fields := pool.Get()
			// Simulate adding fields
			for i := 0; i < 5; i++ {
				fields = append(fields, convertToZapField(log.String("key", "value")))
			}
			pool.Put(fields)
		}
	})
}

// BenchmarkLoggerCreation benchmarks logger creation performance.
func BenchmarkLoggerCreation(b *testing.B) {
	config := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger, err := New(config)
		if err != nil {
			b.Fatalf("Failed to create logger: %v", err)
		}
		_ = logger
	}
}

// BenchmarkConfigBuilder benchmarks config builder performance.
func BenchmarkConfigBuilder(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := NewConfigBuilder().
			WithLevel(log.InfoLevel).
			WithTimeFormat(time.RFC3339).
			WithCaller(true).
			WithStacktrace(true).
			WithColors(false).
			WithDevelopment(false).
			Build()
		_ = config
	}
}

// BenchmarkLevelFiltering benchmarks log level filtering performance.
func BenchmarkLevelFiltering(b *testing.B) {
	config := DefaultConfig()
	config.Level = log.WarnLevel // Filter out debug and info
	logger, err := New(config)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// These should be filtered out
			logger.Debug("debug message", log.String("key", "value"))
			logger.Info("info message", log.String("key", "value"))
		}
	})
}

// BenchmarkConcurrentLogging benchmarks concurrent logging performance.
func BenchmarkConcurrentLogging(b *testing.B) {
	logger, err := New(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			go func() {
				logger.Info("concurrent message",
					log.String("goroutine", "test"),
					log.Int("number", 42),
				)
			}()
		}
	})
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns.
func BenchmarkMemoryAllocation(b *testing.B) {
	logger, err := NewPerformanceOptimized(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		logger.Info("memory allocation test",
			log.String("key1", "value1"),
			log.Int("key2", i),
			log.Bool("key3", i%2 == 0),
			log.Time("key4", time.Now()),
		)
	}
}

// BenchmarkComplexFields benchmarks logging with complex field types.
func BenchmarkComplexFields(b *testing.B) {
	logger, err := New(DefaultConfig())
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	complexData := map[string]interface{}{
		"nested": map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		"array": []int{1, 2, 3, 4, 5},
		"struct": struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{
			Name: "test",
			Age:  30,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("complex fields test",
				log.Any("complex", complexData),
				log.String("simple", "value"),
			)
		}
	})
}
