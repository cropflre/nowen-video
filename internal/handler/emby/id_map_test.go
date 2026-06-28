package emby

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIDMapper_BasicMapping(t *testing.T) {
	mapper := NewIDMapper()

	// 测试正常映射
	id1 := mapper.ToEmbyID("uuid-1")
	id2 := mapper.ToEmbyID("uuid-2")

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// 测试稳定性
	id1Again := mapper.ToEmbyID("uuid-1")
	assert.Equal(t, id1, id1Again)
}

func TestIDMapper_EmptyUUID(t *testing.T) {
	mapper := NewIDMapper()

	// 空 UUID 应该返回空字符串
	assert.Equal(t, "", mapper.ToEmbyID(""))
}

func TestIDMapper_NumericUUID(t *testing.T) {
	mapper := NewIDMapper()

	// 纯数字 UUID 应该直接返回
	assert.Equal(t, "12345", mapper.ToEmbyID("12345"))
	assert.Equal(t, "0", mapper.ToEmbyID("0"))
}

func TestIDMapper_Resolve(t *testing.T) {
	mapper := NewIDMapper()

	// 注册映射
	uuid := "test-uuid-123"
	embyID := mapper.ToEmbyID(uuid)

	// 测试反向解析
	resolved := mapper.Resolve(embyID)
	assert.Equal(t, uuid, resolved)

	// 测试未知 ID 的回退
	assert.Equal(t, "unknown-id", mapper.Resolve("unknown-id"))
}

func TestIDMapper_ResolveEmpty(t *testing.T) {
	mapper := NewIDMapper()

	// 空 ID 应该返回空字符串
	assert.Equal(t, "", mapper.Resolve(""))
}

func TestIDMapper_CollisionHandling(t *testing.T) {
	mapper := NewIDMapper()

	// 生成大量 UUID，测试碰撞处理
	uuids := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		uuids[i] = "uuid-" + string(rune(i))
	}

	// 注册所有 UUID
	ids := make(map[string]bool)
	for _, uuid := range uuids {
		id := mapper.ToEmbyID(uuid)
		assert.NotEmpty(t, id)
		assert.False(t, ids[id], "ID 碰撞: %s", id)
		ids[id] = true
	}

	// 验证所有 UUID 都能正确解析
	for _, uuid := range uuids {
		id := mapper.ToEmbyID(uuid)
		resolved := mapper.Resolve(id)
		assert.Equal(t, uuid, resolved)
	}
}

func TestIDMapper_ConcurrentAccess(t *testing.T) {
	mapper := NewIDMapper()

	// 并发测试
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			uuid := "uuid-" + string(rune(i))
			id := mapper.ToEmbyID(uuid)
			resolved := mapper.Resolve(id)
			assert.Equal(t, uuid, resolved)
		}(i)
	}

	wg.Wait()
}

func TestIDMapper_RegisterMany(t *testing.T) {
	mapper := NewIDMapper()

	uuids := []string{"uuid-1", "uuid-2", "uuid-3"}
	mapper.RegisterMany(uuids)

	// 验证所有 UUID 都已注册
	for _, uuid := range uuids {
		id := mapper.ToEmbyID(uuid)
		resolved := mapper.Resolve(id)
		assert.Equal(t, uuid, resolved)
	}
}

func TestIDMapper_WarmupAll(t *testing.T) {
	mapper := NewIDMapper()

	libraries := []string{"lib-1", "lib-2"}
	media := []string{"media-1", "media-2", "media-3"}
	series := []string{"series-1", "series-2"}

	mapper.WarmupAll(libraries, media, series)

	// 验证所有 UUID 都已注册
	allUUIDs := append(libraries, media...)
	allUUIDs = append(allUUIDs, series...)

	for _, uuid := range allUUIDs {
		id := mapper.ToEmbyID(uuid)
		resolved := mapper.Resolve(id)
		assert.Equal(t, uuid, resolved)
	}
}

func TestIDMapper_GetStats(t *testing.T) {
	mapper := NewIDMapper()

	// 注册一些 UUID
	mapper.ToEmbyID("uuid-1")
	mapper.ToEmbyID("uuid-2")

	stats := mapper.GetStats()
	assert.Equal(t, 2, stats["forward_count"])
	assert.Equal(t, 2, stats["reverse_count"])
}

func TestIDMapper_HasCollision(t *testing.T) {
	mapper := NewIDMapper()

	// 初始状态应该没有碰撞
	assert.False(t, mapper.HasCollision())
}

func TestIDMapper_IDLength(t *testing.T) {
	mapper := NewIDMapper()

	// 测试生成的 ID 长度
	id := mapper.ToEmbyID("test-uuid")

	// int31 最大值为 2^31-1 = 2147483647，最多 10 位数字
	assert.LessOrEqual(t, len(id), 10)
	assert.Greater(t, len(id), 0)
}
