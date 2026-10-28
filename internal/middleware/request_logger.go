package middleware

import (
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequestLogger deliberately omits query strings: image, subtitle and stream
// URLs can carry authentication tokens that must not enter access logs.
func RequestLogger(output io.Writer) gin.HandlerFunc {
	return gin.LoggerWithConfig(gin.LoggerConfig{Output: output, Formatter: func(p gin.LogFormatterParams) string {
		path, _, _ := strings.Cut(p.Path, "?")
		return fmt.Sprintf("[HTTP] %s | %d | %v | %s | %s %q\n", p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode, p.Latency, p.ClientIP, p.Method, path)
	}})
}
