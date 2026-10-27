package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/service"
)

func deviceError(c *gin.Context, err error) {
	code, message := http.StatusServiceUnavailable, "暂时无法处理设备授权，请稍后重试"
	switch {
	case errors.Is(err, service.ErrPairUnavailable):
		code, message = http.StatusNotFound, err.Error()
	case errors.Is(err, service.ErrPairFinished):
		code, message = http.StatusGone, err.Error()
	case errors.Is(err, service.ErrPairBusy):
		code, message = http.StatusConflict, err.Error()
	case errors.Is(err, service.ErrPairCapacity):
		code, message = http.StatusTooManyRequests, err.Error()
	case errors.Is(err, service.ErrForbidden):
		code, message = http.StatusForbidden, "请先完成密码修改，再授权新设备"
	}
	c.JSON(code, gin.H{"error": message})
}

func (h *AuthHandler) RegisterDevicePairing(r *gin.Engine) {
	auth := r.Group("/api/auth/device-pairing")
	auth.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	})
	auth.POST("", middleware.RateLimit(10), func(c *gin.Context) {
		var req struct {
			Name string `json:"device_name" binding:"max=80"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请填写有效的设备名称"})
			return
		}
		data, err := h.authService.CreateDevicePairing(req.Name, c.Request.UserAgent())
		if err != nil {
			deviceError(c, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": data})
	})
	for _, action := range []string{"status", "cancel"} {
		action := action
		auth.POST("/"+action, middleware.RateLimit(90), func(c *gin.Context) {
			var req struct {
				ID     string `json:"id" binding:"required,max=80"`
				Secret string `json:"secret" binding:"required,max=128"`
			}
			if c.ShouldBindJSON(&req) != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "配对信息无效"})
				return
			}
			if action == "cancel" {
				if err := h.authService.CancelDevicePairing(req.ID, req.Secret); err != nil {
					deviceError(c, err)
					return
				}
				c.JSON(http.StatusOK, gin.H{"message": "配对已取消"})
				return
			}
			data, err := h.authService.PollDevicePairing(req.ID, req.Secret, c.ClientIP(), c.Request.UserAgent())
			if err != nil {
				deviceError(c, err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"data": data})
		})
	}
}

func (h *AuthHandler) RegisterDeviceManagement(api *gin.RouterGroup) {
	api.GET("/users/me/device-status", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"current_id": c.GetString("device_session_id")}})
	})
	api.GET("/users/me/devices", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		devices, err := h.authService.Devices(c.GetString("user_id"))
		if err != nil {
			deviceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": devices, "current_id": c.GetString("device_session_id")})
	})
	api.DELETE("/users/me/devices/:deviceId", func(c *gin.Context) {
		if err := h.authService.RevokeDevice(c.GetString("user_id"), c.Param("deviceId")); err != nil {
			deviceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "设备已退出"})
	})
	api.POST("/users/me/devices/revoke-all", func(c *gin.Context) {
		if err := h.authService.RevokeAllDevices(c.GetString("user_id")); err != nil {
			deviceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "全部设备已退出，请重新登录"})
	})
	api.POST("/auth/logout", func(c *gin.Context) {
		if err := h.authService.RevokeDevice(c.GetString("user_id"), c.GetString("device_session_id")); err != nil {
			deviceError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "已退出登录", "server_revoked": c.GetString("device_session_id") != ""})
	})
	for _, action := range []string{"inspect", "approve"} {
		action := action
		api.POST("/users/me/device-pairing/"+action, middleware.RateLimit(10), func(c *gin.Context) {
			c.Header("Cache-Control", "private, no-store")
			var req struct {
				Code string `json:"code" binding:"required,max=32"`
			}
			if c.ShouldBindJSON(&req) != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请输入新设备上的配对码"})
				return
			}
			if action == "inspect" {
				data, err := h.authService.InspectDevicePairing(c.GetString("user_id"), req.Code)
				if err != nil {
					deviceError(c, err)
					return
				}
				c.JSON(http.StatusOK, gin.H{"data": data})
				return
			}
			if err := h.authService.ApproveDevicePairing(c.GetString("user_id"), req.Code, c.GetInt("token_version"), c.GetString("device_session_id")); err != nil {
				deviceError(c, err)
				return
			}
			c.JSON(http.StatusOK, gin.H{"message": "已确认，请回到新设备"})
		})
	}
}
