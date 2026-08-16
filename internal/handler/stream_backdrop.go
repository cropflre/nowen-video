package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// Backdrop serves the real wide artwork associated with a media item. It
// deliberately returns 404 when no backdrop exists so the client can fall back
// to the poster and keep the existing blurred-poster treatment.
func (h *StreamHandler) Backdrop(c *gin.Context) {
	id := c.Param("id")
	backdropPath, err := h.streamService.GetBackdropPath(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if backdropPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体背景图不存在"})
		return
	}

	if service.IsWebDAVPath(backdropPath) {
		vfsFile, openErr := h.streamService.OpenMediaFile(backdropPath)
		if openErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "媒体背景图不可用"})
			return
		}
		defer vfsFile.Close()

		stat, statErr := vfsFile.Stat()
		if statErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "媒体背景图不可用"})
			return
		}
		etag := fmt.Sprintf(`"%x-%x"`, stat.ModTime().UnixNano(), stat.Size())
		c.Header("ETag", etag)
		if match := c.GetHeader("If-None-Match"); match == etag {
			c.Status(http.StatusNotModified)
			return
		}
		setPosterContentType(c, backdropPath)
		c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
		reader := service.NewVFSReadSeeker(vfsFile, stat.Size())
		http.ServeContent(c.Writer, c.Request, filepath.Base(backdropPath), stat.ModTime(), reader)
		return
	}

	fileInfo, statErr := os.Stat(backdropPath)
	if statErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体背景图不可用"})
		return
	}

	etag := fmt.Sprintf(`"%x-%x"`, fileInfo.ModTime().UnixNano(), fileInfo.Size())
	c.Header("ETag", etag)
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	setPosterContentType(c, backdropPath)
	c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
	c.File(backdropPath)
}
