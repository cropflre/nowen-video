package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// SetFamilyRating supports both explicit review and withdrawal of approval.
// Series approval applies to current episodes only; newly scanned episodes
// remain unreviewed and must be confirmed again by a parent.
func (h *AdminHandler) SetFamilyRating(c *gin.Context) {
	var req struct {
		Level string `json:"level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分级参数无效"})
		return
	}
	valid := map[string]bool{"": true, "G": true, "PG": true, "PG-13": true, "R": true, "NC-17": true}
	if !valid[req.Level] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分级无效"})
		return
	}
	var ids []string
	q := h.db.Model(&model.Media{})
	target := c.Param("mediaId")
	if seriesID := c.Param("seriesId"); seriesID != "" {
		q = q.Where("series_id = ?", seriesID)
		target = seriesID
	} else {
		q = q.Where("id = ?", target)
	}
	if err := q.Pluck("id", &ids).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法读取影片"})
		return
	}
	if len(ids) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "没有可确认的影片"})
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("media_id IN ?", ids).Delete(&model.ContentRating{}).Error; err != nil {
			return err
		}
		if req.Level == "" {
			return nil
		}
		ratings := make([]model.ContentRating, 0, len(ids))
		for _, id := range ids {
			ratings = append(ratings, model.ContentRating{MediaID: id, Level: req.Level})
		}
		return tx.CreateInBatches(ratings, 100).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "未能保存分级"})
		return
	}
	h.auditFromContext(c, "family.rating", "media", target, req.Level)
	c.JSON(http.StatusOK, gin.H{"message": "分级已保存", "count": len(ids)})
}
