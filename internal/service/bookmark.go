package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// BookmarkService 视频书签服务
type BookmarkService struct {
	bookmarkRepo *repository.BookmarkRepo
	mediaRepo    *repository.MediaRepo
	logger       *zap.SugaredLogger
}

// NewBookmarkService 创建书签服务
func NewBookmarkService(bookmarkRepo *repository.BookmarkRepo, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *BookmarkService {
	return &BookmarkService{
		bookmarkRepo: bookmarkRepo,
		mediaRepo:    mediaRepo,
		logger:       logger,
	}
}

// Create 添加书签
func (s *BookmarkService) Create(userID, mediaID, title, note string, position float64) (*model.Bookmark, error) {
	// 验证媒体存在
	if _, err := s.mediaRepo.FindByID(mediaID); err != nil {
		return nil, ErrMediaNotFound
	}

	bookmark := &model.Bookmark{
		UserID:   userID,
		MediaID:  mediaID,
		Position: position,
		Title:    title,
		Note:     note,
	}

	if err := s.bookmarkRepo.Create(bookmark); err != nil {
		return nil, err
	}

	s.logger.Debugf("用户 %s 为媒体 %s 添加书签: %.1fs - %s", userID, mediaID, position, title)
	return bookmark, nil
}

// ListByMedia 获取某个媒体的所有书签
func (s *BookmarkService) ListByMedia(userID, mediaID string) ([]model.Bookmark, error) {
	return s.bookmarkRepo.ListByUserAndMedia(userID, mediaID)
}

// ListByUser 获取用户所有书签（分页）
func (s *BookmarkService) ListByUser(userID string, page, size int) ([]model.Bookmark, int64, error) {
	return s.bookmarkRepo.ListByUser(userID, page, size)
}

// Delete 删除书签
func (s *BookmarkService) Delete(userID, bookmarkID string) error {
	bookmark, err := s.bookmarkRepo.FindByID(bookmarkID)
	if err != nil {
		return err
	}
	// 验证归属
	if bookmark.UserID != userID {
		return ErrUnauthorized
	}
	return s.bookmarkRepo.Delete(bookmarkID)
}

// Update 更新书签
func (s *BookmarkService) Update(userID, bookmarkID, title, note string) error {
	bookmark, err := s.bookmarkRepo.FindByID(bookmarkID)
	if err != nil {
		return err
	}
	if bookmark.UserID != userID {
		return ErrUnauthorized
	}
	bookmark.Title = title
	bookmark.Note = note
	return s.bookmarkRepo.Update(bookmark)
}
