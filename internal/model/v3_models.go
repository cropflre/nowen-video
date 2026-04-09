package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== V3: AI 场景识别与内容理解 ====================

// VideoChapter 视频章节（AI自动生成或手动标记）
type VideoChapter struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID     string    `json:"media_id" gorm:"index;type:text;not null"`
	Title       string    `json:"title" gorm:"type:text;not null"`    // 章节标题
	StartTime   float64   `json:"start_time"`                         // 开始时间（秒）
	EndTime     float64   `json:"end_time"`                           // 结束时间（秒）
	Description string    `json:"description" gorm:"type:text"`       // 章节描述
	SceneType   string    `json:"scene_type" gorm:"type:text"`        // 场景类型: action/dialogue/landscape/credits 等
	Confidence  float64   `json:"confidence"`                         // AI识别置信度 0-1
	Source      string    `json:"source" gorm:"type:text;default:ai"` // 来源: ai / manual
	Thumbnail   string    `json:"thumbnail" gorm:"type:text"`         // 章节缩略图路径
	CreatedAt   time.Time `json:"created_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (vc *VideoChapter) BeforeCreate(tx *gorm.DB) error {
	if vc.ID == "" {
		vc.ID = uuid.New().String()
	}
	return nil
}

// VideoHighlight 视频精彩片段
type VideoHighlight struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID   string    `json:"media_id" gorm:"index;type:text;not null"`
	Title     string    `json:"title" gorm:"type:text;not null"`    // 片段标题
	StartTime float64   `json:"start_time"`                         // 开始时间（秒）
	EndTime   float64   `json:"end_time"`                           // 结束时间（秒）
	Score     float64   `json:"score"`                              // 精彩程度评分 0-10
	Tags      string    `json:"tags" gorm:"type:text"`              // 标签，逗号分隔
	Thumbnail string    `json:"thumbnail" gorm:"type:text"`         // 精彩片段缩略图
	GifPath   string    `json:"gif_path" gorm:"type:text"`          // GIF预览路径
	Source    string    `json:"source" gorm:"type:text;default:ai"` // 来源: ai / manual
	CreatedAt time.Time `json:"created_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (vh *VideoHighlight) BeforeCreate(tx *gorm.DB) error {
	if vh.ID == "" {
		vh.ID = uuid.New().String()
	}
	return nil
}

// AIAnalysisTask AI分析任务
type AIAnalysisTask struct {
	ID          string     `json:"id" gorm:"primaryKey;type:text"`
	MediaID     string     `json:"media_id" gorm:"index;type:text;not null"`
	TaskType    string     `json:"task_type" gorm:"type:text;not null"`     // scene_detect / highlight / cover_select / chapter_gen
	Status      string     `json:"status" gorm:"type:text;default:pending"` // pending / running / completed / failed
	Progress    float64    `json:"progress"`                                // 0-100
	Result      string     `json:"result" gorm:"type:text"`                 // JSON格式的分析结果
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

// ==================== V3: AI 驱动的封面优化 ====================

// CoverCandidate 封面候选帧
type CoverCandidate struct {
	ID          string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID     string    `json:"media_id" gorm:"index;type:text;not null"`
	FrameTime   float64   `json:"frame_time"`                           // 帧时间点（秒）
	ImagePath   string    `json:"image_path" gorm:"type:text;not null"` // 候选图片路径
	Score       float64   `json:"score"`                                // AI评分 0-10
	Brightness  float64   `json:"brightness"`                           // 亮度评分
	Sharpness   float64   `json:"sharpness"`                            // 清晰度评分
	Composition float64   `json:"composition"`                          // 构图评分
	FaceCount   int       `json:"face_count"`                           // 检测到的人脸数量
	IsSelected  bool      `json:"is_selected" gorm:"default:false"`     // 是否被选为封面
	CreatedAt   time.Time `json:"created_at"`

	Media Media `json:"-" gorm:"foreignKey:MediaID"`
}

func (cc *CoverCandidate) BeforeCreate(tx *gorm.DB) error {
	if cc.ID == "" {
		cc.ID = uuid.New().String()
	}
	return nil
}
