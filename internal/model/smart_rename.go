package model

import (
	"time"

	"gorm.io/gorm"
)

// ==================== SmartRename 智能扫描重命名 ====================
//
// 该子系统独立于 FileManager 的 PreviewRename / ExecuteRename（后者仅改 DB Title）。
// 这里提供：
//   1. 扫描 -> 识别评分 -> 命名规划（plan）-> 安全检测 -> 执行（dry-run / confirm 落盘）
//   2. 全程 plan + journal 双写，保留回滚能力
//
// 设计要点：
//   - RenamePlan：一次扫描会话/规划任务，含整体状态、配置、统计
//   - RenamePlanItem：单条文件改名条目，含置信度、AI 是否介入、命名结果、附属文件
//   - RenameJournal：实际落盘后的日志（每条物理操作一条），用于回滚和审计

// RenamePlanStatus 智能改名规划任务整体状态
type RenamePlanStatus string

const (
	RenamePlanStatusDraft      RenamePlanStatus = "draft"      // 扫描完成、AI 完成、安全检测完成，待用户确认
	RenamePlanStatusExecuting  RenamePlanStatus = "executing"  // 执行中（确认后落盘阶段）
	RenamePlanStatusCompleted  RenamePlanStatus = "completed"  // 已落盘
	RenamePlanStatusFailed     RenamePlanStatus = "failed"     // 落盘过程出错（部分回滚）
	RenamePlanStatusRolledBack RenamePlanStatus = "rolledback" // 已手动回滚
	RenamePlanStatusCanceled   RenamePlanStatus = "canceled"   // 用户取消（dry-run 阶段就丢弃）
)

// RenameItemStatus 单条改名条目状态
type RenameItemStatus string

const (
	RenameItemStatusPending  RenameItemStatus = "pending"  // 待执行
	RenameItemStatusSkipped  RenameItemStatus = "skipped"  // 不需要改名（命中目标格式 / 用户排除）
	RenameItemStatusUnsafe   RenameItemStatus = "unsafe"   // 安全检测未通过，禁止执行（除非用户强制）
	RenameItemStatusExecuted RenameItemStatus = "executed" // 已落盘
	RenameItemStatusFailed   RenameItemStatus = "failed"   // 单条执行失败
	RenameItemStatusReverted RenameItemStatus = "reverted" // 已回滚
)

// RenamePlan 一次"智能扫描+改名"任务
type RenamePlan struct {
	ID        string `json:"id" gorm:"primaryKey;type:text"`
	LibraryID string `json:"library_id" gorm:"index;type:text"`

	// 描述：扫描的根目录（绝对路径）
	RootPath string `json:"root_path" gorm:"type:text"`
	// 命名模板风格："jellyfin" / "plex"（默认 jellyfin）
	NamingStyle string `json:"naming_style" gorm:"type:text;default:jellyfin"`
	// 用户实际选用的模板表达式（可空，空则用默认）
	Template string `json:"template" gorm:"type:text"`
	// 是否启用 AI Fallback
	EnableAIFallback bool `json:"enable_ai_fallback" gorm:"default:true"`
	// AI 介入触发阈值（0~1，规则评分低于该值才调 AI）
	AIConfidenceThreshold float64 `json:"ai_confidence_threshold" gorm:"default:0.7"`

	// 状态
	Status RenamePlanStatus `json:"status" gorm:"type:text;default:draft;index"`
	// 是否仅 dry-run（强制只生成 plan 不落盘）
	DryRun bool `json:"dry_run" gorm:"default:true"`

	// 统计快照
	TotalItems    int `json:"total_items"`
	NeedRename    int `json:"need_rename"`    // 真正需要改名的条目数
	SkippedItems  int `json:"skipped_items"`  // 命中目标格式或被忽略
	UnsafeItems   int `json:"unsafe_items"`   // 安全检测拦截
	ExecutedItems int `json:"executed_items"` // 已落盘
	FailedItems   int `json:"failed_items"`   // 执行失败
	AIInvocations int `json:"ai_invocations"` // AI 调用次数（成本观测）

	// 创建者（用户 ID）
	CreatedBy string `json:"created_by" gorm:"type:text"`

	CreatedAt   time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time      `json:"updated_at"`
	ExecutedAt  *time.Time     `json:"executed_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	// 关联条目（延迟加载）
	Items []RenamePlanItem `json:"items,omitempty" gorm:"foreignKey:PlanID"`
}

// RenamePlanItem 单条改名条目
type RenamePlanItem struct {
	ID     string `json:"id" gorm:"primaryKey;type:text"`
	PlanID string `json:"plan_id" gorm:"index;type:text;not null"`

	// 关联媒体（可空：若文件在 DB 里没匹配上 Media）
	MediaID string `json:"media_id" gorm:"index;type:text"`

	// 源路径（绝对路径）
	SourcePath string `json:"source_path" gorm:"type:text;not null"`
	// 源文件名（含扩展名，无目录）
	SourceName string `json:"source_name" gorm:"type:text"`

	// 目标路径（绝对路径，含新文件名）
	TargetPath string `json:"target_path" gorm:"type:text"`
	// 目标文件名（含扩展名）
	TargetName string `json:"target_name" gorm:"type:text"`
	// 目标目录是否需要新建
	NeedMkdir bool `json:"need_mkdir"`

	// 识别结果
	ParsedTitle    string `json:"parsed_title" gorm:"type:text"`
	ParsedTitleAlt string `json:"parsed_title_alt" gorm:"type:text"`
	ParsedYear     int    `json:"parsed_year"`
	ParsedTMDbID   int    `json:"parsed_tmdb_id"`
	ParsedIMDbID   string `json:"parsed_imdb_id" gorm:"type:text"`
	// MediaType: movie / episode / unknown
	MediaType  string `json:"media_type" gorm:"type:text"`
	SeasonNum  int    `json:"season_num"`
	EpisodeNum int    `json:"episode_num"`

	// 识别置信度（0~1，1 表示完全可信）
	Confidence float64 `json:"confidence"`
	// 是否调用了 AI Fallback
	AIInvoked bool `json:"ai_invoked" gorm:"default:false"`
	// AI 原始回复（debug 用，JSON 字符串）
	AIRawResponse string `json:"ai_raw_response" gorm:"type:text"`

	// 附属（相关）资源：和主视频一同迁移的文件（.nfo / .srt / -poster.jpg ...）
	// JSON 数组：[{"source":"...","target":"...","kind":"nfo|subtitle|poster|fanart|thumb|other"}]
	RelatedFilesJSON string `json:"related_files_json" gorm:"type:text"`

	// 安全检测
	SafetyJSON string `json:"safety_json" gorm:"type:text"` // 详细 JSON
	SafetyOK   bool   `json:"safety_ok"`                    // 总判定
	SafetyNote string `json:"safety_note" gorm:"type:text"` // 单行简述

	// 状态
	Status   RenameItemStatus `json:"status" gorm:"type:text;default:pending;index"`
	ErrorMsg string           `json:"error_msg" gorm:"type:text"`

	// 用户手动调整的目标名（若用户编辑过，优先取这里）
	OverrideName string `json:"override_name" gorm:"type:text"`
	// 用户排除标记
	Excluded bool `json:"excluded" gorm:"default:false"`

	CreatedAt time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// RenameJournalOp 单条物理操作类型
type RenameJournalOp string

const (
	RenameJournalOpMove   RenameJournalOp = "move"   // 移动/重命名文件
	RenameJournalOpMkdir  RenameJournalOp = "mkdir"  // 创建目录
	RenameJournalOpRmdir  RenameJournalOp = "rmdir"  // 删除空目录（回滚时）
	RenameJournalOpUpdate RenameJournalOp = "update" // 更新 DB
)

// RenameJournal 物理操作日志（每条物理动作一条，用于回滚 + 审计）
type RenameJournal struct {
	ID     uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	PlanID string `json:"plan_id" gorm:"index;type:text;not null"`
	ItemID string `json:"item_id" gorm:"index;type:text"`

	Op       RenameJournalOp `json:"op" gorm:"type:text;not null"`
	FromPath string          `json:"from_path" gorm:"type:text"`
	ToPath   string          `json:"to_path" gorm:"type:text"`

	// 是否已成功执行
	Success bool   `json:"success"`
	Error   string `json:"error" gorm:"type:text"`

	// 是否已回滚
	Reverted   bool       `json:"reverted" gorm:"default:false"`
	RevertedAt *time.Time `json:"reverted_at"`

	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

// TableName 自定义表名，避免 gorm 默认复数化
func (RenamePlan) TableName() string     { return "rename_plans" }
func (RenamePlanItem) TableName() string { return "rename_plan_items" }
func (RenameJournal) TableName() string  { return "rename_journals" }
