package service

import (
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// ==================== 随机延迟工具 ====================
// 所有外部 HTTP 刮削服务共享，防止因请求过于频繁导致 IP 被封禁或限流

// scrapeRand 全局随机数生成器（并发安全）
var (
	scrapeRandMu  sync.Mutex
	scrapeRandSrc = rand.New(rand.NewSource(time.Now().UnixNano()))
)

// randomDelay 在 [minMs, maxMs] 毫秒之间随机等待
// 使用随机化的延迟模式来模拟真实用户行为，避免被反爬虫机制检测到固定间隔模式
func randomDelay(minMs, maxMs int) {
	if minMs >= maxMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}
	scrapeRandMu.Lock()
	delay := minMs + scrapeRandSrc.Intn(maxMs-minMs)
	scrapeRandMu.Unlock()
	time.Sleep(time.Duration(delay) * time.Millisecond)
}

// ==================== User-Agent 轮换池 ====================
// 模拟真实浏览器请求，降低被识别为爬虫的风险

// browserUserAgents 常见桌面浏览器 User-Agent 列表
var browserUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36 Edg/122.0.0.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:122.0) Gecko/20100101 Firefox/122.0",
}

// getRandomUserAgent 从 User-Agent 池中随机选取一个
func getRandomUserAgent() string {
	scrapeRandMu.Lock()
	idx := scrapeRandSrc.Intn(len(browserUserAgents))
	scrapeRandMu.Unlock()
	return browserUserAgents[idx]
}

// ==================== 请求头增强 ====================

// setBrowserHeaders 为 HTTP 请求设置完整的浏览器请求头
// 模拟真实浏览器行为，包含 Accept-Language、Sec-Fetch-* 等
//
// 注意：不要手动设置 Accept-Encoding！
// Go net/http 默认会自动添加 Accept-Encoding: gzip 并自动解压响应；
// 一旦手动设置（例如 "gzip, deflate, br"），Go 会认为"调用方自行处理压缩"，
// 不再自动解压，导致 io.ReadAll 读到的是二进制 gzip 流，字符串匹配/正则全失效。
func setBrowserHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,ja;q=0.7")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	// Sec-Fetch-* 是现代浏览器发起的"导航/资源请求"标记，
	// 豆瓣的反爬会根据这些头部判断"是真实浏览器导航"还是"脚本请求"
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	// Client Hints（可选但有助于通过风控）
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

// setAPIHeaders 为 API 请求设置合理的请求头（JSON API 类型）
// 适用于 TMDb、Bangumi 等 REST API
func setAPIHeaders(req *http.Request) {
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
}
