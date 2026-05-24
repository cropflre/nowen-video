package service

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// JavDBShortReview is a browser-captured JavDB short review.
type JavDBShortReview struct {
	ID      string  `json:"id"`
	Author  string  `json:"author"`
	Content string  `json:"content"`
	Rating  float64 `json:"rating"`
	Likes   int     `json:"likes"`
}

// DanmakuService stores and lists bullet comments.
type DanmakuService struct {
	danmakuRepo *repository.DanmakuRepo
	mediaRepo   *repository.MediaRepo
	logger      *zap.SugaredLogger
}

func NewDanmakuService(danmakuRepo *repository.DanmakuRepo, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *DanmakuService {
	return &DanmakuService{danmakuRepo: danmakuRepo, mediaRepo: mediaRepo, logger: logger}
}

func (s *DanmakuService) ImportJavDBReviewsForPath(videoPath string, reviews []JavDBShortReview) (mediaID string, imported int, err error) {
	videoPath = filepath.Clean(strings.TrimSpace(videoPath))
	if videoPath == "" {
		return "", 0, fmt.Errorf("视频路径为空")
	}
	if len(reviews) == 0 {
		return "", 0, nil
	}
	media, err := s.mediaRepo.FindByFilePath(videoPath)
	if err != nil || media == nil || media.ID == "" {
		return "", 0, fmt.Errorf("未在媒体库中找到该视频路径，请先扫描入库: %s", videoPath)
	}
	items := buildJavDBDanmaku(media, reviews)
	if err := s.danmakuRepo.UpsertMany(items); err != nil {
		return "", 0, err
	}
	return media.ID, len(items), nil
}

func (s *DanmakuService) ListByMedia(mediaID string, limit int) ([]model.DanmakuComment, error) {
	return s.danmakuRepo.ListByMedia(mediaID, limit)
}

func buildJavDBDanmaku(media *model.Media, reviews []JavDBShortReview) []model.DanmakuComment {
	duration := media.Duration
	if duration <= 0 && media.Runtime > 0 {
		duration = float64(media.Runtime * 60)
	}
	if duration <= 0 {
		duration = 1800
	}
	start := 8.0
	end := duration * 0.82
	if end < start+30 {
		end = start + 30
	}
	if end > 1800 {
		end = 1800
	}
	step := (end - start) / float64(len(reviews)+1)
	if step < 4 {
		step = 4
	}

	now := time.Now()
	items := make([]model.DanmakuComment, 0, len(reviews))
	seen := map[string]struct{}{}
	for i, review := range reviews {
		content := strings.TrimSpace(review.Content)
		if content == "" {
			continue
		}
		sourceID := strings.TrimSpace(review.ID)
		if sourceID == "" {
			sourceID = javdbReviewHash(media.ID, review.Author, content)
		}
		if _, ok := seen[sourceID]; ok {
			continue
		}
		seen[sourceID] = struct{}{}
		items = append(items, model.DanmakuComment{
			MediaID:    media.ID,
			Source:     "javdb",
			SourceID:   sourceID,
			Author:     strings.TrimSpace(review.Author),
			Content:    content,
			Rating:     review.Rating,
			Likes:      review.Likes,
			Position:   start + float64(i+1)*step,
			Color:      colorForJavDBRating(review.Rating),
			Mode:       "scroll",
			ImportedAt: now,
		})
	}
	return items
}

func javdbReviewHash(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return "javdb-" + hex.EncodeToString(h.Sum(nil))[:16]
}

func colorForJavDBRating(rating float64) string {
	switch {
	case rating >= 4.5:
		return "#ffd166"
	case rating >= 3.5:
		return "#ffffff"
	case rating > 0:
		return "#b8c2cc"
	default:
		return "#ffffff"
	}
}
