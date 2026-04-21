package emby

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== 用户-条目交互 ====================
//
// Emby 客户端做"加收藏 / 标记已看 / 手动设置进度"时会走这些接口：
//
//   POST   /Users/{userId}/FavoriteItems/{itemId}    -> 加收藏
//   DELETE /Users/{userId}/FavoriteItems/{itemId}    -> 取消收藏
//   POST   /Users/{userId}/PlayedItems/{itemId}      -> 标记已播放
//   DELETE /Users/{userId}/PlayedItems/{itemId}      -> 标记未播放
//   POST   /Users/{userId}/Items/{itemId}/UserData   -> 批量更新 UserData
//
// 所有接口返回最新的 UserItemData。

// AddFavoriteHandler POST /Users/{userId}/FavoriteItems/{itemId}
func (h *Handler) AddFavoriteHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid params"})
		return
	}
	// 去重
	if !h.favoriteRepo.Exists(userID, uuid) {
		if err := h.favoriteRepo.Add(&model.Favorite{UserID: userID, MediaID: uuid}); err != nil {
			h.logger.Warnf("[emby] add favorite failed user=%s media=%s err=%v", userID, uuid, err)
			c.JSON(http.StatusInternalServerError, gin.H{"Error": err.Error()})
			return
		}
	}
	h.writeUserData(c, userID, uuid)
}

// RemoveFavoriteHandler DELETE /Users/{userId}/FavoriteItems/{itemId}
func (h *Handler) RemoveFavoriteHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid params"})
		return
	}
	if err := h.favoriteRepo.Remove(userID, uuid); err != nil {
		h.logger.Warnf("[emby] remove favorite failed user=%s media=%s err=%v", userID, uuid, err)
	}
	h.writeUserData(c, userID, uuid)
}

// MarkPlayedHandler POST /Users/{userId}/PlayedItems/{itemId}
func (h *Handler) MarkPlayedHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid params"})
		return
	}
	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Media not found"})
		return
	}
	duration := m.Duration
	if duration <= 0 && m.Runtime > 0 {
		duration = float64(m.Runtime) * 60
	}
	_ = h.watchRepo.Upsert(&model.WatchHistory{
		UserID:    userID,
		MediaID:   uuid,
		Position:  duration, // 进度拉满
		Duration:  duration,
		Completed: true,
		UpdatedAt: time.Now(),
	})
	h.writeUserData(c, userID, uuid)
}

// MarkUnplayedHandler DELETE /Users/{userId}/PlayedItems/{itemId}
func (h *Handler) MarkUnplayedHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid params"})
		return
	}
	// 直接把进度清零；若仓储有 DeleteHistory 接口，可选用它。
	_ = h.watchRepo.Upsert(&model.WatchHistory{
		UserID:    userID,
		MediaID:   uuid,
		Position:  0,
		Completed: false,
		UpdatedAt: time.Now(),
	})
	h.writeUserData(c, userID, uuid)
}

// writeUserData 统一返回最新的 UserItemData。
func (h *Handler) writeUserData(c *gin.Context, userID, mediaUUID string) {
	m, err := h.mediaRepo.FindByID(mediaUUID)
	if err != nil {
		c.JSON(http.StatusOK, &UserItemData{})
		return
	}
	ud := h.buildUserItemData(userID, m)
	if ud == nil {
		ud = &UserItemData{Key: mediaUUID}
	}
	c.JSON(http.StatusOK, ud)
}
