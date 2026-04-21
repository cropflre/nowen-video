package emby

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== Images ====================
//
// Emby 客户端通过这个系列接口拉取海报/背景图：
//
//   GET /Items/{id}/Images/{type}
//   GET /Items/{id}/Images/{type}/{index}
//   GET /Items/{id}/Images/{type}?tag=xxx&maxWidth=500
//
// `type` 常见取值：Primary / Backdrop / Thumb / Logo / Banner。
// 我们主要支持 Primary（海报）和 Backdrop（背景），其它类型若找不到就 404，
// Infuse 会自动退回到 Primary。

// ImageHandler 统一处理 /Items/{id}/Images/{type}(/{index})。
func (h *Handler) ImageHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.Status(http.StatusNotFound)
		return
	}
	imgType := strings.ToLower(c.Param("type"))

	// 解析实际路径
	var imgPath string
	switch imgType {
	case "primary", "":
		imgPath = h.resolvePrimaryImage(uuid)
	case "backdrop":
		imgPath = h.resolveBackdropImage(uuid)
	case "thumb":
		// Thumb 退化到 Primary
		imgPath = h.resolvePrimaryImage(uuid)
	case "logo", "banner", "art", "disc", "box":
		// 这些类型我们不存储，返回 404 让客户端退回 Primary
		c.Status(http.StatusNotFound)
		return
	default:
		imgPath = h.resolvePrimaryImage(uuid)
	}

	if imgPath == "" {
		c.Status(http.StatusNotFound)
		return
	}
	fi, err := os.Stat(imgPath)
	if err != nil || fi.IsDir() {
		c.Status(http.StatusNotFound)
		return
	}

	// 根据扩展名设 Content-Type
	ext := strings.ToLower(filepath.Ext(imgPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.File(imgPath)
}

// resolvePrimaryImage 解析条目的海报文件路径。
// 先尝试 Media（含 fallback 到 Series 的逻辑复用 StreamService.GetPosterPath），
// 再尝试 Series/Library 自身的 PosterPath。
func (h *Handler) resolvePrimaryImage(uuid string) string {
	// 1) Media → 复用 streamService 已有的 fallback 逻辑
	if path, err := h.stream.GetPosterPath(uuid); err == nil && path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// 2) Series
	if s, err := h.seriesRepo.FindByIDOnly(uuid); err == nil {
		if s.PosterPath != "" {
			if _, err := os.Stat(s.PosterPath); err == nil {
				return s.PosterPath
			}
		}
		// Series 根目录下常见的海报文件
		if s.FolderPath != "" {
			for _, n := range []string{"poster.jpg", "poster.png", "folder.jpg", "cover.jpg"} {
				candidate := filepath.Join(s.FolderPath, n)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
	}

	// 3) Library（一般没有图，返回空）
	return ""
}

// resolveBackdropImage 解析背景图路径。
func (h *Handler) resolveBackdropImage(uuid string) string {
	// Media
	if m, err := h.mediaRepo.FindByID(uuid); err == nil {
		if m.BackdropPath != "" {
			if _, err := os.Stat(m.BackdropPath); err == nil {
				return m.BackdropPath
			}
		}
		// 同目录下的 fanart.jpg
		if m.FilePath != "" {
			candidate := filepath.Join(filepath.Dir(m.FilePath), "fanart.jpg")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	// Series
	if s, err := h.seriesRepo.FindByIDOnly(uuid); err == nil {
		if s.BackdropPath != "" {
			if _, err := os.Stat(s.BackdropPath); err == nil {
				return s.BackdropPath
			}
		}
		if s.FolderPath != "" {
			for _, n := range []string{"fanart.jpg", "backdrop.jpg"} {
				candidate := filepath.Join(s.FolderPath, n)
				if _, err := os.Stat(candidate); err == nil {
					return candidate
				}
			}
		}
	}
	return ""
}
