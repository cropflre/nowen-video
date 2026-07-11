package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
)

// NormalizeTMDbNetworkProxy 校验 HTTP/SOCKS 网络出口代理。
func NormalizeTMDbNetworkProxy(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("代理地址解析失败: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", fmt.Errorf("仅支持 http://、https://、socks5:// 或 socks5h://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("代理地址缺少主机或端口")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("网络代理地址不能包含查询参数或片段")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("网络代理地址不能包含路径")
	}
	parsed.Scheme = scheme
	parsed.Path = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// tmdbNetworkProxyFunc 每次请求都读取最新配置，保存后无需重启。
func tmdbNetworkProxyFunc(cfg *config.Config) func(*http.Request) (*url.URL, error) {
	return func(_ *http.Request) (*url.URL, error) {
		normalized, err := NormalizeTMDbNetworkProxy(cfg.GetTMDbNetworkProxy())
		if err != nil || normalized == "" {
			return nil, err
		}
		return url.Parse(normalized)
	}
}

// PingTMDbRouting 测试反向代理 Base URL 与网络出口代理组成的完整链路。
func (s *MetadataService) PingTMDbRouting(apiBase, imageBase, networkProxy string) (bool, string, bool, string) {
	client, err := newTMDbRoutingClient(networkProxy)
	if err != nil {
		msg := "网络代理配置无效: " + err.Error()
		return false, msg, false, msg
	}
	apiTarget := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if apiTarget == "" {
		apiTarget = "https://api.themoviedb.org"
	}
	imageTarget := strings.TrimRight(strings.TrimSpace(imageBase), "/")
	if imageTarget == "" {
		imageTarget = "https://image.tmdb.org"
	}
	apiOK, apiMsg := probeTMDbAPI(client, apiTarget)
	imageOK, imageMsg := probeTMDbImage(client, imageTarget)
	return apiOK, apiMsg, imageOK, imageMsg
}

func newTMDbRoutingClient(networkProxy string) (*http.Client, error) {
	normalized, err := NormalizeTMDbNetworkProxy(networkProxy)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if normalized != "" {
		parsed, _ := url.Parse(normalized)
		transport.Proxy = http.ProxyURL(parsed)
	}
	return &http.Client{Timeout: 8 * time.Second, Transport: transport}, nil
}

func probeTMDbAPI(client *http.Client, base string) (bool, string) {
	req, err := http.NewRequest(http.MethodGet, base+"/3/configuration?api_key=ping", nil)
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	setAPIHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return false, "无法连通: " + err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
		return true, fmt.Sprintf("可达（HTTP %d，%s）", resp.StatusCode, base)
	}
	return false, fmt.Sprintf("HTTP %d（%s）", resp.StatusCode, base)
}

func probeTMDbImage(client *http.Client, base string) (bool, string) {
	target := base + "/t/p/w92/wwemzKWzjKYJFfCeiB57q3r4Bcm.png"
	req, _ := http.NewRequest(http.MethodHead, target, nil)
	setAPIHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return false, "无法连通: " + err.Error()
	}
	status := resp.StatusCode
	resp.Body.Close()
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		req, _ = http.NewRequest(http.MethodGet, target, nil)
		setAPIHeaders(req)
		if resp, err = client.Do(req); err == nil {
			status = resp.StatusCode
			resp.Body.Close()
		}
	}
	if status == http.StatusOK || (status >= 300 && status < 400) {
		return true, fmt.Sprintf("可达（HTTP %d，%s）", status, base)
	}
	return false, fmt.Sprintf("HTTP %d（%s）", status, base)
}

func proxyDisplayURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || raw == "" {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}
