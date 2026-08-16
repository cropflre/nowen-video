package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== V3: 场景识别与本地媒体分析 ====================

// VideoChapter 视频章节（自动生成或手动标记）。
type VideoChapter struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID     string    `json:"media_id" gorm:"index;type:text;not null"`
	Title       string    `json:"title" gorm:"type:text;not null"`
	StartTime   float64   `json:"start_time"`
	EndTime     float64   `json:"end_time"`
	Description string    `json:"description" gorm:"type:text"`
	SceneType   string    `json:"scene_type" gorm:"type:text"`
	Confidence  float64   `json:"confidence"`
	Source      string    `json:"source" gorm:"type:text;default:analysis"` // analysis / ai / manual
	Thumbnail   string    `json:"thumbnail" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (vc *VideoChapter) BeforeCreate(tx *gorm.DB) error {
	if vc.ID == "" {
		vc.ID = uuid.New().String()
	}
	return nil
}

// VideoHighlight 视频精彩片段。
// 片段仅保存原媒体的时间范围和轻量预览资产，不复制生成独立视频文件。
type VideoHighlight struct {
	ID             string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID        string    `json:"media_id" gorm:"index;type:text;not null"`
	Title          string    `json:"title" gorm:"type:text;not null"`
	StartTime      float64   `json:"start_time"`
	EndTime        float64   `json:"end_time"`
	Score          float64   `json:"score"`
	Tags           string    `json:"tags" gorm:"type:text"`
	Thumbnail      string    `json:"thumbnail" gorm:"type:text"`
	GifPath        string    `json:"gif_path" gorm:"type:text"` // 旧字段，保留兼容
	PreviewPath    string    `json:"preview_path" gorm:"type:text"`
	Source         string    `json:"source" gorm:"type:text;default:ffmpeg"` // ffmpeg / manual / ai(legacy)
	AnalysisMethod string    `json:"analysis_method" gorm:"type:text"`       // audio_scene / scene / heuristic
	Fingerprint    string    `json:"fingerprint" gorm:"type:text;index"`
	Version        int       `json:"version" gorm:"default:1"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (vh *VideoHighlight) BeforeCreate(tx *gorm.DB) error {
	if vh.ID == "" {
		vh.ID = uuid.New().String()
	}
	if vh.Version <= 0 {
		vh.Version = 1
	}
	if vh.Source == "" {
		vh.Source = "ffmpeg"
	}
	return nil
}

// AIAnalysisTask 是历史表名对应的数据模型。
// Lite 的本地 Media Analysis 也复用这张持久化任务表，TaskType/Stage 用于区分非 AI 任务。
type AIAnalysisTask struct {
	ID          string     `json:"id" gorm:"primaryKey;type:text"`
	MediaID     string     `json:"media_id" gorm:"index;type:text;not null"`
	TaskType    string     `json:"task_type" gorm:"type:text;not null;index"` // media_highlight / scene_detect / highlight(legacy) ...
	Status      string     `json:"status" gorm:"type:text;default:pending;index"`
	Stage       string     `json:"stage" gorm:"type:text;index"`
	Progress    float64    `json:"progress"`
	Result      string     `json:"result" gorm:"type:text"`
	Error       string     `json:"error" gorm:"type:text"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (at *AIAnalysisTask) BeforeCreate(tx *gorm.DB) error {
	if at.ID == "" {
		at.ID = uuid.New().String()
	}
	return nil
}

// ==================== V3: 封面候选 ====================

type CoverCandidate struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID     string    `json:"media_id" gorm:"index;type:text;not null"`
	FrameTime   float64   `json:"frame_time"`
	ImagePath   string    `json:"image_path" gorm:"type:text;not null"`
	Score       float64   `json:"score"`
	Brightness  float64   `json:"brightness"`
	Sharpness   float64   `json:"sharpness"`
	Composition float64   `json:"composition"`
	FaceCount   int       `json:"face_count"`
	IsSelected  bool      `json:"is_selected" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (cc *CoverCandidate) BeforeCreate(tx *gorm.DB) error {
	if cc.ID == "" {
		cc.ID = uuid.New().String()
	}
	return nil
}
