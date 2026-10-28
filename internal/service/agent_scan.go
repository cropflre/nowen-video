package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

var ErrAgentPreviewChanged = errors.New("预览已过期，或影片、影视库及账号状态已变化，请重新核对；撤销不会覆盖后续编辑")
var ErrAgentRunBusy = errors.New("这个影视库已有任务，请等待完成后再试")
var ErrAgentRunUnavailable = errors.New("当前服务暂不能执行此任务")
var ErrAgentScanUnsupported = errors.New("此影视库暂不支持查找新电影")

type AgentScanExecutor interface {
	ScanNewMovies(*model.Library) (int, error)
}
type AgentRunView struct {
	model.AgentRun
	Paths       []string              `json:"paths"`
	Effect      string                `json:"effect"`
	MinFileSize int                   `json:"min_file_size_mb"`
	FileFilter  bool                  `json:"file_filter"`
	MediaID     string                `json:"media_id,omitempty"`
	MediaTitle  string                `json:"media_title,omitempty"`
	SourceTitle string                `json:"source_title,omitempty"`
	SourceURL   string                `json:"source_url,omitempty"`
	Changes     []AgentMetadataChange `json:"changes,omitempty"`
}
type AgentScanService struct {
	db       *gorm.DB
	executor AgentScanExecutor
	metadata AgentMetadataProvider
	worker   string
	now      func() time.Time
}

func NewAgentScanService(db *gorm.DB, executor AgentScanExecutor) *AgentScanService {
	s := &AgentScanService{db: db, executor: executor, worker: uuid.NewString(), now: time.Now}
	if provider, ok := executor.(AgentMetadataProvider); ok {
		s.metadata = provider
	}
	return s
}
func agentScanAdmin(db *gorm.DB, uid string) (model.User, error) {
	var user model.User
	if err := db.Select("id", "role", "child_mode", "disabled", "must_change_pwd", "token_version").First(&user, "id = ?", uid).Error; err != nil {
		return user, ErrForbidden
	}
	if user.Role != "admin" || user.ChildMode || user.Disabled || user.MustChangePwd {
		return user, ErrForbidden
	}
	return user, nil
}
func scanSnapshot(lib model.Library) string { raw, _ := json.Marshal(lib); return string(raw) }
func scanDigest(run model.AgentRun) string {
	payload := run.ID + "\x00" + run.UserID + "\x00" + run.Kind + "\x00" + run.Snapshot + "\x00" + run.ExpiresAt.UTC().Format(time.RFC3339Nano)
	if run.Proposal != "" {
		payload += "\x00" + run.Proposal
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}
func scanRunView(run model.AgentRun) AgentRunView {
	if run.Kind == "fill_movie_metadata" {
		return metadataRunView(run)
	}
	var lib model.Library
	_ = json.Unmarshal([]byte(run.Snapshot), &lib)
	return AgentRunView{AgentRun: run, Paths: lib.AllPaths(), Effect: "登记新电影，已有记录不变；原文件不会移动、改名或删除，不自动整理或补全在线资料。新增影片仍需家长审核分级。", MinFileSize: lib.MinFileSize, FileFilter: lib.EnableFileFilter}
}
func (s *AgentScanService) Preview(ctx context.Context, uid, libraryID string) (AgentRunView, error) {
	db := s.db.WithContext(ctx)
	user, err := agentScanAdmin(db, uid)
	if err != nil {
		return AgentRunView{}, err
	}
	if s.executor == nil {
		return AgentRunView{}, ErrAgentRunUnavailable
	}
	var lib model.Library
	if err = db.First(&lib, "id = ?", libraryID).Error; err != nil {
		return AgentRunView{}, err
	}
	if lib.Type != "movie" && lib.Type != "other" {
		return AgentRunView{}, fmt.Errorf("%w：目前只支持电影库", ErrAgentScanUnsupported)
	}
	if len(lib.AllPaths()) == 0 {
		return AgentRunView{}, ErrAgentPreviewChanged
	}
	for _, path := range lib.AllPaths() {
		if !filepath.IsAbs(path) {
			return AgentRunView{}, fmt.Errorf("%w：需要本机或已挂载的绝对文件夹路径", ErrAgentScanUnsupported)
		}
	}
	now := s.now()
	run := model.AgentRun{ID: uuid.NewString(), UserID: uid, TokenVersion: user.TokenVersion, Kind: "discover_movies", LibraryID: lib.ID, LibraryName: lib.Name, Snapshot: scanSnapshot(lib), Status: "draft", ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now, UpdatedAt: now, Message: "请核对范围，确认后开始查找新影片"}
	run.Digest = scanDigest(run)
	if err = db.Create(&run).Error; err != nil {
		return AgentRunView{}, err
	}
	return scanRunView(run), nil
}
func (s *AgentScanService) recoverExpired(db *gorm.DB) error {
	now := s.now()
	if err := db.Model(&model.AgentRun{}).Where("status = ? AND expires_at <= ?", "draft", now).Updates(map[string]interface{}{"status": "expired", "message": "预览已过期，请重新预览"}).Error; err != nil {
		return err
	}
	return db.Model(&model.AgentRun{}).Where("status = ? AND lease_until <= ?", "running", now).Updates(map[string]interface{}{"status": "interrupted", "active_library_id": nil, "lease_until": nil, "message": "任务心跳已中断，可能已登记部分影片。请核对后重新预览；不会自动重复执行。", "completed_at": now}).Error
}
func (s *AgentScanService) List(ctx context.Context, uid string) ([]AgentRunView, error) {
	db := s.db.WithContext(ctx)
	if _, err := agentScanAdmin(db, uid); err != nil {
		return nil, err
	}
	if err := s.recoverExpired(db); err != nil {
		return nil, err
	}
	runs := []model.AgentRun{}
	if err := db.Where("user_id = ?", uid).Order("created_at DESC,id ASC").Limit(20).Find(&runs).Error; err != nil {
		return nil, err
	}
	views := []AgentRunView{}
	for _, run := range runs {
		views = append(views, scanRunView(run))
	}
	return views, nil
}
func (s *AgentScanService) Cancel(ctx context.Context, uid, id string) (AgentRunView, error) {
	db := s.db.WithContext(ctx)
	if _, err := agentScanAdmin(db, uid); err != nil {
		return AgentRunView{}, err
	}
	var run model.AgentRun
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&run).Error; err != nil {
		return AgentRunView{}, err
	}
	if run.Status == "draft" {
		if err := db.Model(&model.AgentRun{}).Where("id = ? AND user_id = ? AND status = ?", id, uid, "draft").Updates(map[string]interface{}{"status": "cancelled", "message": "预览已取消，未启动任务"}).Error; err != nil {
			return AgentRunView{}, err
		}
	}
	if err := db.Where("id = ? AND user_id = ?", id, uid).First(&run).Error; err != nil {
		return AgentRunView{}, err
	}
	return scanRunView(run), nil
}
func (s *AgentScanService) Confirm(ctx context.Context, uid, id, digest string) (AgentRunView, error) {
	// Metadata proposals contain the fetched values; confirmation only commits
	// the approved database patch, never another provider request.
	var kind string
	if err := s.db.WithContext(ctx).Model(&model.AgentRun{}).Where("id = ? AND user_id = ?", id, uid).Pluck("kind", &kind).Error; err != nil {
		return AgentRunView{}, err
	}
	if kind == "fill_movie_metadata" {
		return s.confirmMetadata(ctx, uid, id, digest, false)
	}
	if s.executor == nil {
		return AgentRunView{}, ErrAgentRunUnavailable
	}
	db := s.db.WithContext(ctx)
	var run model.AgentRun
	var lib model.Library
	launch := false
	err := db.Transaction(func(tx *gorm.DB) error {
		user, err := agentScanAdmin(tx, uid)
		if err != nil {
			return err
		}
		// Acquire the write transaction before reading/claiming a draft (SQLite).
		if err = tx.Model(&model.AgentRun{}).Where("id = ? AND user_id = ?", id, uid).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; err != nil {
			return err
		}
		if err = tx.Where("id = ? AND user_id = ?", id, uid).First(&run).Error; err != nil {
			return err
		}
		if digest == "" || digest != run.Digest || scanDigest(run) != digest {
			return ErrAgentPreviewChanged
		}
		if run.Status != "draft" {
			return nil
		} // A replay returns its existing receipt.
		if !s.now().Before(run.ExpiresAt) || user.TokenVersion != run.TokenVersion {
			return ErrAgentPreviewChanged
		}
		if err = tx.First(&lib, "id = ?", run.LibraryID).Error; err != nil {
			return ErrAgentPreviewChanged
		}
		if scanSnapshot(lib) != run.Snapshot {
			return ErrAgentPreviewChanged
		}
		if err = s.recoverExpired(tx); err != nil {
			return err
		}
		var busy int64
		if err = tx.Model(&model.AgentRun{}).Where("active_library_id = ?", run.LibraryID).Count(&busy).Error; err != nil {
			return err
		}
		if busy > 0 {
			return ErrAgentRunBusy
		}
		now := s.now()
		until := now.Add(2 * time.Minute)
		result := tx.Model(&model.AgentRun{}).Where("id = ? AND status = ?", id, "draft").Updates(map[string]interface{}{"status": "running", "active_library_id": run.LibraryID, "worker_id": s.worker, "lease_until": until, "started_at": now, "message": "已确认，正在查找并登记新电影"})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAgentRunBusy
		}
		launch = true
		return tx.First(&run, "id = ?", id).Error
	})
	if err != nil {
		return AgentRunView{}, err
	}
	if launch {
		go s.execute(run, lib)
	}
	return scanRunView(run), nil
}
func (s *AgentScanService) execute(run model.AgentRun, lib model.Library) {
	done := make(chan struct{})
	defer close(done)
	go func() {
		timer := time.NewTicker(15 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-done:
				return
			case <-timer.C:
				until := s.now().Add(2 * time.Minute)
				_ = s.db.Model(&model.AgentRun{}).Where("id = ? AND status = ? AND worker_id = ?", run.ID, "running", s.worker).Update("lease_until", until).Error
			}
		}
	}()
	count := 0
	var err error
	func() {
		defer func() {
			if recover() != nil {
				err = errors.New("scan interrupted")
			}
		}()
		count, err = s.executor.ScanNewMovies(&lib)
	}()
	now := s.now()
	status, message := "completed", fmt.Sprintf("查找完成，新增登记 %d 部电影。已有记录和原文件保持原状。", count)
	if err != nil {
		status = "failed"
		message = fmt.Sprintf("查找未完成，已登记 %d 部电影。请检查文件夹连接、访问权限或是否已有扫描任务，再重新预览。", count)
	}
	_ = s.db.Model(&model.AgentRun{}).Where("id = ? AND status = ? AND worker_id = ?", run.ID, "running", s.worker).Updates(map[string]interface{}{"status": status, "message": message, "new_count": count, "completed_at": now, "active_library_id": nil, "lease_until": nil}).Error
}
