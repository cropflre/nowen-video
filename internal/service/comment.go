package service

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// CommentService 评论服务
type CommentService struct {
	commentRepo *repository.CommentRepo
	mediaRepo   *repository.MediaRepo
	logger      *zap.SugaredLogger
}

// NewCommentService 创建评论服务
func NewCommentService(commentRepo *repository.CommentRepo, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *CommentService {
	return &CommentService{
		commentRepo: commentRepo,
		mediaRepo:   mediaRepo,
		logger:      logger,
	}
}

// Create 创建评论
func (s *CommentService) Create(userID, mediaID, content string, rating float64) (*model.Comment, error) {
	if _, err := s.mediaRepo.FindByID(mediaID); err != nil {
		return nil, ErrMediaNotFound
	}

	if rating < 0 || rating > 10 {
		return nil, fmt.Errorf("评分范围应在 0-10 之间")
	}

	comment := &model.Comment{
		UserID:  userID,
		MediaID: mediaID,
		Content: content,
		Rating:  rating,
	}

	if err := s.commentRepo.Create(comment); err != nil {
		return nil, err
	}

	s.logger.Debugf("用户 %s 评论媒体 %s: %s (评分: %.1f)", userID, mediaID, content, rating)
	return comment, nil
}

// ListByMedia 获取媒体的评论列表
func (s *CommentService) ListByMedia(mediaID string, page, size int) ([]model.Comment, int64, error) {
	return s.commentRepo.ListByMedia(mediaID, page, size)
}

// Delete 删除评论（自己的评论或管理员删除）
func (s *CommentService) Delete(userID, commentID, role string) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return err
	}
	// 管理员可删除任何评论，普通用户只能删除自己的
	if comment.UserID != userID && role != "admin" {
		return ErrUnauthorized
	}
	return s.commentRepo.Delete(commentID)
}

// Update 更新评论
func (s *CommentService) Update(userID, commentID, content string, rating float64) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return err
	}
	if comment.UserID != userID {
		return ErrUnauthorized
	}
	comment.Content = content
	comment.Rating = rating
	return s.commentRepo.Update(comment)
}

// GetAverageRating 获取媒体的平均评分
func (s *CommentService) GetAverageRating(mediaID string) (float64, int64, error) {
	return s.commentRepo.GetAverageRating(mediaID)
}
