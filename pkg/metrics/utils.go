package metrics

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Metric name validation regex
var (
	metricNameRegex = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	labelNameRegex  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// Reserved label names that should not be used
var reservedLabelNames = map[string]bool{
	"__name__":     true,
	"__value__":    true,
	"__timestamp__": true,
	"job":          true,
	"instance":     true,
}

// ValidateMetricName validates a metric name according to Prometheus conventions
func ValidateMetricName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: metric name cannot be empty", ErrInvalidName)
	}
	
	if !metricNameRegex.MatchString(name) {
		return fmt.Errorf("%w: metric name '%s' is invalid", ErrInvalidName, name)
	}
	
	return nil
}

// ValidateLabelName validates a label name according to Prometheus conventions
func ValidateLabelName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: label name cannot be empty", ErrInvalidLabel)
	}
	
	if !labelNameRegex.MatchString(name) {
		return fmt.Errorf("%w: label name '%s' is invalid", ErrInvalidLabel, name)
	}
	
	if reservedLabelNames[name] {
		return fmt.Errorf("%w: label name '%s' is reserved", ErrInvalidLabel, name)
	}
	
	return nil
}

// ValidateLabelNames validates multiple label names
func ValidateLabelNames(names []string) error {
	seen := make(map[string]bool)
	for _, name := range names {
		if err := ValidateLabelName(name); err != nil {
			return err
		}
		if seen[name] {
			return fmt.Errorf("%w: duplicate label name '%s'", ErrInvalidLabel, name)
		}
		seen[name] = true
	}
	return nil
}

// ValidateLabelValues validates that label values match label names
func ValidateLabelValues(names []string, values []string) error {
	if len(names) != len(values) {
		return fmt.Errorf("%w: label names and values count mismatch", ErrInvalidLabel)
	}
	return nil
}

// ValidateHistogramBuckets validates histogram buckets
func ValidateHistogramBuckets(buckets []float64) error {
	if len(buckets) == 0 {
		return fmt.Errorf("%w: buckets cannot be empty", ErrInvalidBuckets)
	}
	
	for i, bucket := range buckets {
		if i > 0 && bucket <= buckets[i-1] {
			return fmt.Errorf("%w: buckets must be sorted in increasing order", ErrInvalidBuckets)
		}
	}
	
	return nil
}

// ValidateSummaryObjectives validates summary objectives
func ValidateSummaryObjectives(objectives map[float64]float64) error {
	if len(objectives) == 0 {
		return fmt.Errorf("%w: objectives cannot be empty", ErrInvalidObjectives)
	}
	
	for quantile, error := range objectives {
		if quantile < 0 || quantile > 1 {
			return fmt.Errorf("%w: quantile %f must be between 0 and 1", ErrInvalidObjectives, quantile)
		}
		if error < 0 || error > 1 {
			return fmt.Errorf("%w: error %f must be between 0 and 1", ErrInvalidObjectives, error)
		}
	}
	
	return nil
}

// BuildFQName builds a fully qualified metric name
func BuildFQName(namespace, subsystem, name string) string {
	if namespace == "" && subsystem == "" {
		return name
	}
	
	parts := make([]string, 0, 3)
	if namespace != "" {
		parts = append(parts, namespace)
	}
	if subsystem != "" {
		parts = append(parts, subsystem)
	}
	if name != "" {
		parts = append(parts, name)
	}
	
	return strings.Join(parts, "_")
}

// NormalizeLabelValue normalizes a label value
func NormalizeLabelValue(value string) string {
	// Remove any control characters and normalize whitespace
	return strings.TrimSpace(value)
}

// MergeLabelMaps merges multiple label maps, with later maps taking precedence
func MergeLabelMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// SortLabelPairs sorts label pairs by name for consistent output
func SortLabelPairs(pairs []LabelPair) {
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Name < pairs[j].Name
	})
}

// LabelsToLabelPairs converts a map of labels to sorted label pairs
func LabelsToLabelPairs(labels map[string]string) []LabelPair {
	pairs := make([]LabelPair, 0, len(labels))
	for name, value := range labels {
		pairs = append(pairs, LabelPair{
			Name:  name,
			Value: value,
		})
	}
	SortLabelPairs(pairs)
	return pairs
}

// LabelPairsToLabels converts label pairs to a map
func LabelPairsToLabels(pairs []LabelPair) map[string]string {
	labels := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		labels[pair.Name] = pair.Value
	}
	return labels
}

// FormatDuration formats a duration for metric help text
func FormatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%.0fns", float64(d.Nanoseconds()))
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.0fμs", float64(d.Nanoseconds())/1000)
	}
	if d < time.Second {
		return fmt.Sprintf("%.0fms", float64(d.Nanoseconds())/1000000)
	}
	return d.String()
}

// FormatBytes formats bytes for metric help text
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// SanitizeMetricName sanitizes a metric name to be valid
func SanitizeMetricName(name string) string {
	// Replace invalid characters with underscores
	result := strings.Builder{}
	for i, r := range name {
		if i == 0 {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == ':' {
				result.WriteRune(r)
			} else {
				result.WriteRune('_')
			}
		} else {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == ':' {
				result.WriteRune(r)
			} else {
				result.WriteRune('_')
			}
		}
	}
	return result.String()
}

// SanitizeLabelName sanitizes a label name to be valid
func SanitizeLabelName(name string) string {
	// Replace invalid characters with underscores
	result := strings.Builder{}
	for i, r := range name {
		if i == 0 {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
				result.WriteRune(r)
			} else {
				result.WriteRune('_')
			}
		} else {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
				result.WriteRune(r)
			} else {
				result.WriteRune('_')
			}
		}
	}
	return result.String()
}

// GetDefaultBuckets returns default histogram buckets for different use cases
func GetDefaultBuckets(bucketType string) []float64 {
	switch bucketType {
	case "duration":
		return []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	case "size":
		return ExponentialBuckets(100, 10, 8) // 100B to 1GB
	case "latency":
		return []float64{.0001, .0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1}
	default:
		return DefaultBuckets
	}
}

// GetDefaultObjectives returns default summary objectives
func GetDefaultObjectives() map[float64]float64 {
	return map[float64]float64{
		0.5:  0.05,
		0.9:  0.01,
		0.99: 0.001,
	}
}

// IsValidMetricValue checks if a metric value is valid (not NaN or Inf)
func IsValidMetricValue(value float64) bool {
	return !isNaN(value) && !isInf(value)
}

// Helper functions for NaN and Inf detection
func isNaN(f float64) bool {
	return f != f
}

func isInf(f float64) bool {
	return f > 1.7976931348623157e+308 || f < -1.7976931348623157e+308
}
