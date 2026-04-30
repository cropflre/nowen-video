package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
)

// ==================== STRM 远程流全局配置 ====================

// GetSTRMConfig 获取当前 STRM 全局配置
// GET /api/admin/strm/config
func (h *AdminHandler) GetSTRMConfig(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "配置未加载"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.cfg.STRM})
}

// UpdateSTRMConfig 更新 STRM 全局配置
// PUT /api/admin/strm/config
//
// Body:
//
//	{
//	  "default_user_agent": "Mozilla/5.0 ...",
//	  "default_referer": "",
//	  "connect_timeout": 30,
//	  "rewrite_hls": true,
//	  "remote_probe": true,
//	  "remote_probe_timeout": 8,
//	  "domain_user_agents": {"115.com": "Mozilla/5.0 ..."},
//	  "domain_referers":    {"115.com": "https://115.com/"}
//	}
func (h *AdminHandler) UpdateSTRMConfig(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "配置未加载"})
		return
	}
	var body config.STRMConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if body.ConnectTimeout < 0 || body.ConnectTimeout > 600 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connect_timeout 必须在 0-600 之间"})
		return
	}
	if body.RemoteProbeTimeout < 0 || body.RemoteProbeTimeout > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "remote_probe_timeout 必须在 0-120 之间"})
		return
	}

	// 覆盖写入内存 + 持久化到 config 文件
	h.cfg.STRM = body
	if err := h.cfg.SaveSTRMConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败: " + err.Error()})
		return
	}
	h.auditFromContext(c, "strm.update_config", "system", "strm", "")
	c.JSON(http.StatusOK, gin.H{
		"message": "STRM 配置已更新",
		"data":    h.cfg.STRM,
	})
}
