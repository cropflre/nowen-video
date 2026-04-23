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
	userRepo   *repository.UserRepo
	inviteRepo *repository.InviteCodeRepo
	logRepo    *repository.LoginLogRepo
	auditRepo  *repository.AuditLogRepo
	cfg        *config.Config
	logger     *zap.SugaredLogger
}

func NewAuthService(userRepo *repository.UserRepo, inviteRepo *repository.InviteCodeRepo, logRepo *repository.LoginLogRepo, auditRepo *repository.AuditLogRepo, cfg *config.Config, logger *zap.SugaredLogger) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		inviteRepo: inviteRepo,
		logRepo:    logRepo,
		auditRepo:  auditRepo,
		cfg:        cfg,
		logger:     logger,
	}
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
	// 附加：是否必须立即修改密码（如默认 admin 首次登录）
	MustChangePassword bool `json:"must_change_password,omitempty"`
}

// recordLogin 记录登录日志（尽量不影响主流程）
func (s *AuthService) recordLogin(userID, username, ip, ua, reason string, success bool) {
	if s.logRepo == nil {
		return
	}
	_ = s.logRepo.Create(&model.LoginLog{
		UserID:    userID,
		Username:  username,
		IP:        ip,
		UserAgent: ua,
		Success:   success,
		Reason:    reason,
	})
}

// Login 用户登录
func (s *AuthService) Login(req *LoginRequest, ip, ua string) (*TokenResponse, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		s.recordLogin("", req.Username, ip, ua, "user_not_found", false)
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.recordLogin(user.ID, user.Username, ip, ua, "password_error", false)
		return nil, ErrInvalidCredentials
	}

	if user.Disabled {
		s.recordLogin(user.ID, user.Username, ip, ua, "user_disabled", false)
		return nil, ErrUserDisabled
	}

	// 更新最后登录信息
	_ = s.userRepo.UpdateLastLogin(user.ID, ip)
	s.recordLogin(user.ID, user.Username, ip, ua, "", true)

	resp, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}
	if user.MustChangePwd {
		resp.MustChangePassword = true
	}
	return resp, nil
}

// InitStatus 系统初始化状态
type InitStatus struct {
	Initialized      bool `json:"initialized"`       // 是否已有用户
	RegistrationOpen bool `json:"registration_open"` // 是否允许注册
	InviteRequired   bool `json:"invite_required"`   // 是否要求邀请码
}

// GetInitStatus 获取系统初始化状态（公开接口，无需认证）
func (s *AuthService) GetInitStatus() (*InitStatus, error) {
	count, err := s.userRepo.Count()
	if err != nil {
		return nil, err
	}
	// 判断是否有效邀请码要求：配置中有单一 invite_code 或存在 InviteCode 表记录
	inviteRequired := s.cfg.Registration.InviteCode != ""
	if !inviteRequired && s.inviteRepo != nil {
		if codes, _ := s.inviteRepo.List(); len(codes) > 0 {
			inviteRequired = true
		}
	}
	return &InitStatus{
		Initialized:      count > 0,
		RegistrationOpen: count == 0 || s.cfg.Registration.Enabled,
		InviteRequired:   count > 0 && inviteRequired,
	}, nil
}

// Register 用户注册
func (s *AuthService) Register(req *RegisterRequest, ip, ua string) (*TokenResponse, error) {
	// 检查是否为第一个用户（第一个用户始终允许注册为管理员）
	count, _ := s.userRepo.Count()
	isFirstUser := count == 0

	var consumedInvite *model.InviteCode

	// 非第一个用户时，检查注册限制
	if !isFirstUser {
		if !s.cfg.Registration.Enabled {
			return nil, ErrRegistrationDisabled
		}
		// 检查邀请码：优先使用数据库中的一码一用表，其次再回退到配置中的全局邀请码
		needInvite := s.cfg.Registration.InviteCode != ""
		var dbCodes []model.InviteCode
		if s.inviteRepo != nil {
			dbCodes, _ = s.inviteRepo.List()
			if len(dbCodes) > 0 {
				needInvite = true
			}
		}
		if needInvite {
			if req.InviteCode == "" {
				return nil, ErrInvalidInviteCode
			}
			// 先匹配数据库一码一用
			matched := false
			if s.inviteRepo != nil {
				if ic, err := s.inviteRepo.FindByCode(req.InviteCode); err == nil && ic != nil {
					// 过期校验
					if ic.ExpiresAt != nil && time.Now().After(*ic.ExpiresAt) {
						return nil, ErrInvalidInviteCode
					}
					// 使用次数上限
					if ic.MaxUses > 0 && ic.UsedCount >= ic.MaxUses {
						return nil, ErrInvalidInviteCode
					}
					consumedInvite = ic
					matched = true
				}
			}
			// 再匹配配置中的全局邀请码
			if !matched && s.cfg.Registration.InviteCode != "" && req.InviteCode == s.cfg.Registration.InviteCode {
				matched = true
			}
			if !matched {
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

	// 邀请码消费
	if consumedInvite != nil && s.inviteRepo != nil {
		_ = s.inviteRepo.IncrUsed(consumedInvite.ID)
	}

	// 首次登录即设置 IP
	_ = s.userRepo.UpdateLastLogin(user.ID, ip)
	s.recordLogin(user.ID, user.Username, ip, ua, "", true)

	s.logger.Infof("新用户注册: %s (角色: %s)", user.Username, user.Role)
	return s.generateToken(user)
}

// RefreshToken 刷新令牌
func (s *AuthService) RefreshToken(userID string) (*TokenResponse, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	if user.Disabled {
		return nil, ErrUserDisabled
	}
	return s.generateToken(user)
}

// ValidateTokenVersion 供 JWT 中间件使用：返回用户当前令牌版本号和禁用状态
func (s *AuthService) ValidateTokenVersion(userID string) (int, bool, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return 0, false, err
	}
	return user.TokenVersion, user.Disabled, nil
}

// generateToken 生成JWT令牌
func (s *AuthService) generateToken(user *model.User) (*TokenResponse, error) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7天过期

	claims := &middleware.Claims{
		UserID:       user.ID,
		Username:     user.Username,
		Role:         user.Role,
		TokenVersion: user.TokenVersion,
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
// 修改成功后会自增 token_version，使旧 Token 失效；当前会话需重新登录获取新 Token。
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

	// 更新密码 + 清除强制改密标记 + 自增 token_version
	if err := s.userRepo.UpdateFields(userID, map[string]interface{}{
		"password":        string(hashedPassword),
		"must_change_pwd": false,
	}); err != nil {
		return err
	}
	_ = s.userRepo.IncrTokenVersion(userID)

	s.logger.Infof("用户 %s 修改了密码", user.Username)
	return nil
}

// ResetPassword 管理员重置用户密码（无需旧密码）
// 重置成功后自增 token_version 并可选地设置 must_change_pwd=true。
func (s *AuthService) ResetPassword(userID string, newPassword string, forceChangeOnNextLogin bool) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return ErrUserNotFound
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	fields := map[string]interface{}{
		"password":        string(hashedPassword),
		"must_change_pwd": forceChangeOnNextLogin,
	}
	if err := s.userRepo.UpdateFields(userID, fields); err != nil {
		return err
	}
	_ = s.userRepo.IncrTokenVersion(userID)

	s.logger.Infof("管理员重置了用户 %s 的密码", user.Username)
	return nil
}
