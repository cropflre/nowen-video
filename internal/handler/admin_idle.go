package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// ==================== 潮汐调度器 API ====================

// GetIdleSchedulerStatus 获取潮汐调度器当前状态
func (h *AdminHandler) GetIdleSchedulerStatus(c *gin.Context) {
	if h.idleScheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "潮汐调度器未启用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.idleScheduler.GetStatus()})
}

// GetIdleSchedulerConfig 获取潮汐调度器配置
func (h *AdminHandler) GetIdleSchedulerConfig(c *gin.Context) {
	if h.idleScheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "潮汐调度器未启用"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.idleScheduler.GetConfig()})
}

// UpdateIdleSchedulerConfig 更新潮汐调度器配置
func (h *AdminHandler) UpdateIdleSchedulerConfig(c *gin.Context) {
	if h.idleScheduler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "潮汐调度器未启用"})
		return
	}
	var cfg service.TidalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 参数校验
	if cfg.IdleCPU > 0 && (cfg.IdleCPU < 1 || cfg.IdleCPU > 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "idle_cpu_percent 必须在 1-100 之间"})
		return
	}
	if cfg.BusyCPU > 0 && (cfg.BusyCPU < 1 || cfg.BusyCPU > 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "busy_cpu_percent 必须在 1-100 之间"})
		return
	}
	if cfg.PlayingCPU > 0 && (cfg.PlayingCPU < 1 || cfg.PlayingCPU > 100) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playing_cpu_percent 必须在 1-100 之间"})
		return
	}
	if cfg.PlayingAction != "" && cfg.PlayingAction != "pause" && cfg.PlayingAction != "throttle" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playing_action 只能为 pause 或 throttle"})
		return
	}
	if cfg.DebounceSec != 0 && (cfg.DebounceSec < 1 || cfg.DebounceSec > 300) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "debounce_sec 必须在 1-300 之间"})
		return
	}

	if err := h.idleScheduler.UpdateConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	h.auditFromContext(c, "idle_scheduler.update_config", "system", "idle_scheduler", "")
	c.JSON(http.StatusOK, gin.H{
		"message": "潮汐调度配置已更新",
		"data":    h.idleScheduler.GetConfig(),
	})
}
