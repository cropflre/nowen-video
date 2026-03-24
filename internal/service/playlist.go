package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// PlaylistService 播放列表服务
type PlaylistService struct {
	repo   *repository.PlaylistRepo
	logger *zap.SugaredLogger
}

func NewPlaylistService(repo *repository.PlaylistRepo, logger *zap.SugaredLogger) *PlaylistService {
	return &PlaylistService{repo: repo, logger: logger}
}

// Create 创建播放列表
func (s *PlaylistService) Create(userID, name string) (*model.Playlist, error) {
	playlist := &model.Playlist{
		UserID: userID,
		Name:   name,
	}
	if err := s.repo.Create(playlist); err != nil {
		return nil, err
	}
	return playlist, nil
}

// List 获取用户的所有播放列表
func (s *PlaylistService) List(userID string) ([]model.Playlist, error) {
	return s.repo.ListByUserID(userID)
}

// Get 获取播放列表详情
func (s *PlaylistService) Get(id string) (*model.Playlist, error) {
	return s.repo.FindByID(id)
}

// Delete 删除播放列表
func (s *PlaylistService) Delete(id, userID string) error {
	playlist, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if playlist.UserID != userID {
		return ErrForbidden
	}
	return s.repo.Delete(id)
}

// AddItem 添加媒体到播放列表
func (s *PlaylistService) AddItem(playlistID, mediaID, userID string) error {
	playlist, err := s.repo.FindByID(playlistID)
	if err != nil {
		return err
	}
	if playlist.UserID != userID {
		return ErrForbidden
	}

	maxOrder := s.repo.GetMaxSortOrder(playlistID)
	item := &model.PlaylistItem{
		PlaylistID: playlistID,
		MediaID:    mediaID,
		SortOrder:  maxOrder + 1,
	}
	return s.repo.AddItem(item)
}

// RemoveItem 从播放列表移除媒体
func (s *PlaylistService) RemoveItem(playlistID, mediaID, userID string) error {
	playlist, err := s.repo.FindByID(playlistID)
	if err != nil {
		return err
	}
	if playlist.UserID != userID {
		return ErrForbidden
	}
	return s.repo.RemoveItem(playlistID, mediaID)
}
