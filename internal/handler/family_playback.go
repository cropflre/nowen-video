package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

func RegisterFamilyPlayback(api *gin.RouterGroup, db *gorm.DB) {
	s := service.NewFamilyPlaybackService(db)
	write := func(c *gin.Context, data service.FamilyPlaybackStatus, err error) {
		c.Header("Cache-Control", "private, no-store")
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"data": data})
			return
		}
		status, code, message := http.StatusServiceUnavailable, "FAMILY_UNAVAILABLE", "暂时无法确认观看时间，请稍后重试"
		switch {
		case errors.Is(err, service.ErrTimeLimitExceeded):
			status, code, message = http.StatusForbidden, "FAMILY_TIME_LIMIT", "今天的观看时间已用完，休息一下吧"
		case errors.Is(err, service.ErrFamilyLeaseBusy):
			status, code, message = http.StatusConflict, "FAMILY_IN_USE", err.Error()
		case errors.Is(err, service.ErrFamilyLeaseExpired):
			status, code, message = http.StatusConflict, "FAMILY_LEASE_EXPIRED", err.Error()
		case errors.Is(err, service.ErrContentRestricted), errors.Is(err, service.ErrForbidden):
			status, code, message = http.StatusForbidden, "FAMILY_FORBIDDEN", "此内容暂未向儿童账号开放"
		}
		c.JSON(status, gin.H{"error": message, "code": code})
	}
	api.GET("/users/me/family-controls", func(c *gin.Context) { data, err := s.Status(c.GetString("user_id")); write(c, data, err) })
	api.POST("/users/me/viewing/start", func(c *gin.Context) {
		var req struct {
			MediaID string `json:"media_id" binding:"required"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请选择影片"})
			return
		}
		data, err := s.Start(c.GetString("user_id"), req.MediaID)
		write(c, data, err)
	})
	for _, action := range []string{"renew", "stop"} {
		action := action
		api.POST("/users/me/viewing/"+action, func(c *gin.Context) {
			var req struct {
				LeaseID string `json:"lease_id" binding:"required"`
			}
			if c.ShouldBindJSON(&req) != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "观看授权无效"})
				return
			}
			var data service.FamilyPlaybackStatus
			var err error
			if action == "renew" {
				data, err = s.Renew(c.GetString("user_id"), req.LeaseID)
			} else {
				data, err = s.Stop(c.GetString("user_id"), req.LeaseID)
			}
			write(c, data, err)
		})
	}
}
