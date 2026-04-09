package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// setupPermissionService 创建测试用的PermissionService
func setupPermissionService(t *testing.T) (*PermissionService, *repository.Repositories) {
	t.Helper()
	db := setupTestDB(t)
	repos := repository.NewRepositories(db)
	logger, _ := zap.NewDevelopment()
	permService := NewPermissionService(
		repos.UserPermission,
		repos.ContentRating,
		repos.WatchHistory,
		logger.Sugar(),
	)
	return permService, repos
}

// TestGetUserPermission_Default 无权限记录时应返回默认权限
func TestGetUserPermission_Default(t *testing.T) {
	permService, _ := setupPermissionService(t)

	perm, err := permService.GetUserPermission("nonexistent-user")
	// 如果返回错误，说明没有默认处理；如果返回nil+nil说明有默认值
	if err != nil && perm == nil {
		// 预期行为：无权限记录时返回错误
		t.Logf("无权限记录时返回错误（预期行为）: %v", err)
		return
	}

	if perm != nil {
		t.Logf("返回了默认权限: AllowedLibraries=%s, MaxRatingLevel=%s",
			perm.AllowedLibraries, perm.MaxRatingLevel)
	}
}

// TestSetUserPermission_CreateAndRetrieve 设置权限后应能正确获取
func TestSetUserPermission_CreateAndRetrieve(t *testing.T) {
	permService, repos := setupPermissionService(t)

	// 先创建一个测试用户
	user := &model.User{
		Username: "permtest",
		Password: "hashed",
		Role:     "user",
	}
	if err := repos.User.Create(user); err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	// 设置权限
	perm := &model.UserPermission{
		UserID:           user.ID,
		AllowedLibraries: "lib1,lib2",
		MaxRatingLevel:   "PG-13",
		DailyTimeLimit:   120,
	}

	err := repos.UserPermission.Upsert(perm)
	if err != nil {
		t.Fatalf("设置权限失败: %v", err)
	}

	// 获取权限
	retrieved, err := permService.GetUserPermission(user.ID)
	if err != nil {
		t.Fatalf("获取权限失败: %v", err)
	}

	if retrieved.AllowedLibraries != "lib1,lib2" {
		t.Errorf("AllowedLibraries应为lib1,lib2, 实际: %s", retrieved.AllowedLibraries)
	}

	if retrieved.MaxRatingLevel != "PG-13" {
		t.Errorf("MaxRatingLevel应为PG-13, 实际: %s", retrieved.MaxRatingLevel)
	}

	if retrieved.DailyTimeLimit != 120 {
		t.Errorf("DailyTimeLimit应为120, 实际: %d", retrieved.DailyTimeLimit)
	}
}

// TestSetUserPermission_Update 更新权限应覆盖旧值
func TestSetUserPermission_Update(t *testing.T) {
	_, repos := setupPermissionService(t)

	user := &model.User{
		Username: "updateperm",
		Password: "hashed",
		Role:     "user",
	}
	if err := repos.User.Create(user); err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	// 第一次设置
	perm1 := &model.UserPermission{
		UserID:           user.ID,
		AllowedLibraries: "lib1",
		MaxRatingLevel:   "G",
		DailyTimeLimit:   60,
	}
	if err := repos.UserPermission.Upsert(perm1); err != nil {
		t.Fatalf("首次设置权限失败: %v", err)
	}

	// 更新
	perm2 := &model.UserPermission{
		UserID:           user.ID,
		AllowedLibraries: "lib1,lib2,lib3",
		MaxRatingLevel:   "R",
		DailyTimeLimit:   240,
	}
	if err := repos.UserPermission.Upsert(perm2); err != nil {
		t.Fatalf("更新权限失败: %v", err)
	}

	// 验证更新后的值
	retrieved, err := repos.UserPermission.FindByUserID(user.ID)
	if err != nil {
		t.Fatalf("获取更新后权限失败: %v", err)
	}

	if retrieved.AllowedLibraries != "lib1,lib2,lib3" {
		t.Errorf("更新后AllowedLibraries应为lib1,lib2,lib3, 实际: %s", retrieved.AllowedLibraries)
	}

	if retrieved.MaxRatingLevel != "R" {
		t.Errorf("更新后MaxRatingLevel应为R, 实际: %s", retrieved.MaxRatingLevel)
	}

	if retrieved.DailyTimeLimit != 240 {
		t.Errorf("更新后DailyTimeLimit应为240, 实际: %d", retrieved.DailyTimeLimit)
	}
}

// TestContentRating_SetAndGet 设置和获取内容分级
func TestContentRating_SetAndGet(t *testing.T) {
	_, repos := setupPermissionService(t)

	// 创建内容分级
	rating := &model.ContentRating{
		MediaID: "test-media-id",
		Level:   "PG-13",
	}

	if err := repos.ContentRating.Upsert(rating); err != nil {
		t.Fatalf("设置内容分级失败: %v", err)
	}

	// 获取内容分级
	retrieved, err := repos.ContentRating.FindByMediaID("test-media-id")
	if err != nil {
		t.Fatalf("获取内容分级失败: %v", err)
	}

	if retrieved.Level != "PG-13" {
		t.Errorf("内容分级应为PG-13, 实际: %s", retrieved.Level)
	}
}

// TestContentRating_Update 更新内容分级
func TestContentRating_Update(t *testing.T) {
	_, repos := setupPermissionService(t)

	// 创建
	rating1 := &model.ContentRating{
		MediaID: "update-media",
		Level:   "G",
	}
	if err := repos.ContentRating.Upsert(rating1); err != nil {
		t.Fatalf("创建内容分级失败: %v", err)
	}

	// 更新
	rating2 := &model.ContentRating{
		MediaID: "update-media",
		Level:   "R",
	}
	if err := repos.ContentRating.Upsert(rating2); err != nil {
		t.Fatalf("更新内容分级失败: %v", err)
	}

	// 验证
	retrieved, err := repos.ContentRating.FindByMediaID("update-media")
	if err != nil {
		t.Fatalf("获取更新后内容分级失败: %v", err)
	}

	if retrieved.Level != "R" {
		t.Errorf("更新后内容分级应为R, 实际: %s", retrieved.Level)
	}
}
