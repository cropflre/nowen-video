package service

import (
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// UserService 用户服务
type UserService struct {
	repo   *repository.UserRepo
	cfg    *config.Config
	logger *zap.SugaredLogger
}

func NewUserService(repo *repository.UserRepo, cfg *config.Config, logger *zap.SugaredLogger) *UserService {
	return &UserService{repo: repo, cfg: cfg, logger: logger}
}

// EnsureAdminExists 确保管理员账号存在（首次启动时）
func (s *UserService) EnsureAdminExists() error {
	count, err := s.repo.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // 已有用户，跳过
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &model.User{
		Username: "admin",
		Password: string(hashedPassword),
		Role:     "admin",
	}

	if err := s.repo.Create(admin); err != nil {
		return err
	}

	s.logger.Info("已创建默认管理员账号: admin / admin123（请尽快修改密码）")
	return nil
}

// GetProfile 获取用户信息
func (s *UserService) GetProfile(userID string) (*model.User, error) {
	return s.repo.FindByID(userID)
}

// ListUsers 获取所有用户
func (s *UserService) ListUsers() ([]model.User, error) {
	return s.repo.List()
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(id string) error {
	return s.repo.Delete(id)
}
