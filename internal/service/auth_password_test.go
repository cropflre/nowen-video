package service

import "testing"

// TestChangePassword_PersistsNewPassword 回归首次登录改密：
// 新密码必须能够登录，旧密码必须立即失效，强制改密标记必须清除。
func TestChangePassword_PersistsNewPassword(t *testing.T) {
	authService := setupAuthService(t)

	registered, err := authService.Register(&RegisterRequest{
		Username: "admin",
		Password: "admin123",
	}, "", "")
	if err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}

	if err := authService.userRepo.UpdateFields(registered.User.ID, map[string]interface{}{
		"must_change_pwd": true,
	}); err != nil {
		t.Fatalf("设置强制改密标记失败: %v", err)
	}

	if err := authService.ChangePassword(registered.User.ID, &ChangePasswordRequest{
		OldPassword: "admin123",
		NewPassword: "new-password-123",
	}); err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}

	if _, err := authService.Login(&LoginRequest{
		Username: "admin",
		Password: "admin123",
	}, "", ""); err != ErrInvalidCredentials {
		t.Fatalf("旧密码应立即失效，实际错误: %v", err)
	}

	loginResp, err := authService.Login(&LoginRequest{
		Username: "admin",
		Password: "new-password-123",
	}, "", "")
	if err != nil {
		t.Fatalf("新密码应能登录: %v", err)
	}
	if loginResp.MustChangePassword {
		t.Fatal("修改密码后不应继续要求强制改密")
	}

	stored, err := authService.userRepo.FindByID(registered.User.ID)
	if err != nil {
		t.Fatalf("重新读取用户失败: %v", err)
	}
	if stored.MustChangePwd {
		t.Fatal("数据库中的 must_change_pwd 应已清除")
	}
	if stored.TokenVersion != 1 {
		t.Fatalf("修改密码后 token_version 应自增为 1，实际为 %d", stored.TokenVersion)
	}
}
