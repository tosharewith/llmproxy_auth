package middleware

import (
	"net/http"
	"time"

	"github.com/tosharewith/llmproxy_auth/pkg/metrics"
	"github.com/gin-gonic/gin"
)

// Metrics middleware records HTTP metrics
func Metrics() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method

		metrics.HTTPRequestDuration.WithLabelValues(method, c.FullPath()).Observe(duration.Seconds())
		metrics.HTTPRequestsTotal.WithLabelValues(method, c.FullPath(), http.StatusText(status)).Inc()

		if status >= 400 {
			metrics.HTTPRequestErrors.WithLabelValues(method, c.FullPath()).Inc()
		}
	})
}
