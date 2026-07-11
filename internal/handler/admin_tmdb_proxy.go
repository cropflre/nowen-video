package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// UpdateTMDbProxyRequest 中 API/Image 是反向代理 Base URL，Network 是网络出口代理。
type UpdateTMDbProxyRequest struct {
	APIProxy     string `json:"api_proxy"`
	ImageProxy   string `json:"image_proxy"`
	NetworkProxy string `json:"network_proxy"`
}

func validateTMDbProxyURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("地址解析失败: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("反向代理 Base URL 必须以 http:// 或 https:// 开头")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("反向代理 Base URL 缺少主机")
	}
	return strings.TrimRight(value, "/"), nil
}

func (h *AdminHandler) UpdateTMDbProxy(c *gin.Context) {
	var req UpdateTMDbProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	apiProxy, err := validateTMDbProxyURL(req.APIProxy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API 反向代理无效: " + err.Error()})
		return
	}
	imageProxy, err := validateTMDbProxyURL(req.ImageProxy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片反向代理无效: " + err.Error()})
		return
	}
	networkProxy, err := service.NormalizeTMDbNetworkProxy(req.NetworkProxy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "HTTP/SOCKS 网络代理无效: " + err.Error()})
		return
	}
	for _, item := range []struct {
		name  string
		value string
		set   func(string) error
	}{
		{"API 反向代理", apiProxy, h.cfg.SetTMDbAPIProxy},
		{"图片反向代理", imageProxy, h.cfg.SetTMDbImageProxy},
		{"网络出口代理", networkProxy, h.cfg.SetTMDbNetworkProxy},
	} {
		if err := item.set(item.value); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存" + item.name + "失败: " + err.Error()})
			return
		}
	}
	h.logger.Infof("TMDb 连接配置已更新 (api_base=%q, image_base=%q, network=%q)", apiProxy, imageProxy, displayProxyURL(networkProxy))
	c.JSON(http.StatusOK, gin.H{"message": "TMDb 连接配置已保存，立即生效", "data": gin.H{
		"api_proxy": apiProxy, "image_proxy": imageProxy, "network_proxy": networkProxy,
	}})
}

func (h *AdminHandler) ClearTMDbProxy(c *gin.Context) {
	for _, setter := range []func(string) error{h.cfg.SetTMDbAPIProxy, h.cfg.SetTMDbImageProxy, h.cfg.SetTMDbNetworkProxy} {
		if err := setter(""); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复官方直连失败: " + err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "已恢复 TMDb 官方直连", "data": gin.H{
		"api_proxy": "", "image_proxy": "", "network_proxy": "",
	}})
}

type TestTMDbProxyRequest struct {
	APIProxy     *string `json:"api_proxy"`
	ImageProxy   *string `json:"image_proxy"`
	NetworkProxy *string `json:"network_proxy"`
}

func (h *AdminHandler) TestTMDbProxy(c *gin.Context) {
	var req TestTMDbProxyRequest
	_ = c.ShouldBindJSON(&req)
	apiProxy := optionalProxy(req.APIProxy, h.cfg.GetTMDbAPIProxy())
	imageProxy := optionalProxy(req.ImageProxy, h.cfg.GetTMDbImageProxy())
	networkProxy := optionalProxy(req.NetworkProxy, h.cfg.GetTMDbNetworkProxy())
	var err error
	if apiProxy, err = validateTMDbProxyURL(apiProxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "API 反向代理无效: " + err.Error()})
		return
	}
	if imageProxy, err = validateTMDbProxyURL(imageProxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片反向代理无效: " + err.Error()})
		return
	}
	if networkProxy, err = service.NormalizeTMDbNetworkProxy(networkProxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "HTTP/SOCKS 网络代理无效: " + err.Error()})
		return
	}
	apiOK, apiMsg, imageOK, imageMsg := h.metadataService.PingTMDbRouting(apiProxy, imageProxy, networkProxy)
	networkOK := networkProxy == "" || apiOK || imageOK
	networkMsg := "未启用网络出口代理，使用直接连接"
	if networkProxy != "" {
		if networkOK {
			networkMsg = "HTTP/SOCKS 网络代理已建立"
		} else {
			networkMsg = "无法通过该网络代理连接 TMDb"
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"api":     gin.H{"ok": apiOK, "message": apiMsg, "target": fallback(apiProxy, "https://api.themoviedb.org")},
		"image":   gin.H{"ok": imageOK, "message": imageMsg, "target": fallback(imageProxy, "https://image.tmdb.org")},
		"network": gin.H{"ok": networkOK, "configured": networkProxy != "", "message": networkMsg, "target": fallback(displayProxyURL(networkProxy), "官方直连")},
	}})
}

func optionalProxy(value *string, current string) string {
	if value == nil {
		return strings.TrimSpace(current)
	}
	return strings.TrimSpace(*value)
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func displayProxyURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || raw == "" {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}
