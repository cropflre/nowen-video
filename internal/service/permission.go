package service

import (
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// 内容分级等级（从低到高）
var ratingLevels = map[string]int{
	"G":     1, // 所有年龄适宜
	"PG":    2, // 家长指导
	"PG-13": 3, // 13岁以上
	"R":     4, // 限制级
	"NC-17": 5, // 17岁以下禁止
}

// PermissionService 权限管理服务
type PermissionService struct {
	permRepo    *repository.UserPermissionRepo
	ratingRepo  *repository.ContentRatingRepo
	historyRepo *repository.WatchHistoryRepo
	logger      *zap.SugaredLogger
}

// NewPermissionService 创建权限服务
func NewPermissionService(
	permRepo *repository.UserPermissionRepo,
	ratingRepo *repository.ContentRatingRepo,
	historyRepo *repository.WatchHistoryRepo,
	logger *zap.SugaredLogger,
) *PermissionService {
	return &PermissionService{
		permRepo:    permRepo,
		ratingRepo:  ratingRepo,
		historyRepo: historyRepo,
		logger:      logger,
	}
}

// ==================== 用户权限管理 ====================

// GetUserPermission 获取用户权限设置
func (s *PermissionService) GetUserPermission(userID string) (*model.UserPermission, error) {
	perm, err := s.permRepo.FindByUserID(userID)
	if err != nil {
		// 返回默认权限
		return &model.UserPermission{
			UserID:           userID,
			AllowedLibraries: "",      // 空表示全部允许
			MaxRatingLevel:   "NC-17", // 默认不限制
			DailyTimeLimit:   0,       // 默认不限制
		}, nil
	}
	return perm, nil
}

// UpdateUserPermission 更新用户权限
func (s *PermissionService) UpdateUserPermission(userID, allowedLibraries, maxRating string, dailyLimit int) error {
	perm := &model.UserPermission{
		UserID:           userID,
		AllowedLibraries: allowedLibraries,
		MaxRatingLevel:   maxRating,
		DailyTimeLimit:   dailyLimit,
	}
	return s.permRepo.Upsert(perm)
}

// CheckLibraryAccess 检查用户是否有权限访问某个媒体库
func (s *PermissionService) CheckLibraryAccess(userID, libraryID string) bool {
	perm, err := s.permRepo.FindByUserID(userID)
	if err != nil {
		return true // 无权限记录默认允许
	}
	if perm.AllowedLibraries == "" {
		return true // 空表示全部允许
	}
	allowed := strings.Split(perm.AllowedLibraries, ",")
	for _, id := range allowed {
		if strings.TrimSpace(id) == libraryID {
			return true
		}
	}
	return false
}

// GetAllowedLibraryIDs 返回用户允许访问的库 ID 列表；
// 第二个返回值 restricted=true 表示"有限制"（即非空），false 表示无限制（空=全部）
func (s *PermissionService) GetAllowedLibraryIDs(userID string) (ids []string, restricted bool) {
	perm, err := s.permRepo.FindByUserID(userID)
	if err != nil || perm.AllowedLibraries == "" {
		return nil, false
	}
	for _, id := range strings.Split(perm.AllowedLibraries, ",") {
		if v := strings.TrimSpace(id); v != "" {
			ids = append(ids, v)
		}
	}
	restricted = len(ids) > 0
	return
}

// FilterLibraries 过滤用户可访问的媒体库列表
func (s *PermissionService) FilterLibraries(userID string, libs []model.Library) []model.Library {
	ids, restricted := s.GetAllowedLibraryIDs(userID)
	if !restricted {
		return libs
	}
	allowMap := make(map[string]bool, len(ids))
	for _, id := range ids {
		allowMap[id] = true
	}
	filtered := make([]model.Library, 0, len(libs))
	for _, lib := range libs {
		if allowMap[lib.ID] {
			filtered = append(filtered, lib)
		}
	}
	return filtered
}

// CheckMediaAccess 综合校验：用户 -> 媒体（library + rating 双重校验）
// 需要 mediaRepo 查询媒体的 library_id；为避免循环依赖，使用回调
type MediaLibraryLookup func(mediaID string) (libraryID string, err error)

func (s *PermissionService) CheckMediaAccess(userID, mediaID string, lookup MediaLibraryLookup) error {
	if lookup != nil {
		if libID, err := lookup(mediaID); err == nil && libID != "" {
			if !s.CheckLibraryAccess(userID, libID) {
				return ErrForbidden
			}
		}
	}
	if !s.CheckContentRating(userID, mediaID) {
		return ErrContentRestricted
	}
	if ok, _, _ := s.CheckDailyTimeLimit(userID); !ok {
		return ErrTimeLimitExceeded
	}
	return nil
}

// CheckContentRating 检查用户是否可以观看该分级内容
func (s *PermissionService) CheckContentRating(userID, mediaID string) bool {
	// 获取内容分级
	rating, err := s.ratingRepo.FindByMediaID(mediaID)
	if err != nil {
		return true // 无分级标记默认允许
	}

	// 获取用户最高允许分级
	perm, err := s.permRepo.FindByUserID(userID)
	if err != nil {
		return true // 无权限记录默认允许
	}

	userLevel, ok1 := ratingLevels[perm.MaxRatingLevel]
	contentLevel, ok2 := ratingLevels[rating.Level]

	if !ok1 || !ok2 {
		return true
	}

	return contentLevel <= userLevel
}

// CheckDailyTimeLimit 检查用户今日观看时长是否超限
func (s *PermissionService) CheckDailyTimeLimit(userID string) (bool, int, int) {
	perm, err := s.permRepo.FindByUserID(userID)
	if err != nil || perm.DailyTimeLimit <= 0 {
		return true, 0, 0 // 无限制
	}

	// 计算今日已观看时长
	todayMinutes := s.getTodayWatchMinutes(userID)
	remaining := perm.DailyTimeLimit - todayMinutes
	if remaining < 0 {
		remaining = 0
	}

	return todayMinutes < perm.DailyTimeLimit, todayMinutes, remaining
}

// getTodayWatchMinutes 获取今日已观看分钟数
func (s *PermissionService) getTodayWatchMinutes(userID string) int {
	histories, err := s.historyRepo.GetAllByUserID(userID)
	if err != nil {
		return 0
	}

	today := time.Now().Truncate(24 * time.Hour)
	var totalSeconds float64
	for _, h := range histories {
		if h.UpdatedAt.After(today) {
			totalSeconds += h.Position
		}
	}
	return int(totalSeconds / 60)
}

// ==================== 内容分级管理 ====================

// SetContentRating 设置媒体内容分级
func (s *PermissionService) SetContentRating(mediaID, level string) error {
	if _, ok := ratingLevels[level]; !ok {
		return ErrInvalidRating
	}
	return s.ratingRepo.Upsert(&model.ContentRating{
		MediaID: mediaID,
		Level:   level,
	})
}

// GetContentRating 获取媒体内容分级
func (s *PermissionService) GetContentRating(mediaID string) (string, error) {
	rating, err := s.ratingRepo.FindByMediaID(mediaID)
	if err != nil {
		return "", err
	}
	return rating.Level, nil
}
