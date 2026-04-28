package service

import (
	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

// ==================== AdultProvider：成人内容数据源适配器 ====================
// 实现 MetadataProvider 接口，注册到 ProviderChain 中
// 仅当媒体被识别为成人内容时激活（通过文件名番号特征判断）

// AdultProvider 成人内容元数据数据源适配器
type AdultProvider struct {
	scraper *AdultScraperService
	logger  *zap.SugaredLogger
}

// NewAdultProvider 创建成人内容 Provider
func NewAdultProvider(scraper *AdultScraperService) *AdultProvider {
	return &AdultProvider{
		scraper: scraper,
		logger:  scraper.logger,
	}
}

// Name 返回数据源名称
func (p *AdultProvider) Name() string { return "Adult" }

// IsEnabled 检查数据源是否可用
func (p *AdultProvider) IsEnabled() bool {
	return p.scraper != nil && p.scraper.IsEnabled()
}

// Priority 返回数据源优先级
// 设置为 5，比 TMDb（10）更高，因为对于番号内容，TMDb 基本无法匹配
// 番号内容应优先使用专业数据源
func (p *AdultProvider) Priority() int { return 5 }

// SupportedTypes 返回支持的媒体类型
func (p *AdultProvider) SupportedTypes() []string { return []string{"adult"} }

// ScrapeMedia 为单个媒体刮削元数据
func (p *AdultProvider) ScrapeMedia(media *model.Media, searchTitle string, year int, mode string) error {
	// 仅处理成人内容
	if !IsAdultContent(media) {
		return nil // 非成人内容，静默跳过
	}

	// 从文件名中提取番号
	code, codeType := ParseCode(media.FilePath)
	if code == "" {
		// 尝试从标题中提取
		code, codeType = ParseCode(media.Title)
	}
	if code == "" {
		return nil // 无法识别番号，跳过
	}

	p.logger.Infof("识别到番号 [%s] (类型: %s)，开始刮削: %s", code, codeType, media.FilePath)

	// 调用混合刮削引擎
	meta, err := p.scraper.ScrapeByCode(code)
	if err != nil {
		return err
	}

	// 应用元数据到 Media
	return p.scraper.ApplyToMedia(media, meta, mode)
}

// ScrapeSeries 为剧集合集刮削元数据（成人内容通常不是剧集，直接跳过）
func (p *AdultProvider) ScrapeSeries(series *model.Series, searchTitle string, year int, mode string) error {
	// 成人内容通常不以剧集形式存在，直接返回
	return nil
}
