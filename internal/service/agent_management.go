package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// Management starts with read-only inspection. File operations and task execution
// require their own durable preview/approval contract and are not exposed here.
type AgentLibrarySummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MediaCount int64  `json:"media_count"`
}
type AgentManagementSummary struct {
	LibraryCount  int64                 `json:"library_count"`
	MediaCount    int64                 `json:"media_count"`
	NeedsMetadata int64                 `json:"needs_metadata"`
	NeedsRating   int64                 `json:"needs_rating"`
	Libraries     []AgentLibrarySummary `json:"libraries"`
}

func (s *ViewerAgent) inspectManagement(uid, message string, out *ViewerAgentResponse) error {
	var user model.User
	if err := s.db.Select("id", "role", "child_mode").First(&user, "id = ?", uid).Error; err != nil {
		return err
	}
	if user.Role != "admin" || user.ChildMode {
		return ErrForbidden
	}
	if message == "检查任务" || message == "打开任务中心" {
		out.Action = &AgentAction{Kind: "inspect_tasks"}
		return nil
	}
	summary := &AgentManagementSummary{Libraries: []AgentLibrarySummary{}}
	libs := s.db.Model(&model.Library{})
	if err := libs.Count(&summary.LibraryCount).Error; err != nil {
		return err
	}
	media := s.db.Model(&model.Media{}).Where("library_id IN (?)", s.db.Model(&model.Library{}).Select("id"))
	if err := media.Count(&summary.MediaCount).Error; err != nil {
		return err
	}
	if err := media.Session(&gorm.Session{}).Where("scrape_status IN ?", []string{"", "pending", "partial", "failed"}).Count(&summary.NeedsMetadata).Error; err != nil {
		return err
	}
	if err := media.Session(&gorm.Session{}).Where("id NOT IN (?)", s.db.Model(&model.ContentRating{}).Select("media_id").Where("level IN ?", []string{"G", "PG", "PG-13", "R", "NC-17"})).Count(&summary.NeedsRating).Error; err != nil {
		return err
	}
	if err := s.db.Table("libraries").Select("libraries.id, libraries.name, COUNT(media.id) AS media_count").
		Joins("LEFT JOIN media ON media.library_id = libraries.id AND media.deleted_at IS NULL").Where("libraries.deleted_at IS NULL").Group("libraries.id,libraries.name").Order("libraries.name ASC,libraries.id ASC").Limit(20).Scan(&summary.Libraries).Error; err != nil {
		return err
	}
	out.Management = summary
	out.Message = "影视库检查已完成。资料待补表示尚未完整匹配，分级待审核的影片不会自动向儿童开放。"
	return nil
}
