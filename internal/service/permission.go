package service

import (
	"errors"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	db          *gorm.DB
}

// NewPermissionService 创建权限服务
func NewPermissionService(
	permRepo *repository.UserPermissionRepo,
	ratingRepo *repository.ContentRatingRepo,
	historyRepo *repository.WatchHistoryRepo,
	logger *zap.SugaredLogger,
	dbs ...*gorm.DB,
) *PermissionService {
	var db *gorm.DB
	if len(dbs) > 0 {
		db = dbs[0]
	}
	return &PermissionService{db: db,
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
	if errors.Is(err, gorm.ErrRecordNotFound) {
		perm = &model.UserPermission{UserID: userID, MaxRatingLevel: "NC-17"}
	} else if err != nil {
		return nil, err
	}
	if s.db != nil {
		var user model.User
		if err := s.db.Select("id", "child_mode").First(&user, "id = ?", userID).Error; err != nil {
			return nil, err
		}
		perm.ChildMode = user.ChildMode
		if perm.ChildMode && errors.Is(err, gorm.ErrRecordNotFound) {
			perm.MaxRatingLevel = "G"
		}
	}
	return perm, nil
}

// UpdateUserPermission preserves child mode when older clients omit the field.
func (s *PermissionService) UpdateUserPermission(userID, allowedLibraries, maxRating string, dailyLimit int, childMode ...bool) error {
	if _, ok := ratingLevels[maxRating]; !ok || dailyLimit < 0 || dailyLimit > 1440 {
		return ErrInvalidPermission
	}
	current, err := s.GetUserPermission(userID)
	if err != nil {
		return err
	}
	mode := current.ChildMode
	if len(childMode) > 0 {
		mode = childMode[0]
	}
	if mode && ratingLevels[maxRating] > ratingLevels["PG-13"] {
		return ErrInvalidPermission
	}
	perm := &model.UserPermission{UserID: userID, AllowedLibraries: allowedLibraries, MaxRatingLevel: maxRating, DailyTimeLimit: dailyLimit, ChildMode: mode}
	if s.db != nil {
		if err := s.permRepo.UpdateFamilyPolicy(perm); errors.Is(err, repository.ErrInvalidFamilyPolicy) {
			return ErrInvalidPermission
		} else {
			return err
		}
	}
	return s.permRepo.Upsert(perm)
}

// CheckLibraryAccess 检查用户是否有权限访问某个媒体库
func (s *PermissionService) CheckLibraryAccess(userID, libraryID string) bool {
	perm, err := s.GetUserPermission(userID)
	if err != nil {
		return false // Permission lookup failures cannot grant access
	}
	if perm.AllowedLibraries == "" {
		return !perm.ChildMode // Children must have explicitly selected libraries
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
	perm, err := s.GetUserPermission(userID)
	if err != nil {
		return nil, true
	}
	if perm.AllowedLibraries == "" {
		return nil, perm.ChildMode
	}
	for _, id := range strings.Split(perm.AllowedLibraries, ",") {
		if v := strings.TrimSpace(id); v != "" {
			ids = append(ids, v)
		}
	}
	restricted = perm.ChildMode || len(ids) > 0
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
	if s.db != nil {
		catalog := NewChildCatalog(s.db)
		policy, err := catalog.Policy(userID)
		if err != nil {
			return ErrForbidden
		}
		if policy.ChildMode {
			if err := catalog.CheckMedia(policy, mediaID); err != nil {
				return ErrContentRestricted
			}
			if ok, _, _ := s.CheckDailyTimeLimit(userID); !ok {
				return ErrTimeLimitExceeded
			}
			return nil
		}
	}
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
	perm, err := s.GetUserPermission(userID)
	if err != nil {
		return false
	}
	rating, err := s.ratingRepo.FindByMediaID(mediaID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return !perm.ChildMode
	}
	if err != nil {
		return false
	}
	userLevel, ok1 := ratingLevels[perm.MaxRatingLevel]
	contentLevel, ok2 := ratingLevels[rating.Level]
	if !ok1 || !ok2 {
		return !perm.ChildMode
	}
	return contentLevel <= userLevel
}

// CheckDailyTimeLimit 检查用户今日观看时长是否超限
func (s *PermissionService) CheckDailyTimeLimit(userID string) (bool, int, int) {
	perm, err := s.GetUserPermission(userID)
	if err != nil {
		return false, 0, 0
	}
	if perm.DailyTimeLimit <= 0 {
		return true, 0, 0
	}

	// 计算今日已观看时长
	todayMinutes, err := s.getTodayWatchMinutes(userID)
	if err != nil {
		return false, 0, 0
	}
	remaining := perm.DailyTimeLimit - todayMinutes
	if remaining < 0 {
		remaining = 0
	}

	return todayMinutes < perm.DailyTimeLimit, todayMinutes, remaining
}

// getTodayWatchMinutes 获取今日已观看分钟数
func (s *PermissionService) getTodayWatchMinutes(userID string) (int, error) {
	histories, err := s.historyRepo.GetAllByUserID(userID)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var totalSeconds float64
	for _, h := range histories {
		if h.UpdatedAt.After(today) {
			totalSeconds += h.Position
		}
	}
	return int(totalSeconds / 60), nil
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
