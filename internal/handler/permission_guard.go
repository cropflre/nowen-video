package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
)

// MediaPermissionGuard 媒体访问权限守卫
// 校验规则：
//  1. 管理员无限制，直接放行
//  2. 非管理员：根据 URL 中的 :id / :mediaId 查询媒体 library_id，校验 UserPermission
//  3. 同时校验内容分级、每日时长限制
//
// 参数 paramKey 指定从哪个 URL 参数读媒体 ID，常见 "id" 或 "mediaId"
func MediaPermissionGuard(permSvc *service.PermissionService, mediaRepo *repository.MediaRepo, paramKey string) gin.HandlerFunc {
	if paramKey == "" {
		paramKey = "id"
	}
	lookup := func(mediaID string) (string, error) {
		m, err := mediaRepo.FindByID(mediaID)
		if err != nil {
			return "", err
		}
		return m.LibraryID, nil
	}
	return func(c *gin.Context) {
		// 管理员放行
		if role, _ := c.Get("role"); role == "admin" {
			c.Next()
			return
		}
		mediaID := c.Param(paramKey)
		if mediaID == "" {
			c.Next()
			return
		}
		userIDRaw, ok := c.Get("user_id")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			c.Abort()
			return
		}
		userID := userIDRaw.(string)

		if err := permSvc.CheckMediaAccess(userID, mediaID, lookup); err != nil {
			switch err {
			case service.ErrForbidden:
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该媒体库的内容"})
			case service.ErrContentRestricted:
				c.JSON(http.StatusForbidden, gin.H{"error": "内容分级限制，无法观看该内容"})
			case service.ErrTimeLimitExceeded:
				c.JSON(http.StatusForbidden, gin.H{"error": "已超出每日观看时长限制"})
			default:
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

// LibraryPermissionGuard 媒体库访问权限守卫
// 从 query(?library_id=) 或 :id 参数中拿 library_id，非空时校验用户是否允许访问
func LibraryPermissionGuard(permSvc *service.PermissionService, paramKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, _ := c.Get("role"); role == "admin" {
			c.Next()
			return
		}
		userIDRaw, ok := c.Get("user_id")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
			c.Abort()
			return
		}
		userID := userIDRaw.(string)

		libID := c.Param(paramKey)
		if libID == "" {
			libID = c.Query("library_id")
		}
		if libID != "" {
			if !permSvc.CheckLibraryAccess(userID, libID) {
				c.JSON(http.StatusForbidden, gin.H{"error": "无权访问该媒体库"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
