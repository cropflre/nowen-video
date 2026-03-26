package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScrapeTask 刮削任务记录
type ScrapeTask struct {
	ID        string `json:"id" gorm:"primaryKey;type:text"`
	URL       string `json:"url" gorm:"type:text;not null"`             // 刮削源URL
	Source    string `json:"source" gorm:"type:text;not null"`          // 数据源: tmdb / douban / imdb / bangumi / url
	Title     string `json:"title" gorm:"type:text"`                    // 识别到的标题
	MediaType string `json:"media_type" gorm:"type:text;default:movie"` // movie / tvshow
	Status    string `json:"status" gorm:"type:text;default:pending"`   // pending / scraping / scraped / failed / translating / completed
	Progress  int    `json:"progress" gorm:"default:0"`                 // 进度 0-100
	MediaID   string `json:"media_id" gorm:"type:text"`                 // 关联的媒体ID（如果已匹配）
	SeriesID  string `json:"series_id" gorm:"type:text"`                // 关联的剧集ID
	// 刮削结果
	ResultTitle     string  `json:"result_title" gorm:"type:text"`
	ResultOrigTitle string  `json:"result_orig_title" gorm:"type:text"`
	ResultYear      int     `json:"result_year"`
	ResultOverview  string  `json:"result_overview" gorm:"type:text"`
	ResultGenres    string  `json:"result_genres" gorm:"type:text"`
	ResultRating    float64 `json:"result_rating"`
	ResultPoster    string  `json:"result_poster" gorm:"type:text"`
	ResultCountry   string  `json:"result_country" gorm:"type:text"`
	ResultLanguage  string  `json:"result_language" gorm:"type:text"`
	// 翻译状态
	TranslateStatus string `json:"translate_status" gorm:"type:text;default:none"` // none / pending / translating / done / failed
	TranslateLang   string `json:"translate_lang" gorm:"type:text"`                // 目标翻译语言
	// 翻译结果
	TranslatedTitle    string `json:"translated_title" gorm:"type:text"`
	TranslatedOverview string `json:"translated_overview" gorm:"type:text"`
	TranslatedGenres   string `json:"translated_genres" gorm:"type:text"`
	TranslatedTagline  string `json:"translated_tagline" gorm:"type:text"`
	// 数据质量评分 (0-100)
	QualityScore int    `json:"quality_score" gorm:"default:0"`
	ErrorMessage string `json:"error_message" gorm:"type:text"`
	// 操作记录
	CreatedBy string    `json:"created_by" gorm:"type:text"` // 创建者用户ID
	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *ScrapeTask) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// ScrapeHistory 刮削操作历史记录
type ScrapeHistory struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	TaskID    string    `json:"task_id" gorm:"index;type:text;not null"`
	Action    string    `json:"action" gorm:"type:text;not null"` // created / scrape_start / scrape_done / scrape_fail / translate_start / translate_done / translate_fail / edited / deleted / exported
	Detail    string    `json:"detail" gorm:"type:text"`
	UserID    string    `json:"user_id" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

func (h *ScrapeHistory) BeforeCreate(tx *gorm.DB) error {
	if h.ID == "" {
		h.ID = uuid.New().String()
	}
	return nil
}
