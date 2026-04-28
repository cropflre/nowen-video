// Package service 番号元数据缓存（LRU + TTL）
// 借鉴自 mdcx-master 的缓存设计，避免短时间重复抓取同一番号
// 缓存落盘，服务重启后保留
package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	cacheFileName     = "adult_metadata_cache.json"
	defaultCacheTTL   = 7 * 24 * time.Hour // 默认 7 天
	defaultCacheLimit = 5000
)

// adultCacheEntry 缓存条目
type adultCacheEntry struct {
	Meta      *AdultMetadata `json:"meta"`
	ExpiresAt time.Time      `json:"expires_at"`
	Hit       int            `json:"hit"`
}

// AdultMetadataCache 番号元数据缓存
type AdultMetadataCache struct {
	mu        sync.RWMutex
	filePath  string
	entries   map[string]*adultCacheEntry // key = 番号
	ttl       time.Duration
	maxItems  int
	lastPurge time.Time
}

// NewAdultMetadataCache 创建缓存
func NewAdultMetadataCache(dir string) *AdultMetadataCache {
	if dir == "" {
		dir = "./data"
	}
	_ = os.MkdirAll(dir, 0o755)
	c := &AdultMetadataCache{
		filePath: filepath.Join(dir, cacheFileName),
		entries:  make(map[string]*adultCacheEntry),
		ttl:      defaultCacheTTL,
		maxItems: defaultCacheLimit,
	}
	_ = c.load()
	return c
}

// SetTTL 调整 TTL
func (c *AdultMetadataCache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl > 0 {
		c.ttl = ttl
	}
}

// Get 从缓存取元数据（命中则返回）
func (c *AdultMetadataCache) Get(code string) (*AdultMetadata, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[code]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(c.entries, code)
		return nil, false
	}
	entry.Hit++
	return entry.Meta, true
}

// Put 写入缓存
func (c *AdultMetadataCache) Put(code string, meta *AdultMetadata) {
	if meta == nil || code == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// 容量检查
	if len(c.entries) >= c.maxItems {
		c.evictOneLocked()
	}

	c.entries[code] = &adultCacheEntry{
		Meta:      meta,
		ExpiresAt: time.Now().Add(c.ttl),
	}
	// 定期刷盘（每 5 分钟最多一次）
	if time.Since(c.lastPurge) > 5*time.Minute {
		c.lastPurge = time.Now()
		go func() { _ = c.Save() }()
	}
}

// Invalidate 失效某个番号的缓存（手动刷新）
func (c *AdultMetadataCache) Invalidate(code string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, code)
}

// Clear 清空全部缓存
func (c *AdultMetadataCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*adultCacheEntry)
	_ = os.Remove(c.filePath)
}

// Size 当前缓存数量
func (c *AdultMetadataCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Stats 缓存统计
type AdultCacheStats struct {
	Size    int `json:"size"`
	MaxSize int `json:"max_size"`
	TotalHit int `json:"total_hit"`
	TTL      string `json:"ttl"`
}

// Stats 返回缓存统计信息
func (c *AdultMetadataCache) Stats() AdultCacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	totalHit := 0
	for _, e := range c.entries {
		totalHit += e.Hit
	}
	return AdultCacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxItems,
		TotalHit: totalHit,
		TTL:     c.ttl.String(),
	}
}

// Save 持久化到文件
func (c *AdultMetadataCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := json.Marshal(c.entries)
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, data, 0o644)
}

// load 从文件加载
func (c *AdultMetadataCache) load() error {
	raw, err := os.ReadFile(c.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var entries map[string]*adultCacheEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	// 清理已过期的
	now := time.Now()
	for k, e := range entries {
		if now.After(e.ExpiresAt) {
			delete(entries, k)
		}
	}
	c.entries = entries
	return nil
}

// evictOneLocked 淘汰一条（最久未使用的），需调用方持有写锁
func (c *AdultMetadataCache) evictOneLocked() {
	var oldestKey string
	var oldestAt time.Time
	first := true
	for k, e := range c.entries {
		if first || e.ExpiresAt.Before(oldestAt) {
			oldestKey = k
			oldestAt = e.ExpiresAt
			first = false
		}
	}
	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}
