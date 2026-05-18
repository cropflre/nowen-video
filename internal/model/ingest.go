package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ==================== 懒人入库（Lazy Ingest）任务 ====================
//
// 该模型只为"一键入库"流程提供任务编排与进度跟踪：
//   1) 用户只给一个源目录 SourcePath；
//   2) 系统自动扫描 -> AI 分类 -> 建库 -> 生成命名规划 -> 落盘；
//   3) 全过程进度、统计、错误均沉淀在该任务记录里，供前端轮询/订阅。
//
// 安全约束：
//   - 所有产出的文件最终落到 SourcePath/_organized/ 之下（懒人模式 A 方案）；
//   - 默认 KeepOriginal=true（hardlink 优先；不可时降级 copy，绝不删除源）；
//   - 置信度低于阈值的条目挂在 RenamePlan 中等待人工，绝不自动落盘。

// IngestJobStatus 懒人入库任务状态
type IngestJobStatus string

const (
	IngestJobStatusPending     IngestJobStatus = "pending"     // 已创建，等待执行
	IngestJobStatusScanning    IngestJobStatus = "scanning"    // 扫描磁盘+落库
	IngestJobStatusClassifying IngestJobStatus = "classifying" // AI 分类
	IngestJobStatusPlanning    IngestJobStatus = "planning"    // 生成 RenamePlan
	IngestJobStatusExecuting   IngestJobStatus = "executing"   // 落盘移动
	IngestJobStatusCompleted   IngestJobStatus = "completed"   // 完成
	IngestJobStatusFailed      IngestJobStatus = "failed"      // 失败
	IngestJobStatusCanceled    IngestJobStatus = "canceled"    // 用户取消
)

// IngestJobMode 落盘策略
//   - hardlink: 默认，仅创建硬链接，源文件 0 风险，零额外空间（懒人模式）
//   - move    : 专家模式，os.Rename 原地移动；同卷瞬间完成、释放源；提供 journal 回滚
type IngestJobMode string

const (
	IngestJobModeHardlink IngestJobMode = "hardlink"
	IngestJobModeMove     IngestJobMode = "move"
)

// IngestJob 懒人入库任务
//
// 字段尽量小而精，只保留"一键入库"必要的编排字段；明细放在 RenamePlan / RenamePlanItem。
type IngestJob struct {
	ID string `json:"id" gorm:"primaryKey;type:text"`

	// 输入：用户唯一需要给的参数
	SourcePath string `json:"source_path" gorm:"type:text;not null"`

	// 自动派生：目标根目录（默认 = SourcePath/_organized）
	TargetRoot string `json:"target_root" gorm:"type:text"`

	// 执行策略（懒人模式默认值即可，全部硬编码到服务里，模型仅记录）
	KeepOriginal bool   `json:"keep_original" gorm:"default:true"` // 默认不删源
	NamingStyle  string `json:"naming_style" gorm:"type:text;default:jellyfin"`

	// 落盘模式：hardlink（默认，懒人）/ move（专家，原地移动 + journal 回滚）
	Mode string `json:"mode" gorm:"type:text;default:hardlink;index"`
	// Move 模式下的 journal 文件绝对路径（每行一条 JSON 操作记录，用于回滚）
	JournalPath string `json:"journal_path" gorm:"type:text"`

	// 状态机
	Status IngestJobStatus `json:"status" gorm:"type:text;default:pending;index"`
	Phase  string          `json:"phase" gorm:"type:text"` // 人类可读阶段（与 Status 对应，便于 UI 展示）

	// 进度（0-100，整体感知）
	Progress int `json:"progress"`

	// 派生产物
	LibraryIDs string `json:"library_ids" gorm:"type:text"` // JSON 数组：本次自动创建/复用的媒体库 ID
	PlanIDs    string `json:"plan_ids" gorm:"type:text"`    // JSON 数组：本次产出的 RenamePlan ID

	// 统计快照（JSON）
	// {"scanned":N,"classified":N,"planned":N,"executed":N,"skipped":N,"failed":N,"unsorted":N}
	Stats string `json:"stats" gorm:"type:text"`

	// 错误信息（最后一条致命错误）
	ErrorMessage string `json:"error_message" gorm:"type:text"`

	// 创建者（用户 ID，多用户审计）
	CreatedBy string `json:"created_by" gorm:"type:text"`

	CreatedAt   time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// TableName 显式表名
func (IngestJob) TableName() string {
	return "ingest_jobs"
}

// BeforeCreate 自动生成 UUID
func (j *IngestJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == "" {
		j.ID = uuid.New().String()
	}
	return nil
}
