// Package emby 实现 Emby Server API 兼容层，用于支持 Infuse、Emby for iOS/Android、
// Kodi Emby 插件等第三方客户端直接连接本服务器。
//
// 本层不改动底层业务模型（仍然使用 UUID 作为主键），而是在边界处完成：
//   - UUID ↔ 数字字符串 ID 的双向稳定映射（Emby/Infuse 期望形如 "12345" 的数字 ID）
//   - 下划线命名 ↔ PascalCase 的字段转换（Emby JSON 使用 PascalCase）
//   - nowen 业务模型 ↔ Emby BaseItemDto 语义对齐
package emby

import (
	"crypto/sha1"
	"encoding/binary"
	"strconv"
	"sync"
)

// IDMapper 负责在 UUID 字符串主键与 Emby 风格数字/字符串 ID 之间做稳定双向映射。
//
// Emby 客户端普遍把 ItemId 当成整数处理（Infuse 某些版本直接 parseInt），
// 因此我们基于 UUID 的 SHA-1 前 8 字节生成一个非负 int64，
// 该结果对同一 UUID 始终稳定，进程重启后仍然一致。
//
// 为了支持反向查询（数字 ID → UUID），缓存在内存里，
// 首次遇到某个 UUID 时自动写入；遇到陌生数字 ID 时（例如客户端缓存的旧 ID），
// 上层会再查一次 DB（或直接当成 UUID）来恢复。
type IDMapper struct {
	mu sync.RWMutex
	// numeric -> original string (UUID)
	reverse map[string]string
}

// NewIDMapper 创建 ID 映射器。
func NewIDMapper() *IDMapper {
	return &IDMapper{reverse: make(map[string]string, 4096)}
}

// ToEmbyID 将内部 UUID 转成 Emby 客户端使用的字符串 ID。
// 空字符串返回空字符串，调用方应在上层过滤。
func (m *IDMapper) ToEmbyID(uuid string) string {
	if uuid == "" {
		return ""
	}
	// 若 uuid 本身就是纯数字（比如未来可能的整型迁移），直接返回。
	if isAllDigits(uuid) {
		m.register(uuid, uuid)
		return uuid
	}
	h := sha1.Sum([]byte(uuid))
	// 取前 8 字节转成非负 int64
	v := int64(binary.BigEndian.Uint64(h[:8]) & 0x7FFFFFFFFFFFFFFF)
	s := strconv.FormatInt(v, 10)
	m.register(s, uuid)
	return s
}

// Resolve 将 Emby 客户端传回的 ID 还原成内部 UUID。
// 如果映射表里没有记录，就把它当成 UUID 原样返回（兼容客户端直接使用 UUID 的情况）。
func (m *IDMapper) Resolve(embyID string) string {
	if embyID == "" {
		return ""
	}
	m.mu.RLock()
	orig, ok := m.reverse[embyID]
	m.mu.RUnlock()
	if ok {
		return orig
	}
	return embyID
}

// RegisterMany 批量注册映射关系，常用于列表响应之前预热。
func (m *IDMapper) RegisterMany(uuids []string) {
	for _, u := range uuids {
		_ = m.ToEmbyID(u)
	}
}

func (m *IDMapper) register(numeric, uuid string) {
	m.mu.Lock()
	m.reverse[numeric] = uuid
	m.mu.Unlock()
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
