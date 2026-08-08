package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 扫描后处理：虚拟归类与命名映射（仅 DB 层） ====================
//
// 该模型由 ScanPostProcess 流水线产出，与 Media 1:1（unique on MediaID）。
//
// 安全约束：模型语义上只表达"如果按 Jellyfin/Emby 风格规范，文件应该叫什么 / 放在哪个虚拟分类下"，
// 全程仅写入数据库；服务层/接口层禁止任何磁盘改名、移动、新建目录的物理操作。
//
// 三阶段产出：
//   阶段 1 · 识别（identify）   - 标题/年份/TMDb/IMDb/置信度，必要时调用 AI Fallback
//   阶段 2 · 归类（classify）   - 类别/地区/年代/类型标签/质量档/虚拟路径
//   阶段 3 · 命名（naming）     - 仅 DB 中保存的「建议命名」与「建议子目录」
//
// 字段命名与 Media 字段保持一致风格，便于前端联表展示。

// 状态枚举
const (
	ClassificationStatusPending   = "pending"   // 入队等待
	ClassificationStatusRunning   = "running"   // 处理中
	ClassificationStatusProcessed = "processed" // 完成（识别+归类+命名 全部成功）
	ClassificationStatusPartial   = "partial"   // 部分成功（如 AI 失败但规则识别可用）
	ClassificationStatusFailed    = "failed"    // 完全失败
	ClassificationStatusSkipped   = "skipped"   // 跳过（例如非视频媒体）
)

// 命名风格枚举（与 SmartRename 保持一致）
const (
	ClassificationNamingJellyfin = "jellyfin"
	ClassificationNamingPlex     = "plex"
)

// MediaClassification 媒体扫描后处理产出
//
// 规则：
//  1. MediaID 唯一索引，每条 Media 对应一条记录（不存在或被删除时新建）。
//  2. 当 Media 软删除/硬删除时，本表记录可由调用方按需级联清理（保留以便审计）。
//  3. 不持有 Media 实体的字段，避免冗余；如需展示，前端联表查询。
type MediaClassification struct {
	ID        string `json:"id" gorm:"primaryKey;type:text"`
	MediaID   string `json:"media_id" gorm:"uniqueIndex;type:text;not null"`
	LibraryID string `json:"library_id" gorm:"index;type:text"`

	// =========== 阶段 1：识别（来自 filename_parser + 可选 AI Fallback） ===========
	ParsedTitle    string  `json:"parsed_title" gorm:"type:text"`
	ParsedTitleAlt string  `json:"parsed_title_alt" gorm:"type:text"`
	ParsedYear     int     `json:"parsed_year"`
	ParsedTMDbID   int     `json:"parsed_tmdb_id" gorm:"index"`
	ParsedIMDbID   string  `json:"parsed_imdb_id" gorm:"index;type:text"`
	Confidence     float64 `json:"confidence"`                       // 综合置信度（0-1）
	AIInvoked      bool    `json:"ai_invoked"`                       // 是否调用了 AI Fallback
	AIProvider     string  `json:"ai_provider" gorm:"type:text"`     // AI 服务商（来自 AI 配置当前生效项，例如 openai/deepseek/anthropic）
	AIModel        string  `json:"ai_model" gorm:"type:text"`        // AI 模型名（如 gpt-4o-mini / deepseek-chat）
	AIRawResponse  string  `json:"ai_raw_response" gorm:"type:text"` // AI 原始响应（JSON 字符串）

	// =========== 阶段 2：虚拟归类（按规则推导，仅 DB 标签） ===========
	Category    string `json:"category" gorm:"index;type:text"`     // movie/tvshow/anime/documentary/variety/music/adult/other
	Region      string `json:"region" gorm:"index;type:text"`       // CN/HK/TW/JP/KR/US/EU/IN/OTHER
	Decade      string `json:"decade" gorm:"index;type:text"`       // 2020s/2010s/2000s/1990s/...
	GenreTags   string `json:"genre_tags" gorm:"type:text"`         // 规范化后的逗号分隔类型
	LanguageTag string `json:"language_tag" gorm:"type:text"`       // zh/ja/en/...
	QualityTier string `json:"quality_tier" gorm:"type:text"`       // 4K/1080p/720p/SD
	VirtualPath string `json:"virtual_path" gorm:"index;type:text"` // 虚拟路径，如 "电影/华语/2020s/科幻"

	// =========== 阶段 3：Jellyfin/Emby 风格命名建议（仅记录） ===========
	SuggestedName     string `json:"suggested_name" gorm:"type:text"`      // 例：流浪地球 (2019) [tmdbid-12345].mkv
	SuggestedDir      string `json:"suggested_dir" gorm:"type:text"`       // 例：流浪地球 (2019)
	SuggestedFullPath string `json:"suggested_full_path" gorm:"type:text"` // libraryRoot/SuggestedDir/SuggestedName
	NamingStyle       string `json:"naming_style" gorm:"type:text"`        // jellyfin/plex

	// =========== 流程状态 ===========
	Status      string     `json:"status" gorm:"index;type:text;default:pending"`
	ErrorMsg    string     `json:"error_msg" gorm:"type:text"`
	ProcessedAt *time.Time `json:"processed_at"`

	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式指定表名，避免与未来可能的"分类"业务概念冲突
func (MediaClassification) TableName() string {
	return "media_classifications"
}

// BeforeCreate 自动生成 UUID
func (c *MediaClassification) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
