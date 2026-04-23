package service

import "errors"

// 业务错误定义
var (
	ErrInvalidCredentials   = errors.New("用户名或密码错误")
	ErrUserExists           = errors.New("用户名已存在")
	ErrUserNotFound         = errors.New("用户不存在")
	ErrUserDisabled         = errors.New("账号已被禁用，请联系管理员")
	ErrLastAdmin            = errors.New("不能删除或降级最后一个管理员")
	ErrLibraryNotFound      = errors.New("媒体库不存在")
	ErrMediaNotFound        = errors.New("媒体不存在")
	ErrAlreadyFavorited     = errors.New("已收藏")
	ErrScanInProgress       = errors.New("扫描正在进行中")
	ErrForbidden            = errors.New("无权操作")
	ErrPlaylistNotFound     = errors.New("播放列表不存在")
	ErrUnauthorized         = errors.New("未授权操作")
	ErrInvalidRating        = errors.New("无效的内容分级")
	ErrTimeLimitExceeded    = errors.New("已超出每日观看时长限制")
	ErrContentRestricted    = errors.New("内容分级限制，无权观看")
	ErrTaskNotFound         = errors.New("任务不存在")
	ErrRegistrationDisabled = errors.New("注册功能已关闭，请联系管理员")
	ErrInvalidInviteCode    = errors.New("邀请码无效")
)
