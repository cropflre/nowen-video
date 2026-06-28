// Package emby 实现 Emby Server API 兼容层，用于支持 Infuse、Emby for iOS/Android、
// Kodi Emby 插件等第三方客户端直接连接本服务器。
//
// 本层不改动底层业务模型（仍然使用 UUID 作为主键），而是在边界处完成：
//   - UUID ↔ 数字字符串 ID 的双向稳定映射（Emby/Infuse 期望形如 "12345" 的数字 ID）
//   - 下划线命名 ↔ PascalCase 的字段转换（Emby JSON 使用 PascalCase）
//   - nowen 业务模型 ↔ Emby BaseItemDto 语义对齐
package emby

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strconv"
	"sync"
)

// IDMapper 负责在 UUID 字符串主键与 Emby 风格数字/字符串 ID 之间做稳定双向映射。
//
// 改进点：
//   - 使用 int31 空间避免 Kodi setDbId 大整数崩溃
//   - 添加碰撞处理：当发生哈希碰撞时，使用 salt 递增生成新 ID
//   - 支持启动时预热 ID 映射
//
// Emby 客户端普遍把 ItemId 当成整数处理（Infuse 某些版本直接 parseInt），
// 因此我们基于 UUID 的 SHA-256 前 4 字节生成一个非负 int32，
// 该结果对同一 UUID 始终稳定，进程重启后仍然一致。
//
// 为了支持反向查询（数字 ID → UUID），缓存在内存里，
// 首次遇到某个 UUID 时自动写入；遇到陌生数字 ID 时（例如客户端缓存的旧 ID），
// 上层会再查一次 DB（或直接当成 UUID）来恢复。
type IDMapper struct {
	mu      sync.RWMutex
	forward map[string]int32 // uuid -> emby_id
	reverse map[int32]string // emby_id -> uuid
	salt    map[string]int   // uuid -> 当前使用的 salt（用于碰撞处理）
}

// NewIDMapper 创建 ID 映射器。
func NewIDMapper() *IDMapper {
	return &IDMapper{
		forward: make(map[string]int32, 4096),
		reverse: make(map[int32]string, 4096),
		salt:    make(map[string]int, 4096),
	}
}

// ToEmbyID 将内部 UUID 转成 Emby 客户端使用的字符串 ID。
// 空字符串返回空字符串，调用方应在上层过滤。
func (m *IDMapper) ToEmbyID(uuid string) string {
	if uuid == "" {
		return ""
	}

	// 若 uuid 本身就是纯数字（比如未来可能的整型迁移），直接返回。
	if isAllDigits(uuid) {
		id, _ := strconv.ParseInt(uuid, 10, 32)
		m.register(int32(id), uuid)
		return uuid
	}

	// 检查是否已存在映射
	m.mu.RLock()
	if id, ok := m.forward[uuid]; ok {
		m.mu.RUnlock()
		return strconv.FormatInt(int64(id), 10)
	}
	m.mu.RUnlock()

	// 生成 ID 并处理碰撞
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if id, ok := m.forward[uuid]; ok {
		return strconv.FormatInt(int64(id), 10)
	}

	// 生成 ID 并处理碰撞
	salt := 0
	if existingSalt, ok := m.salt[uuid]; ok {
		salt = existingSalt
	}

	for {
		id := m.generateID(uuid, salt)
		if existingUUID, exists := m.reverse[id]; !exists {
			// 未碰撞，直接注册
			m.forward[uuid] = id
			m.reverse[id] = uuid
			m.salt[uuid] = salt
			return strconv.FormatInt(int64(id), 10)
		} else if existingUUID == uuid {
			// 同一 UUID，直接返回
			return strconv.FormatInt(int64(id), 10)
		}
		// 碰撞，递增 salt 重试
		salt++
	}
}

// Resolve 将 Emby 客户端传回的 ID 还原成内部 UUID。
// 如果映射表里没有记录，就把它当成 UUID 原样返回（兼容客户端直接使用 UUID 的情况）。
func (m *IDMapper) Resolve(embyID string) string {
	if embyID == "" {
		return ""
	}

	// 尝试解析为数字 ID
	if id, err := strconv.ParseInt(embyID, 10, 32); err == nil {
		m.mu.RLock()
		orig, ok := m.reverse[int32(id)]
		m.mu.RUnlock()
		if ok {
			return orig
		}
	}

	// 如果不是数字或映射表里没有记录，就把它当成 UUID 原样返回
	return embyID
}

// RegisterMany 批量注册映射关系，常用于列表响应之前预热。
func (m *IDMapper) RegisterMany(uuids []string) {
	for _, u := range uuids {
		_ = m.ToEmbyID(u)
	}
}

// WarmupAll 预热所有 ID 映射（启动时调用）。
func (m *IDMapper) WarmupAll(libraries []string, media []string, series []string) {
	m.RegisterMany(libraries)
	m.RegisterMany(media)
	m.RegisterMany(series)
}

// register 注册映射关系（内部使用，调用方需持有锁）。
func (m *IDMapper) register(id int32, uuid string) {
	m.mu.Lock()
	m.forward[uuid] = id
	m.reverse[id] = uuid
	m.mu.Unlock()
}

// generateID 生成 int31 ID（内部使用，调用方需持有锁）。
func (m *IDMapper) generateID(uuid string, salt int) int32 {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", uuid, salt)))
	return int32(binary.BigEndian.Uint32(hash[:4]) & 0x7FFFFFFF)
}

// GetStats 获取映射统计信息（用于调试）。
func (m *IDMapper) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"forward_count": len(m.forward),
		"reverse_count": len(m.reverse),
		"salt_count":    len(m.salt),
	}
}

// HasCollision 检查是否存在碰撞（用于测试）。
func (m *IDMapper) HasCollision() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.salt) > 0
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
