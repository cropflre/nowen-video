package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
)

// STRMMeta 一个 .strm 文件解析后的完整元数据
// 支持从以下四种来源汇总：
//  1. KODI 格式：#KODIPROP:inputstream.adaptive.manifest_headers=User-Agent=xxx&Referer=yyy
//  2. 注释提示：# ua=xxx / # referer=yyy / # cookie=k=v / # header=K:V
//  3. M3U 扩展头：#EXTVLCOPT:http-user-agent=xxx
//  4. 同名 .json 侧车：my.strm 旁边放 my.strm.json（优先级最高，覆盖上面）
type STRMMeta struct {
	URL        string            `json:"url"`
	UserAgent  string            `json:"user_agent"`
	Referer    string            `json:"referer"`
	Cookie     string            `json:"cookie"`
	Headers    map[string]string `json:"headers"`
	RefreshURL string            `json:"refresh_url"` // 预留：用于动态刷新的上游 API
}

// ParseSTRMFileEnhanced 增强版 .strm 解析器
// - 支持 KODI/M3U 扩展头、行注释元数据
// - 支持同名 .strm.json 侧车覆写
// - 支持多行 URL（取第一条有效 http(s)/magnet/ftp）
func ParseSTRMFileEnhanced(filePath string) (*STRMMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取 .strm 文件失败: %w", err)
	}

	meta := &STRMMeta{Headers: map[string]string{}}

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			parseSTRMDirective(line, meta)
			continue
		}
		if meta.URL == "" && isSupportedStreamScheme(line) {
			meta.URL = line
		}
	}

	if meta.URL == "" {
		return nil, fmt.Errorf(".strm 文件中未找到有效的 URL: %s", filePath)
	}

	// 读取同名 .json 侧车（优先级最高）
	sidecarPath := filePath + ".json"
	if sb, err := os.ReadFile(sidecarPath); err == nil {
		var sc STRMMeta
		if err := json.Unmarshal(sb, &sc); err == nil {
			mergeSTRMMeta(meta, &sc)
		}
	} else {
		// 也兼容 basename.json（去掉 .strm 扩展名后 + .json）
		alt := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".json"
		if sb2, err2 := os.ReadFile(alt); err2 == nil {
			var sc STRMMeta
			if err := json.Unmarshal(sb2, &sc); err == nil {
				mergeSTRMMeta(meta, &sc)
			}
		}
	}

	return meta, nil
}

// isSupportedStreamScheme 判断一行是否为可接受的流 URL 协议
func isSupportedStreamScheme(line string) bool {
	lower := strings.ToLower(line)
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "rtmp://") ||
		strings.HasPrefix(lower, "rtsp://") ||
		strings.HasPrefix(lower, "ftp://") ||
		strings.HasPrefix(lower, "magnet:") ||
		strings.HasPrefix(lower, "ed2k://")
}

// isDirectVideoLink 判断是否是直链视频（http(s) + 明显扩展名），适合做 FFprobe 远程探测
// HLS / 磁力 / rtmp 不适合
func isDirectVideoLink(u string) bool {
	lower := strings.ToLower(u)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	if strings.Contains(lower, ".m3u8") {
		return false
	}
	// 去 query
	if idx := strings.Index(lower, "?"); idx > 0 {
		lower = lower[:idx]
	}
	for _, ext := range []string{".mp4", ".mkv", ".mov", ".webm", ".avi", ".ts", ".m4v", ".flv"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// parseSTRMDirective 解析单行注释指令，回填到 meta
//
// 支持：
//
//	#KODIPROP:inputstream.adaptive.manifest_headers=User-Agent=xxx&Referer=yyy
//	#EXTVLCOPT:http-user-agent=xxx
//	#EXTVLCOPT:http-referrer=xxx
//	# ua=xxx
//	# user-agent=xxx
//	# referer=xxx
//	# cookie=k=v; k2=v2
//	# header=Key:Value
//	# refresh=https://api.example.com/refresh?id=xxx
func parseSTRMDirective(line string, meta *STRMMeta) {
	// 去掉开头的 '#' 和可选空格
	trimmed := strings.TrimLeft(line, "#")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return
	}
	upper := strings.ToUpper(trimmed)

	switch {
	case strings.HasPrefix(upper, "KODIPROP:"):
		body := strings.TrimSpace(trimmed[len("KODIPROP:"):])
		// 典型格式：key=value
		if eq := strings.Index(body, "="); eq > 0 {
			key := strings.ToLower(strings.TrimSpace(body[:eq]))
			val := strings.TrimSpace(body[eq+1:])
			if strings.Contains(key, "manifest_headers") || strings.Contains(key, "stream_headers") {
				// value 本身是 URL query 风格：User-Agent=xxx&Referer=yyy
				applyQueryLikeHeaders(val, meta)
			} else if strings.Contains(key, "user-agent") || strings.Contains(key, "useragent") {
				meta.UserAgent = val
			} else if strings.Contains(key, "referer") {
				meta.Referer = val
			} else if strings.Contains(key, "cookie") {
				meta.Cookie = val
			}
		}
	case strings.HasPrefix(upper, "EXTVLCOPT:"):
		body := strings.TrimSpace(trimmed[len("EXTVLCOPT:"):])
		if eq := strings.Index(body, "="); eq > 0 {
			key := strings.ToLower(strings.TrimSpace(body[:eq]))
			val := strings.TrimSpace(body[eq+1:])
			switch {
			case strings.Contains(key, "user-agent"):
				meta.UserAgent = val
			case strings.Contains(key, "referrer"), strings.Contains(key, "referer"):
				meta.Referer = val
			case strings.Contains(key, "cookies"):
				meta.Cookie = val
			}
		}
	default:
		// 自定义简单 key=value 注释（建议作者使用的写法）
		if eq := strings.Index(trimmed, "="); eq > 0 {
			key := strings.ToLower(strings.TrimSpace(trimmed[:eq]))
			val := strings.TrimSpace(trimmed[eq+1:])
			switch key {
			case "ua", "user-agent", "useragent":
				meta.UserAgent = val
			case "referer", "referrer":
				meta.Referer = val
			case "cookie", "cookies":
				meta.Cookie = val
			case "refresh", "refresh_url", "refresh-url":
				meta.RefreshURL = val
			case "header":
				// Key:Value 形式
				if colon := strings.Index(val, ":"); colon > 0 {
					hk := strings.TrimSpace(val[:colon])
					hv := strings.TrimSpace(val[colon+1:])
					if hk != "" {
						meta.Headers[hk] = hv
					}
				}
			case "headers":
				// k1:v1; k2:v2
				applySemicolonHeaders(val, meta)
			}
		}
	}
}

// applyQueryLikeHeaders 把 "Key=Val&Key2=Val2" 应用到 meta
// 优先识别 User-Agent / Referer / Cookie 等熟知 Header，其余放入 meta.Headers
func applyQueryLikeHeaders(raw string, meta *STRMMeta) {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return
	}
	for k, vs := range vals {
		if len(vs) == 0 {
			continue
		}
		v := vs[0]
		switch strings.ToLower(k) {
		case "user-agent", "useragent":
			meta.UserAgent = v
		case "referer", "referrer":
			meta.Referer = v
		case "cookie":
			meta.Cookie = v
		default:
			if k != "" {
				meta.Headers[k] = v
			}
		}
	}
}

// applySemicolonHeaders 解析 "K1:V1; K2:V2" 风格的 headers 字段
func applySemicolonHeaders(raw string, meta *STRMMeta) {
	for _, seg := range strings.Split(raw, ";") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		colon := strings.Index(seg, ":")
		if colon <= 0 {
			continue
		}
		hk := strings.TrimSpace(seg[:colon])
		hv := strings.TrimSpace(seg[colon+1:])
		switch strings.ToLower(hk) {
		case "user-agent", "useragent":
			meta.UserAgent = hv
		case "referer", "referrer":
			meta.Referer = hv
		case "cookie":
			meta.Cookie = hv
		default:
			if hk != "" {
				meta.Headers[hk] = hv
			}
		}
	}
}

// mergeSTRMMeta 用 src 覆盖 dst 中非空字段（src 优先级更高）
func mergeSTRMMeta(dst, src *STRMMeta) {
	if src == nil {
		return
	}
	if src.URL != "" {
		dst.URL = src.URL
	}
	if src.UserAgent != "" {
		dst.UserAgent = src.UserAgent
	}
	if src.Referer != "" {
		dst.Referer = src.Referer
	}
	if src.Cookie != "" {
		dst.Cookie = src.Cookie
	}
	if src.RefreshURL != "" {
		dst.RefreshURL = src.RefreshURL
	}
	if len(src.Headers) > 0 {
		if dst.Headers == nil {
			dst.Headers = map[string]string{}
		}
		for k, v := range src.Headers {
			dst.Headers[k] = v
		}
	}
}

// ApplySTRMMetaToMedia 把解析得到的 meta 写入 Media 实体
func ApplySTRMMetaToMedia(m *model.Media, meta *STRMMeta) {
	if m == nil || meta == nil {
		return
	}
	m.StreamURL = meta.URL
	m.StreamUA = meta.UserAgent
	m.StreamReferer = meta.Referer
	m.StreamCookie = meta.Cookie
	m.StreamRefreshURL = meta.RefreshURL
	if len(meta.Headers) > 0 {
		if b, err := json.Marshal(meta.Headers); err == nil {
			m.StreamHeaders = string(b)
		}
	} else {
		m.StreamHeaders = ""
	}
}

// DecodeSTRMHeaders 把 media.StreamHeaders JSON 字符串反序列化为 map
func DecodeSTRMHeaders(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

// ResolveSTRMHeaders 按优先级构造最终请求头集合
// 优先级（低 -> 高）：config 全局默认 < 域名白名单 < media 自身自定义
func ResolveSTRMHeaders(cfg *config.STRMConfig, media *model.Media) (ua, referer, cookie string, headers map[string]string) {
	headers = map[string]string{}

	// 全局默认
	if cfg != nil {
		if cfg.DefaultUserAgent != "" {
			ua = cfg.DefaultUserAgent
		}
		if cfg.DefaultReferer != "" {
			referer = cfg.DefaultReferer
		}
	}

	// 域名白名单覆盖
	if cfg != nil && media != nil && media.StreamURL != "" {
		if u, err := url.Parse(media.StreamURL); err == nil {
			host := strings.ToLower(u.Hostname())
			for dom, v := range cfg.DomainUserAgents {
				if strings.Contains(host, strings.ToLower(dom)) && v != "" {
					ua = v
					break
				}
			}
			for dom, v := range cfg.DomainReferers {
				if strings.Contains(host, strings.ToLower(dom)) && v != "" {
					referer = v
					break
				}
			}
		}
	}

	// media 自身字段（最高优先级）
	if media != nil {
		if strings.TrimSpace(media.StreamUA) != "" {
			ua = media.StreamUA
		}
		if strings.TrimSpace(media.StreamReferer) != "" {
			referer = media.StreamReferer
		}
		cookie = media.StreamCookie
		if hs := DecodeSTRMHeaders(media.StreamHeaders); hs != nil {
			for k, v := range hs {
				headers[k] = v
			}
		}
	}
	return
}

// RemoteProbeSTRM 对直链型 STRM（mp4/mkv 等）异步调用 FFprobe 获取时长/编码/分辨率
// 不抛错，最大努力。超时由调用方上下文控制。
func RemoteProbeSTRM(ctx context.Context, ffprobePath string, media *model.Media, headers http.Header, timeoutSec int) bool {
	if media == nil || media.StreamURL == "" {
		return false
	}
	if timeoutSec <= 0 {
		timeoutSec = 8
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := []string{
		"-v", "quiet",
		"-hide_banner",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
	}
	// -user_agent / -headers 让 FFprobe 在拉流时携带鉴权
	if ua := headers.Get("User-Agent"); ua != "" {
		args = append(args, "-user_agent", ua)
	}
	// FFmpeg 要求 -headers 里每行以 \r\n 结尾
	var hb strings.Builder
	for k, vs := range headers {
		if strings.EqualFold(k, "User-Agent") {
			continue
		}
		for _, v := range vs {
			hb.WriteString(k)
			hb.WriteString(": ")
			hb.WriteString(v)
			hb.WriteString("\r\n")
		}
	}
	if hb.Len() > 0 {
		args = append(args, "-headers", hb.String())
	}
	args = append(args, "-i", media.StreamURL)

	out, err := exec.CommandContext(probeCtx, ffprobePath, args...).Output()
	if err != nil || len(out) == 0 {
		return false
	}
	var result FFprobeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return false
	}
	// 写入 media
	for _, st := range result.Streams {
		switch st.CodecType {
		case "video":
			if media.VideoCodec == "" || strings.HasPrefix(media.VideoCodec, "strm_") {
				media.VideoCodec = st.CodecName
			}
			if st.Width > 0 && st.Height > 0 {
				// 简单分类
				h := st.Height
				switch {
				case h >= 2000:
					media.Resolution = "4K"
				case h >= 1000:
					media.Resolution = "1080p"
				case h >= 700:
					media.Resolution = "720p"
				case h >= 400:
					media.Resolution = "480p"
				default:
					media.Resolution = "SD"
				}
			}
		case "audio":
			if media.AudioCodec == "" {
				media.AudioCodec = st.CodecName
			}
		}
	}
	if result.Format.Duration != "" {
		var d float64
		fmt.Sscanf(result.Format.Duration, "%f", &d)
		if d > 0 {
			media.Duration = d
		}
	}
	return true
}
