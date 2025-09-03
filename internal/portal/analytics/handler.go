package analytics

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songzhibin97/stargate/internal/portal/auth"
	"github.com/songzhibin97/stargate/pkg/log"
)

// Handler handles analytics HTTP requests
type Handler struct {
	service *Service
	logger  log.Logger
}

// NewHandler creates a new analytics handler
func NewHandler(service *Service, logger log.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers analytics routes
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	analytics := router.Group("/analytics")
	{
		analytics.GET("/summary", h.GetAnalyticsSummary)
		analytics.GET("/health", h.GetHealth)
	}
}

// GetAnalyticsSummary handles GET /api/analytics/summary
func (h *Handler) GetAnalyticsSummary(c *gin.Context) {
	// Extract user from JWT token
	user, exists := auth.GetUserFromContext(c.Request.Context())
	if !exists {
		h.logger.Warn("User not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "unauthorized",
			"message": "User authentication required",
		})
		return
	}

	// Parse request parameters
	req, err := h.parseAnalyticsRequest(c)
	if err != nil {
		h.logger.Error("Failed to parse analytics request", 
			log.String("user_id", user.ID),
			log.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	// Log the request
	h.logger.Info("Analytics summary requested",
		log.String("user_id", user.ID),
		log.String("time_range", req.TimeRange),
		log.String("application_ids", strings.Join(req.ApplicationIDs, ",")),
		log.String("granularity", req.Granularity))

	// Get analytics data
	response, err := h.service.GetAnalyticsSummary(c.Request.Context(), user.ID, req)
	if err != nil {
		h.logger.Error("Failed to get analytics summary",
			log.String("user_id", user.ID),
			log.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Failed to retrieve analytics data",
		})
		return
	}

	// Log successful response
	h.logger.Info("Analytics summary retrieved successfully",
		log.String("user_id", user.ID),
		log.Int64("total_requests", response.Summary.TotalRequests),
		log.Int("application_count", response.Metadata.ApplicationCount),
		log.Int64("query_duration_ms", response.Metadata.QueryDuration))

	c.JSON(http.StatusOK, response)
}

// GetHealth handles GET /api/analytics/health
func (h *Handler) GetHealth(c *gin.Context) {
	err := h.service.Health(c.Request.Context())
	if err != nil {
		h.logger.Error("Analytics service health check failed", log.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"timestamp": time.Now().UTC(),
	})
}

// parseAnalyticsRequest parses the analytics request from query parameters
func (h *Handler) parseAnalyticsRequest(c *gin.Context) (*AnalyticsSummaryRequest, error) {
	req := &AnalyticsSummaryRequest{}

	// Parse time range
	if timeRange := c.Query("time_range"); timeRange != "" {
		req.TimeRange = timeRange
	}

	// Parse start_time
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return nil, err
		}
		req.StartTime = &startTime
	}

	// Parse end_time
	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return nil, err
		}
		req.EndTime = &endTime
	}

	// Parse application_ids
	if appIDsStr := c.Query("application_ids"); appIDsStr != "" {
		req.ApplicationIDs = strings.Split(appIDsStr, ",")
		// Trim whitespace from each ID
		for i, id := range req.ApplicationIDs {
			req.ApplicationIDs[i] = strings.TrimSpace(id)
		}
	}

	// Parse granularity
	if granularity := c.Query("granularity"); granularity != "" {
		req.Granularity = granularity
	}

	// Parse metrics
	if metricsStr := c.Query("metrics"); metricsStr != "" {
		req.Metrics = strings.Split(metricsStr, ",")
		// Trim whitespace from each metric
		for i, metric := range req.Metrics {
			req.Metrics[i] = strings.TrimSpace(metric)
		}
	}

	return req, nil
}

// AnalyticsMiddleware provides middleware for analytics endpoints
type AnalyticsMiddleware struct {
	logger log.Logger
}

// NewAnalyticsMiddleware creates a new analytics middleware
func NewAnalyticsMiddleware(logger log.Logger) *AnalyticsMiddleware {
	return &AnalyticsMiddleware{
		logger: logger,
	}
}

// RateLimitMiddleware provides rate limiting for analytics endpoints
func (m *AnalyticsMiddleware) RateLimitMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Simple rate limiting based on user ID
		user, exists := auth.GetUserFromContext(c.Request.Context())
		if !exists {
			c.Next()
			return
		}

		// TODO: Implement proper rate limiting with Redis or in-memory store
		// For now, just log the request
		m.logger.Debug("Analytics request rate limit check",
			log.String("user_id", user.ID),
			log.String("endpoint", c.Request.URL.Path),
			log.String("method", c.Request.Method))

		c.Next()
	})
}

// CacheMiddleware provides caching for analytics responses
func (m *AnalyticsMiddleware) CacheMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// TODO: Implement response caching
		// For now, just add cache headers
		c.Header("Cache-Control", "public, max-age=300") // 5 minutes cache
		c.Next()
	})
}

// ValidationMiddleware validates analytics request parameters
func (m *AnalyticsMiddleware) ValidationMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Validate time_range parameter
		if timeRange := c.Query("time_range"); timeRange != "" {
			if _, err := ParseTimeRange(timeRange); err != nil {
				m.logger.Warn("Invalid time_range parameter",
					log.String("time_range", timeRange),
					log.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_time_range",
					"message": "Invalid time range format. Use formats like '1h', '24h', '7d', or '30d'",
				})
				c.Abort()
				return
			}
		}

		// Validate start_time and end_time
		var startTime, endTime *time.Time
		if startTimeStr := c.Query("start_time"); startTimeStr != "" {
			t, err := time.Parse(time.RFC3339, startTimeStr)
			if err != nil {
				m.logger.Warn("Invalid start_time parameter",
					log.String("start_time", startTimeStr),
					log.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_start_time",
					"message": "Invalid start_time format. Use RFC3339 format (e.g., '2023-01-01T00:00:00Z')",
				})
				c.Abort()
				return
			}
			startTime = &t
		}

		if endTimeStr := c.Query("end_time"); endTimeStr != "" {
			t, err := time.Parse(time.RFC3339, endTimeStr)
			if err != nil {
				m.logger.Warn("Invalid end_time parameter",
					log.String("end_time", endTimeStr),
					log.Error(err))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_end_time",
					"message": "Invalid end_time format. Use RFC3339 format (e.g., '2023-01-01T23:59:59Z')",
				})
				c.Abort()
				return
			}
			endTime = &t
		}

		// Validate time range if both start and end are provided
		if startTime != nil && endTime != nil {
			if endTime.Before(*startTime) {
				m.logger.Warn("Invalid time range: end_time before start_time",
					log.Time("start_time", *startTime),
					log.Time("end_time", *endTime))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_time_range",
					"message": "end_time must be after start_time",
				})
				c.Abort()
				return
			}

			// Limit maximum time range to prevent excessive queries
			maxRange := 90 * 24 * time.Hour // 90 days
			if endTime.Sub(*startTime) > maxRange {
				m.logger.Warn("Time range too large",
					log.Time("start_time", *startTime),
					log.Time("end_time", *endTime),
					log.Duration("duration", endTime.Sub(*startTime)))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "time_range_too_large",
					"message": "Time range cannot exceed 90 days",
				})
				c.Abort()
				return
			}
		}

		// Validate granularity
		if granularity := c.Query("granularity"); granularity != "" {
			validGranularities := []string{"1m", "5m", "15m", "30m", "1h", "6h", "12h", "1d"}
			valid := false
			for _, validGran := range validGranularities {
				if granularity == validGran {
					valid = true
					break
				}
			}
			if !valid {
				m.logger.Warn("Invalid granularity parameter",
					log.String("granularity", granularity))
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_granularity",
					"message": "Invalid granularity. Valid values: " + strings.Join(validGranularities, ", "),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	})
}
