// Package service 番号刮削的智能代理/镜像管理
// 借鉴自 mdcx-master 的多镜像轮询策略
//
// 功能：
//   - 每个数据源维护多镜像列表（主站 + 镜像）
//   - 自动探测镜像可用性（并发 HEAD 请求）
//   - 失败的镜像自动降权/熔断（冷却 10 分钟）
//   - 提供首选镜像优先的 URL 解析
package service

import (
	"net/http"
	"sync"
	"time"
)

// ==================== 内置多镜像列表 ====================

// builtinMirrors 每个数据源的候选镜像列表（首个为首选）
var builtinMirrors = map[string][]string{
	"javbus": {
		"https://www.javbus.com",
		"https://www.buscdn.work",
		"https://www.javbus.red",
		"https://www.seejav.art",
	},
	"javdb": {
		"https://javdb.com",
		"https://javdb001.com",
		"https://javdb002.com",
		"https://javdb521.com",
	},
	"freejavbt": {
		"https://freejavbt.com",
		"https://www.freejavbt.com",
	},
	"jav321": {
		"https://www.jav321.com",
		"https://jav321.com",
	},
	"fanza": {
		"https://www.dmm.co.jp",
	},
	"mgstage": {
		"https://www.mgstage.com",
	},
	"fc2hub": {
		"https://fc2hub.com",
		"https://fc2ppvdb.com",
	},
}

// ==================== 镜像状态管理 ====================

// MirrorStatus 镜像状态
type MirrorStatus struct {
	URL        string    `json:"url"`
	Healthy    bool      `json:"healthy"`
	LatencyMS  int64     `json:"latency_ms"`
	LastCheck  time.Time `json:"last_check"`
	FailCount  int       `json:"fail_count"`
	CooldownTo time.Time `json:"cooldown_to,omitempty"` // 熔断冷却到某时刻
}

// AdultProxyManager 智能代理/镜像管理器
type AdultProxyManager struct {
	mu           sync.RWMutex
	mirrorCache  map[string][]*MirrorStatus // source -> mirrors
	client       *http.Client
	healthCheckInterval time.Duration
	lastHealthAll       time.Time
}

// NewAdultProxyManager 创建镜像管理器
func NewAdultProxyManager() *AdultProxyManager {
	m := &AdultProxyManager{
		mirrorCache: make(map[string][]*MirrorStatus),
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
		healthCheckInterval: 10 * time.Minute,
	}
	// 初始化：从 builtinMirrors 注册
	for src, urls := range builtinMirrors {
		items := make([]*MirrorStatus, 0, len(urls))
		for _, u := range urls {
			items = append(items, &MirrorStatus{URL: u, Healthy: true})
		}
		m.mirrorCache[src] = items
	}
	return m
}

// PreferredURL 返回某数据源当前首选的镜像 URL
// 规则：健康且未熔断的第一个
func (m *AdultProxyManager) PreferredURL(source string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list, ok := m.mirrorCache[source]
	if !ok || len(list) == 0 {
		return ""
	}
	now := time.Now()
	for _, item := range list {
		if item.Healthy && (item.CooldownTo.IsZero() || now.After(item.CooldownTo)) {
			return item.URL
		}
	}
	// 全部不健康时，返回首个（放弃挣扎）
	return list[0].URL
}

// AllMirrors 返回某数据源的所有候选镜像（只读快照）
func (m *AdultProxyManager) AllMirrors(source string) []MirrorStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list, ok := m.mirrorCache[source]
	if !ok {
		return nil
	}
	out := make([]MirrorStatus, 0, len(list))
	for _, item := range list {
		out = append(out, *item)
	}
	return out
}

// SetMirrors 覆盖某数据源的镜像列表（支持用户自定义）
func (m *AdultProxyManager) SetMirrors(source string, urls []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]*MirrorStatus, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			continue
		}
		items = append(items, &MirrorStatus{URL: u, Healthy: true})
	}
	m.mirrorCache[source] = items
}

// MarkFailure 标记某镜像失败（累计触发熔断）
func (m *AdultProxyManager) MarkFailure(source, url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, ok := m.mirrorCache[source]
	if !ok {
		return
	}
	for _, item := range list {
		if item.URL != url {
			continue
		}
		item.FailCount++
		// 连续失败 3 次熔断 10 分钟
		if item.FailCount >= 3 {
			item.Healthy = false
			item.CooldownTo = time.Now().Add(10 * time.Minute)
		}
		return
	}
}

// MarkSuccess 标记某镜像成功（清零失败计数）
func (m *AdultProxyManager) MarkSuccess(source, url string, latencyMS int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, ok := m.mirrorCache[source]
	if !ok {
		return
	}
	for _, item := range list {
		if item.URL != url {
			continue
		}
		item.Healthy = true
		item.FailCount = 0
		item.LatencyMS = latencyMS
		item.LastCheck = time.Now()
		item.CooldownTo = time.Time{}
		return
	}
}

// HealthCheckAll 并发健康检查所有镜像
// 返回 (检查总数, 健康数)
func (m *AdultProxyManager) HealthCheckAll() (int, int) {
	m.mu.Lock()
	// 收集所有 (source, mirror) 配对
	type pair struct {
		src   string
		mirror *MirrorStatus
	}
	pairs := []pair{}
	for src, list := range m.mirrorCache {
		for _, item := range list {
			pairs = append(pairs, pair{src, item})
		}
	}
	m.mu.Unlock()

	if len(pairs) == 0 {
		return 0, 0
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		healthy   int
		sem       = make(chan struct{}, 6) // 并发上限 6
	)

	for _, p := range pairs {
		p := p
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ok, latency := m.checkSingle(p.mirror.URL)
			m.mu.Lock()
			p.mirror.LastCheck = time.Now()
			if ok {
				p.mirror.Healthy = true
				p.mirror.LatencyMS = latency
				p.mirror.FailCount = 0
				p.mirror.CooldownTo = time.Time{}
				mu.Lock()
				healthy++
				mu.Unlock()
			} else {
				p.mirror.FailCount++
				if p.mirror.FailCount >= 3 {
					p.mirror.Healthy = false
					p.mirror.CooldownTo = time.Now().Add(10 * time.Minute)
				}
			}
			m.mu.Unlock()
		}()
	}
	wg.Wait()

	m.mu.Lock()
	m.lastHealthAll = time.Now()
	m.mu.Unlock()
	return len(pairs), healthy
}

// checkSingle 单镜像健康检查（HEAD 请求）
func (m *AdultProxyManager) checkSingle(url string) (bool, int64) {
	start := time.Now()
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false, 0
	}
	setAdultScraperHeaders(req)
	resp, err := m.client.Do(req)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	latency := time.Since(start).Milliseconds()
	// 2xx/3xx 都视为可达
	if resp.StatusCode < 400 {
		return true, latency
	}
	return false, latency
}

// LastHealthAt 最近一次批量健康检查时间
func (m *AdultProxyManager) LastHealthAt() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastHealthAll
}
