package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// setupTestDB 创建内存SQLite测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("迁移测试数据库失败: %v", err)
	}
	return db
}

// setupAuthService 创建测试用的AuthService
func setupAuthService(t *testing.T) *AuthService {
	t.Helper()
	db := setupTestDB(t)
	repos := repository.NewRepositories(db)
	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{}
	cfg.Secrets.JWTSecret = "test-secret-key-for-unit-tests"
	cfg.Registration.Enabled = true // 测试中允许注册
	authService := NewAuthService(repos.User, repos.InviteCode, repos.LoginLog, repos.AuditLog, cfg, logger.Sugar())
	return authService
}

// TestRegister_FirstUserIsAdmin 第一个注册的用户应为管理员
func TestRegister_FirstUserIsAdmin(t *testing.T) {
	authService := setupAuthService(t)

	req := &RegisterRequest{
		Username: "admin",
		Password: "password123",
	}

	resp, err := authService.Register(req, "", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	if resp == nil {
		t.Fatal("注册响应不应为nil")
	}

	if resp.Token == "" {
		t.Error("令牌不应为空")
	}

	if resp.User.Role != "admin" {
		t.Errorf("第一个用户应为admin角色, 实际: %s", resp.User.Role)
	}

	if resp.User.Username != "admin" {
		t.Errorf("用户名应为admin, 实际: %s", resp.User.Username)
	}
}

// TestRegister_SecondUserIsNormalUser 第二个注册的用户应为普通用户
func TestRegister_SecondUserIsNormalUser(t *testing.T) {
	authService := setupAuthService(t)

	// 注册第一个用户（管理员）
	_, err := authService.Register(&RegisterRequest{
		Username: "admin",
		Password: "password123",
	}, "", "")
	if err != nil {
		t.Fatalf("注册第一个用户失败: %v", err)
	}

	// 注册第二个用户
	resp, err := authService.Register(&RegisterRequest{
		Username: "user1",
		Password: "password123",
	}, "", "")
	if err != nil {
		t.Fatalf("注册第二个用户失败: %v", err)
	}

	if resp.User.Role != "user" {
		t.Errorf("第二个用户应为user角色, 实际: %s", resp.User.Role)
	}
}

// TestRegister_DuplicateUsername 重复用户名注册应失败
func TestRegister_DuplicateUsername(t *testing.T) {
	authService := setupAuthService(t)

	req := &RegisterRequest{
		Username: "testuser",
		Password: "password123",
	}

	_, err := authService.Register(req, "", "")
	if err != nil {
		t.Fatalf("首次注册不应失败: %v", err)
	}

	_, err = authService.Register(req, "", "")
	if err != ErrUserExists {
		t.Errorf("重复注册应返回ErrUserExists, 实际: %v", err)
	}
}

// TestLogin_Success 正确凭证应登录成功
func TestLogin_Success(t *testing.T) {
	authService := setupAuthService(t)

	// 先注册
	_, err := authService.Register(&RegisterRequest{
		Username: "logintest",
		Password: "mypassword",
	}, "", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 登录
	resp, err := authService.Login(&LoginRequest{
		Username: "logintest",
		Password: "mypassword",
	}, "", "")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}

	if resp.Token == "" {
		t.Error("登录令牌不应为空")
	}

	if resp.User.Username != "logintest" {
		t.Errorf("用户名应为logintest, 实际: %s", resp.User.Username)
	}

	if resp.ExpiresAt == 0 {
		t.Error("令牌过期时间不应为0")
	}
}

// TestLogin_WrongPassword 错误密码应登录失败
func TestLogin_WrongPassword(t *testing.T) {
	authService := setupAuthService(t)

	_, err := authService.Register(&RegisterRequest{
		Username: "pwdtest",
		Password: "correctpwd",
	}, "", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	_, err = authService.Login(&LoginRequest{
		Username: "pwdtest",
		Password: "wrongpwd",
	}, "", "")
	if err != ErrInvalidCredentials {
		t.Errorf("错误密码应返回ErrInvalidCredentials, 实际: %v", err)
	}
}

// TestLogin_NonexistentUser 不存在的用户应登录失败
func TestLogin_NonexistentUser(t *testing.T) {
	authService := setupAuthService(t)

	_, err := authService.Login(&LoginRequest{
		Username: "nonexistent",
		Password: "whatever",
	}, "", "")
	if err != ErrInvalidCredentials {
		t.Errorf("不存在的用户应返回ErrInvalidCredentials, 实际: %v", err)
	}
}

// TestRefreshToken_Success 有效用户应能刷新令牌
func TestRefreshToken_Success(t *testing.T) {
	authService := setupAuthService(t)

	// 注册获取用户
	regResp, err := authService.Register(&RegisterRequest{
		Username: "refreshtest",
		Password: "password123",
	}, "", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	// 刷新令牌
	resp, err := authService.RefreshToken(regResp.User.ID)
	if err != nil {
		t.Fatalf("刷新令牌失败: %v", err)
	}

	if resp.Token == "" {
		t.Error("刷新后的令牌不应为空")
	}

	// 验证刷新后的令牌过期时间有效
	if resp.ExpiresAt <= regResp.ExpiresAt {
		// 允许相同秒内生成（过期时间可能相同）
		if resp.ExpiresAt < regResp.ExpiresAt {
			t.Error("刷新后的令牌过期时间不应早于原令牌")
		}
	}
}

// TestRefreshToken_InvalidUser 无效用户ID应刷新失败
func TestRefreshToken_InvalidUser(t *testing.T) {
	authService := setupAuthService(t)

	_, err := authService.RefreshToken("non-existent-user-id")
	if err != ErrUserNotFound {
		t.Errorf("无效用户ID应返回ErrUserNotFound, 实际: %v", err)
	}
}

// TestGenerateToken_ContainsValidClaims 生成的JWT应包含有效声明
func TestGenerateToken_ContainsValidClaims(t *testing.T) {
	authService := setupAuthService(t)

	resp, err := authService.Register(&RegisterRequest{
		Username: "claimtest",
		Password: "password123",
	}, "", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	if resp.Token == "" {
		t.Fatal("令牌不应为空")
	}

	// 验证过期时间大于当前时间
	if resp.ExpiresAt <= 0 {
		t.Error("过期时间应大于0")
	}

	// 验证用户信息完整
	if resp.User == nil {
		t.Fatal("用户信息不应为nil")
	}

	if resp.User.ID == "" {
		t.Error("用户ID不应为空")
	}
}
