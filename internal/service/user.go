package service

import (
	cryptoRand "crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// UserService 用户服务
type UserService struct {
	repo      *repository.UserRepo
	auditRepo *repository.AuditLogRepo
	cfg       *config.Config
	logger    *zap.SugaredLogger
}

func NewUserService(repo *repository.UserRepo, auditRepo *repository.AuditLogRepo, cfg *config.Config, logger *zap.SugaredLogger) *UserService {
	return &UserService{repo: repo, auditRepo: auditRepo, cfg: cfg, logger: logger}
}

// generateSecurePassword 生成安全的随机密码（16字符十六进制）
func generateSecurePassword() (string, error) {
	b := make([]byte, 8)
	if _, err := cryptoRand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机密码失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// defaultAdminPassword 默认管理员密码（与前端登录页提示一致）
const defaultAdminPassword = "admin123"

// EnsureAdminExists 确保管理员账号存在（首次启动时）
func (s *UserService) EnsureAdminExists() error {
	count, err := s.repo.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // 已有用户，跳过
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(defaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &model.User{
		Username:      "admin",
		Password:      string(hashedPassword),
		Role:          "admin",
		MustChangePwd: true, // 首次登录强制修改密码
	}

	if err := s.repo.Create(admin); err != nil {
		return err
	}

	s.logger.Infof("╔══════════════════════════════════════════════════╗")
	s.logger.Infof("║  首次启动 — 已创建默认管理员账号                    ║")
	s.logger.Infof("║  用户名: admin                                   ║")
	s.logger.Infof("║  密码:   admin123                                ║")
	s.logger.Infof("║  ⚠️  首次登录后需强制修改密码                       ║")
	s.logger.Infof("╚══════════════════════════════════════════════════╝")
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

// CreateUserRequest 管理员创建用户
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6,max=64"`
	Role     string `json:"role"`     // admin / user，默认 user
	Nickname string `json:"nickname"` // 可选
	Email    string `json:"email"`    // 可选
}

// CreateUser 管理员直接创建用户
func (s *UserService) CreateUser(req *CreateUserRequest) (*model.User, error) {
	if _, err := s.repo.FindByUsername(req.Username); err == nil {
		return nil, ErrUserExists
	}
	role := req.Role
	if role != "admin" && role != "user" {
		role = "user"
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:      req.Username,
		Password:      string(hashed),
		Role:          role,
		Nickname:      req.Nickname,
		Email:         req.Email,
		MustChangePwd: true, // 管理员建号后要求首次登录改密
	}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	s.logger.Infof("管理员创建了用户: %s (角色: %s)", user.Username, user.Role)
	return user, nil
}

// UpdateUserRequest 管理员更新用户（部分字段）
type UpdateUserRequest struct {
	Role     *string `json:"role,omitempty"`
	Nickname *string `json:"nickname,omitempty"`
	Email    *string `json:"email,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
}

// UpdateUserByAdmin 管理员更新用户信息
// 注意：切换角色会自增 token_version，使旧 Token 失效
func (s *UserService) UpdateUserByAdmin(id string, req *UpdateUserRequest) (*model.User, error) {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrUserNotFound
	}
	fields := map[string]interface{}{}
	bumpTV := false
	if req.Role != nil {
		role := *req.Role
		if role != "admin" && role != "user" {
			role = "user"
		}
		// 降级最后一个管理员 -> 拒绝
		if user.Role == "admin" && role != "admin" {
			count, _ := s.repo.CountAdmins()
			if count <= 1 {
				return nil, ErrLastAdmin
			}
		}
		if role != user.Role {
			fields["role"] = role
			bumpTV = true
		}
	}
	if req.Nickname != nil {
		fields["nickname"] = *req.Nickname
	}
	if req.Email != nil {
		fields["email"] = *req.Email
	}
	if req.Avatar != nil {
		fields["avatar"] = *req.Avatar
	}
	if len(fields) > 0 {
		if err := s.repo.UpdateFields(id, fields); err != nil {
			return nil, err
		}
	}
	if bumpTV {
		_ = s.repo.IncrTokenVersion(id)
	}
	return s.repo.FindByID(id)
}

// SetUserDisabled 管理员封禁/解禁用户
func (s *UserService) SetUserDisabled(id string, disabled bool) error {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return ErrUserNotFound
	}
	// 不能禁用最后一个管理员
	if disabled && user.Role == "admin" {
		count, _ := s.repo.CountAdmins()
		if count <= 1 {
			return ErrLastAdmin
		}
	}
	if err := s.repo.SetDisabled(id, disabled); err != nil {
		return err
	}
	if disabled {
		_ = s.repo.IncrTokenVersion(id) // 封禁时使旧 Token 失效
	}
	return nil
}

// UpdateSelfProfileRequest 用户自助更新资料
type UpdateSelfProfileRequest struct {
	Nickname *string `json:"nickname,omitempty"`
	Email    *string `json:"email,omitempty"`
	Avatar   *string `json:"avatar,omitempty"`
}

// UpdateSelfProfile 用户更新自己的昵称、邮箱、头像
func (s *UserService) UpdateSelfProfile(userID string, req *UpdateSelfProfileRequest) (*model.User, error) {
	fields := map[string]interface{}{}
	if req.Nickname != nil {
		fields["nickname"] = *req.Nickname
	}
	if req.Email != nil {
		fields["email"] = *req.Email
	}
	if req.Avatar != nil {
		fields["avatar"] = *req.Avatar
	}
	if len(fields) > 0 {
		if err := s.repo.UpdateFields(userID, fields); err != nil {
			return nil, err
		}
	}
	return s.repo.FindByID(userID)
}

// DeleteUser 删除用户
// 禁止删除最后一个管理员
func (s *UserService) DeleteUser(id string) error {
	user, err := s.repo.FindByID(id)
	if err != nil {
		return ErrUserNotFound
	}
	if user.Role == "admin" {
		count, _ := s.repo.CountAdmins()
		if count <= 1 {
			return ErrLastAdmin
		}
	}
	// 自增 token_version，彻底吊销用户持有的 Token
	_ = s.repo.IncrTokenVersion(id)
	return s.repo.Delete(id)
}

// Audit 记录管理员操作（幂等失败不影响主流程）
func (s *UserService) Audit(operatorID, operator, action, targetType, targetID, detail, ip string) {
	if s.auditRepo == nil {
		return
	}
	_ = s.auditRepo.Create(&model.AuditLog{
		OperatorID: operatorID,
		Operator:   operator,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IP:         ip,
	})
}
