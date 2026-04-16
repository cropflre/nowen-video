package service

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// AuthService 认证服务
type AuthService struct {
	userRepo *repository.UserRepo
	cfg      *config.Config
	logger   *zap.SugaredLogger
}

func NewAuthService(userRepo *repository.UserRepo, cfg *config.Config, logger *zap.SugaredLogger) *AuthService {
	return &AuthService{userRepo: userRepo, cfg: cfg, logger: logger}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username   string `json:"username" binding:"required,min=3,max=32"`
	Password   string `json:"password" binding:"required,min=6,max=64"`
	InviteCode string `json:"invite_code"` // 邀请码（可选，根据配置决定是否必填）
}

// TokenResponse 令牌响应
type TokenResponse struct {
	Token     string      `json:"token"`
	ExpiresAt int64       `json:"expires_at"`
	User      *model.User `json:"user"`
}

// Login 用户登录
func (s *AuthService) Login(req *LoginRequest) (*TokenResponse, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.generateToken(user)
}

// InitStatus 系统初始化状态
type InitStatus struct {
	Initialized      bool `json:"initialized"`       // 是否已有用户
	RegistrationOpen bool `json:"registration_open"` // 是否允许注册
}

// GetInitStatus 获取系统初始化状态（公开接口，无需认证）
func (s *AuthService) GetInitStatus() (*InitStatus, error) {
	count, err := s.userRepo.Count()
	if err != nil {
		return nil, err
	}
	return &InitStatus{
		Initialized:      count > 0,
		RegistrationOpen: count == 0 || s.cfg.Registration.Enabled,
	}, nil
}

// Register 用户注册
func (s *AuthService) Register(req *RegisterRequest) (*TokenResponse, error) {
	// 检查是否为第一个用户（第一个用户始终允许注册为管理员）
	count, _ := s.userRepo.Count()
	isFirstUser := count == 0

	// 非第一个用户时，检查注册限制
	if !isFirstUser {
		if !s.cfg.Registration.Enabled {
			return nil, ErrRegistrationDisabled
		}
		// 检查邀请码
		if s.cfg.Registration.InviteCode != "" {
			if req.InviteCode != s.cfg.Registration.InviteCode {
				return nil, ErrInvalidInviteCode
			}
		}
	}

	// 检查用户名是否已存在
	if _, err := s.userRepo.FindByUsername(req.Username); err == nil {
		return nil, ErrUserExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 判断是否为第一个用户（自动设为管理员）
	role := "user"
	if isFirstUser {
		role = "admin"
	}

	user := &model.User{
		Username: req.Username,
		Password: string(hashedPassword),
		Role:     role,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	s.logger.Infof("新用户注册: %s (角色: %s)", user.Username, user.Role)
	return s.generateToken(user)
}

// RefreshToken 刷新令牌
func (s *AuthService) RefreshToken(userID string) (*TokenResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return s.generateToken(user)
}

// generateToken 生成JWT令牌
func (s *AuthService) generateToken(user *model.User) (*TokenResponse, error) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7天过期

	claims := &middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.Secrets.JWTSecret))
	if err != nil {
		return nil, err
	}

	return &TokenResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt.Unix(),
		User:      user,
	}, nil
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=6"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64"`
}

// ChangePassword 修改密码（用户自行修改，需验证旧密码）
func (s *AuthService) ChangePassword(userID string, req *ChangePasswordRequest) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return ErrUserNotFound
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return ErrInvalidCredentials
	}

	// 生成新密码哈希
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// 更新密码
	if err := s.userRepo.UpdatePassword(userID, string(hashedPassword)); err != nil {
		return err
	}

	s.logger.Infof("用户 %s 修改了密码", user.Username)
	return nil
}

// ResetPassword 管理员重置用户密码（无需旧密码）
func (s *AuthService) ResetPassword(userID string, newPassword string) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return ErrUserNotFound
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(userID, string(hashedPassword)); err != nil {
		return err
	}

	s.logger.Infof("管理员重置了用户 %s 的密码", user.Username)
	return nil
}
