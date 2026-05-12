package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Logger writes the request log per memo-docs/standards/observability.md.
// /status*, /metrics, /webhook are demoted to debug to keep info-level logs readable.
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		if raw != "" {
			path = path + "?" + raw
		}

		var errMsg string
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}

		status := c.Writer.Status()

		isProbe := strings.Contains(path, "/status") || strings.Contains(path, "/metrics")
		isWebhook := strings.HasPrefix(path, "/webhook")

		var logger = log.Info()
		switch {
		case isProbe || isWebhook:
			logger = log.Debug()
		case status >= 500:
			logger = log.Error().Str("error", errMsg)
		case status >= 400:
			logger = log.Warn().Str("error", errMsg)
		}

		logger.Str("method", c.Request.Method).
			Str("path", path).
			Int("status", status).
			Str("ip", c.ClientIP()).
			Dur("latency_ms", latency).
			Msg("request handled")
	}
}
