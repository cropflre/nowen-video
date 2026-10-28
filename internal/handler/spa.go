package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// SPAFallback serves browser routes, while missing API routes keep an honest
// JSON 404. Returning index.html with 200 masks stale servers and wrong methods.
func SPAFallback(webDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		if c.Request.URL.Path == "/api" || strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": "API_NOT_FOUND", "error": "接口不存在，请确认服务版本或地址"})
			return
		}
		c.File(filepath.Join(webDir, "index.html"))
	}
}
