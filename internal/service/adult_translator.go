// Package service 番号元数据翻译服务
// 借鉴自 mdcx-master 的 translate 模块，支持多种翻译提供商
// 支持：Google / DeepLX / 百度 / 有道 / disabled（不翻译）
package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ==================== 翻译入口 ====================

// TranslateAdultMetadata 翻译番号元数据的日文字段为中文
// 翻译：Title、Plot（保留原始值到 OriginalTitle、OriginalPlot）
func (s *AdultScraperService) TranslateAdultMetadata(meta *AdultMetadata) {
	if meta == nil {
		return
	}
	if !s.cfg.AdultScraper.EnableTranslate {
		return
	}
	provider := strings.ToLower(strings.TrimSpace(s.cfg.AdultScraper.TranslateProvider))
	if provider == "" || provider == "disabled" {
		return
	}

	targetLang := s.cfg.AdultScraper.TranslateTargetLang
	if targetLang == "" {
		targetLang = "zh-CN"
	}

	// 标题翻译（保留原始标题）
	if meta.Title != "" && !isChineseText(meta.Title) {
		if translated, err := s.translateText(meta.Title, "auto", targetLang, provider); err == nil && translated != "" {
			if meta.OriginalTitle == "" {
				meta.OriginalTitle = meta.Title
			}
			meta.Title = translated
		} else if err != nil {
			s.logger.Debugf("标题翻译失败: %v", err)
		}
	}

	// 简介翻译
	if meta.Plot != "" && !isChineseText(meta.Plot) {
		if translated, err := s.translateText(meta.Plot, "auto", targetLang, provider); err == nil && translated != "" {
			if meta.OriginalPlot == "" {
				meta.OriginalPlot = meta.Plot
			}
			meta.Plot = translated
		} else if err != nil {
			s.logger.Debugf("简介翻译失败: %v", err)
		}
	}
}

// translateText 根据 provider 分发到具体的翻译实现
func (s *AdultScraperService) translateText(text, sourceLang, targetLang, provider string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	switch provider {
	case "google":
		return s.translateByGoogle(text, sourceLang, targetLang)
	case "deeplx":
		return s.translateByDeeplx(text, sourceLang, targetLang)
	case "baidu":
		return s.translateByBaidu(text, sourceLang, targetLang)
	case "youdao":
		return s.translateByYoudao(text, targetLang)
	default:
		return "", fmt.Errorf("不支持的翻译服务: %s", provider)
	}
}

// ==================== Google 翻译（免费接口）====================

// translateByGoogle 使用 Google 翻译免费接口
// 注：需要访问 translate.googleapis.com（国内需代理）
func (s *AdultScraperService) translateByGoogle(text, sourceLang, targetLang string) (string, error) {
	if sourceLang == "auto" {
		sourceLang = "auto"
	}
	// 转换语言代码
	targetLang = mapGoogleLang(targetLang)

	apiURL := "https://translate.googleapis.com/translate_a/single"
	params := url.Values{}
	params.Set("client", "gtx")
	params.Set("sl", sourceLang)
	params.Set("tl", targetLang)
	params.Set("dt", "t")
	params.Set("q", text)

	req, err := http.NewRequest("GET", apiURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; nowen-video/1.0)")

	resp, err := s.httpClientForTranslate().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Google 返回嵌套数组 [[["翻译后","原文",...], ...]]
	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", fmt.Errorf("解析 Google 翻译响应失败: %w", err)
	}

	var sb strings.Builder
	if len(raw) > 0 {
		for _, seg := range raw[0] {
			if arr, ok := seg.([]interface{}); ok && len(arr) > 0 {
				if translated, ok := arr[0].(string); ok {
					sb.WriteString(translated)
				}
			}
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// ==================== DeepLX 自建翻译 ====================

// translateByDeeplx 使用自建的 DeepLX 服务（https://github.com/OwO-Network/DeepLX）
// endpoint 格式：http://host:port/translate
func (s *AdultScraperService) translateByDeeplx(text, sourceLang, targetLang string) (string, error) {
	endpoint := s.cfg.AdultScraper.TranslateEndpoint
	if endpoint == "" {
		return "", fmt.Errorf("DeepLX 需要配置 translate_endpoint")
	}

	if sourceLang == "auto" {
		sourceLang = ""
	}
	targetLang = mapDeeplLang(targetLang)

	reqBody := map[string]string{
		"text":        text,
		"source_lang": sourceLang,
		"target_lang": targetLang,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey := s.cfg.AdultScraper.TranslateAPIKey; apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := s.httpClientForTranslate().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepLX HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Code         int    `json:"code"`
		Data         string `json:"data"`
		SourceLang   string `json:"source_lang"`
		TargetLang   string `json:"target_lang"`
		Alternatives []string `json:"alternatives"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Data, nil
}

// ==================== 百度翻译 ====================

// translateByBaidu 使用百度翻译 API（需要 appid/secret）
func (s *AdultScraperService) translateByBaidu(text, sourceLang, targetLang string) (string, error) {
	appid := s.cfg.AdultScraper.TranslateAPIKey
	secret := s.cfg.AdultScraper.TranslateAPISecret
	if appid == "" || secret == "" {
		return "", fmt.Errorf("百度翻译需要配置 translate_api_key 和 translate_api_secret")
	}

	if sourceLang == "auto" {
		sourceLang = "auto"
	}
	targetLang = mapBaiduLang(targetLang)

	salt := fmt.Sprintf("%d", rand.Int63())
	signRaw := appid + text + salt + secret
	h := md5.Sum([]byte(signRaw))
	sign := hex.EncodeToString(h[:])

	params := url.Values{}
	params.Set("q", text)
	params.Set("from", sourceLang)
	params.Set("to", targetLang)
	params.Set("appid", appid)
	params.Set("salt", salt)
	params.Set("sign", sign)

	apiURL := "https://fanyi-api.baidu.com/api/trans/vip/translate?" + params.Encode()
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := s.httpClientForTranslate().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		ErrorCode string `json:"error_code"`
		ErrorMsg  string `json:"error_msg"`
		TransResult []struct {
			Src string `json:"src"`
			Dst string `json:"dst"`
		} `json:"trans_result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.ErrorCode != "" {
		return "", fmt.Errorf("百度翻译错误: %s - %s", result.ErrorCode, result.ErrorMsg)
	}
	var sb strings.Builder
	for _, r := range result.TransResult {
		sb.WriteString(r.Dst)
	}
	return sb.String(), nil
}

// ==================== 有道翻译（简化实现）====================

// translateByYoudao 使用有道翻译 API（需要 API Key）
func (s *AdultScraperService) translateByYoudao(text, targetLang string) (string, error) {
	appKey := s.cfg.AdultScraper.TranslateAPIKey
	appSecret := s.cfg.AdultScraper.TranslateAPISecret
	if appKey == "" || appSecret == "" {
		return "", fmt.Errorf("有道翻译需要配置 translate_api_key 和 translate_api_secret")
	}

	targetLang = mapYoudaoLang(targetLang)
	salt := fmt.Sprintf("%d", time.Now().UnixNano())
	curtime := fmt.Sprintf("%d", time.Now().Unix())
	// 有道签名：sha256(appKey + truncateInput(q) + salt + curtime + appSecret)
	// 简化版：input 直接用 text 前 20 字符
	input := text
	if len([]rune(text)) > 20 {
		runes := []rune(text)
		input = string(runes[:10]) + fmt.Sprintf("%d", len(runes)) + string(runes[len(runes)-10:])
	}
	signRaw := appKey + input + salt + curtime + appSecret
	h := md5.Sum([]byte(signRaw))
	sign := hex.EncodeToString(h[:])

	form := url.Values{}
	form.Set("q", text)
	form.Set("from", "auto")
	form.Set("to", targetLang)
	form.Set("appKey", appKey)
	form.Set("salt", salt)
	form.Set("sign", sign)
	form.Set("signType", "v3")
	form.Set("curtime", curtime)

	req, err := http.NewRequest("POST", "https://openapi.youdao.com/api", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClientForTranslate().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		ErrorCode   string   `json:"errorCode"`
		Translation []string `json:"translation"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.ErrorCode != "0" {
		return "", fmt.Errorf("有道翻译错误: %s", result.ErrorCode)
	}
	return strings.Join(result.Translation, ""), nil
}

// ==================== 工具函数 ====================

// httpClientForTranslate 翻译专用的 HTTP 客户端
func (s *AdultScraperService) httpClientForTranslate() *http.Client {
	if s.client != nil {
		return s.client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// isChineseText 判断文本是否主要为中文（包含中文字符超过 30% 即视为中文）
func isChineseText(s string) bool {
	if s == "" {
		return false
	}
	runes := []rune(s)
	cnCount := 0
	for _, r := range runes {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			cnCount++
		}
	}
	if len(runes) == 0 {
		return false
	}
	return float64(cnCount)/float64(len(runes)) > 0.3
}

// 语言代码映射

func mapGoogleLang(lang string) string {
	switch strings.ToLower(lang) {
	case "zh-cn", "zh":
		return "zh-CN"
	case "zh-tw":
		return "zh-TW"
	case "ja":
		return "ja"
	case "en":
		return "en"
	}
	return lang
}

func mapDeeplLang(lang string) string {
	switch strings.ToLower(lang) {
	case "zh-cn", "zh", "zh-tw":
		return "ZH"
	case "ja":
		return "JA"
	case "en":
		return "EN"
	}
	return strings.ToUpper(lang)
}

func mapBaiduLang(lang string) string {
	switch strings.ToLower(lang) {
	case "zh-cn", "zh":
		return "zh"
	case "zh-tw":
		return "cht"
	case "ja":
		return "jp"
	case "en":
		return "en"
	}
	return lang
}

func mapYoudaoLang(lang string) string {
	switch strings.ToLower(lang) {
	case "zh-cn", "zh":
		return "zh-CHS"
	case "zh-tw":
		return "zh-CHT"
	case "ja":
		return "ja"
	case "en":
		return "en"
	}
	return lang
}
