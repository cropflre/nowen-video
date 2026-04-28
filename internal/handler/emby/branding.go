package emby

// branding.go 提供 Jellyfin/Emby 客户端加载登录页与本地化信息所需的小型元数据端点。
//
// 这些端点不影响登录链路本身，但若缺失部分 Jellyfin 官方客户端会展示 Toast 错误，
// 影响体验。实现上全部是轻量常量/配置读取，性能开销可忽略。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BrandingConfigHandler 对应 /Branding/Configuration。
// Jellyfin 客户端用它在登录页渲染欢迎语与登录免责声明。
func (h *Handler) BrandingConfigHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"LoginDisclaimer":     h.cfg.Emby.LoginDisclaimer,
		"CustomCss":           h.cfg.Emby.CustomCss,
		"SplashscreenEnabled": false,
	})
}

// BrandingCssHandler 对应 /Branding/Css 与 /Branding/Css.css。
// 仅 Jellyfin Web 客户端使用；原生 iOS/Android 不会请求。
func (h *Handler) BrandingCssHandler(c *gin.Context) {
	c.Header("Content-Type", "text/css; charset=utf-8")
	c.String(http.StatusOK, h.cfg.Emby.CustomCss)
}

// SystemConfigurationHandler 对应 /System/Configuration。
// 返回一个 Emby 客户端能接受的最小配置体；Infuse 不依赖此端点，
// 某些 Jellyfin 客户端登录时会 GET 一次。
func (h *Handler) SystemConfigurationHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"EnableCaseSensitiveItemIds":          true,
		"EnableMetrics":                       false,
		"IsStartupWizardCompleted":            true,
		"MetadataPath":                        "",
		"MetadataNetworkPath":                 "",
		"PreferredMetadataLanguage":           "zh",
		"MetadataCountryCode":                 "CN",
		"SortReplaceCharacters":               []string{".", "+", "%"},
		"SortRemoveCharacters":                []string{",", "&", "-", "{", "}", "'"},
		"SortRemoveWords":                     []string{"the", "a", "an"},
		"MinResumePct":                        5,
		"MaxResumePct":                        90,
		"MinResumeDurationSeconds":            300,
		"UICulture":                           "zh-CN",
		"EnableDashboardResponseCaching":      true,
		"EnableExternalContentInSuggestions":  false,
		"RequireHttps":                        false,
		"EnableNewOmdbSupport":                false,
		"DisableLiveTvChannelUserDataName":    true,
		"EnableFolderView":                    false,
		"EnableGroupingIntoCollections":       false,
		"DisplaySpecialsWithinSeasons":        true,
		"LocalNetworkSubnets":                 []string{},
		"LocalNetworkAddresses":               []string{},
		"CodecsUsed":                          []string{},
		"IgnoreVirtualInterfaces":             true,
		"EnableExternalServiceDiscovery":      true,
		"CachePath":                           "",
	})
}

// LocalizationCulturesHandler 对应 /Localization/Cultures。
// Jellyfin 客户端在音频/字幕选择页面用它把语言代码翻译成用户可读名。
// 返回一个常用语言列表即可。
func (h *Handler) LocalizationCulturesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, defaultCultures())
}

// LocalizationCountriesHandler 对应 /Localization/Countries。
func (h *Handler) LocalizationCountriesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, defaultCountries())
}

// LocalizationOptionsHandler 对应 /Localization/Options。
// 返回 UI 可选语言。
func (h *Handler) LocalizationOptionsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{"Name": "简体中文", "Value": "zh-CN"},
		{"Name": "English", "Value": "en-US"},
		{"Name": "繁體中文", "Value": "zh-TW"},
		{"Name": "日本語", "Value": "ja-JP"},
	})
}

// LocalizationParentalRatingsHandler 对应 /Localization/ParentalRatings。
func (h *Handler) LocalizationParentalRatingsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

// QuickConnectEnabledHandler 对应 /QuickConnect/Enabled。
// 返回 false 表示不支持 Jellyfin 的扫码登录；客户端会隐藏相关按钮。
func (h *Handler) QuickConnectEnabledHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.String(http.StatusOK, "false")
}

// defaultCultures 返回常用语言的 Emby 风格 CultureDto 列表。
func defaultCultures() []gin.H {
	return []gin.H{
		{"Name": "Chinese", "DisplayName": "中文", "TwoLetterISOLanguageName": "zh", "ThreeLetterISOLanguageName": "zho", "ThreeLetterISOLanguageNames": []string{"zho", "chi"}},
		{"Name": "English", "DisplayName": "英语", "TwoLetterISOLanguageName": "en", "ThreeLetterISOLanguageName": "eng", "ThreeLetterISOLanguageNames": []string{"eng"}},
		{"Name": "Japanese", "DisplayName": "日语", "TwoLetterISOLanguageName": "ja", "ThreeLetterISOLanguageName": "jpn", "ThreeLetterISOLanguageNames": []string{"jpn"}},
		{"Name": "Korean", "DisplayName": "韩语", "TwoLetterISOLanguageName": "ko", "ThreeLetterISOLanguageName": "kor", "ThreeLetterISOLanguageNames": []string{"kor"}},
		{"Name": "French", "DisplayName": "法语", "TwoLetterISOLanguageName": "fr", "ThreeLetterISOLanguageName": "fra", "ThreeLetterISOLanguageNames": []string{"fra", "fre"}},
		{"Name": "German", "DisplayName": "德语", "TwoLetterISOLanguageName": "de", "ThreeLetterISOLanguageName": "deu", "ThreeLetterISOLanguageNames": []string{"deu", "ger"}},
		{"Name": "Spanish", "DisplayName": "西班牙语", "TwoLetterISOLanguageName": "es", "ThreeLetterISOLanguageName": "spa", "ThreeLetterISOLanguageNames": []string{"spa"}},
		{"Name": "Russian", "DisplayName": "俄语", "TwoLetterISOLanguageName": "ru", "ThreeLetterISOLanguageName": "rus", "ThreeLetterISOLanguageNames": []string{"rus"}},
		{"Name": "Italian", "DisplayName": "意大利语", "TwoLetterISOLanguageName": "it", "ThreeLetterISOLanguageName": "ita", "ThreeLetterISOLanguageNames": []string{"ita"}},
		{"Name": "Portuguese", "DisplayName": "葡萄牙语", "TwoLetterISOLanguageName": "pt", "ThreeLetterISOLanguageName": "por", "ThreeLetterISOLanguageNames": []string{"por"}},
	}
}

// defaultCountries 返回常用国家/地区列表。
func defaultCountries() []gin.H {
	return []gin.H{
		{"Name": "China", "DisplayName": "中国", "TwoLetterISORegionName": "CN", "ThreeLetterISORegionName": "CHN"},
		{"Name": "Hong Kong", "DisplayName": "中国香港", "TwoLetterISORegionName": "HK", "ThreeLetterISORegionName": "HKG"},
		{"Name": "Taiwan", "DisplayName": "中国台湾", "TwoLetterISORegionName": "TW", "ThreeLetterISORegionName": "TWN"},
		{"Name": "United States", "DisplayName": "美国", "TwoLetterISORegionName": "US", "ThreeLetterISORegionName": "USA"},
		{"Name": "Japan", "DisplayName": "日本", "TwoLetterISORegionName": "JP", "ThreeLetterISORegionName": "JPN"},
		{"Name": "Korea", "DisplayName": "韩国", "TwoLetterISORegionName": "KR", "ThreeLetterISORegionName": "KOR"},
		{"Name": "United Kingdom", "DisplayName": "英国", "TwoLetterISORegionName": "GB", "ThreeLetterISORegionName": "GBR"},
		{"Name": "France", "DisplayName": "法国", "TwoLetterISORegionName": "FR", "ThreeLetterISORegionName": "FRA"},
		{"Name": "Germany", "DisplayName": "德国", "TwoLetterISORegionName": "DE", "ThreeLetterISORegionName": "DEU"},
	}
}
