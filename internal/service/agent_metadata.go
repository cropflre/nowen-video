package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

var ErrAgentMetadataUnsupported = errors.New("目前只支持已关联 TMDb 的电影，请先在影片资料页面确认匹配")
var ErrAgentMetadataNoChanges = errors.New("没有可补全的空缺文字资料；已有资料会保留")
var ErrAgentMetadataSource = errors.New("暂时无法读取 TMDb 资料，请检查资料源配置后重试")

// Fetch is read-only: no media updates, downloads, NFO writes or model calls.
type AgentMetadataProvider interface {
	FetchAgentMovieMetadata(context.Context, int) (*TMDbMovieDetail, error)
}
type AgentMetadataChange struct {
	Field  string `json:"field"`
	Label  string `json:"label"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type agentMetadataSnapshot struct {
	Library   string            `json:"library"`
	ID        string            `json:"id"`
	LibraryID string            `json:"library_id"`
	Title     string            `json:"title"`
	TMDbID    int               `json:"tmdb_id"`
	UpdatedAt time.Time         `json:"updated_at"`
	Values    map[string]string `json:"values"`
	Nulls     map[string]bool   `json:"nulls"`
}
type agentMetadataProposal struct {
	SourceTitle string                `json:"source_title"`
	Changes     []AgentMetadataChange `json:"changes"`
}

var agentMetadataFields = []struct{ field, label string }{
	{"overview", "简介"}, {"orig_title", "原名"}, {"genres", "类型"}, {"premiered", "首映日期"},
}

func metadataValues(m model.Media) map[string]string {
	return map[string]string{"overview": m.Overview, "orig_title": m.OrigTitle, "genres": m.Genres, "premiered": m.Premiered}
}
func loadAgentMetadata(db *gorm.DB, id string) (model.Media, model.Library, error) {
	var m model.Media
	// Do not persist stream headers/cookies in an approval snapshot.
	err := db.Select("id", "library_id", "title", "tm_db_id", "media_type", "overview", "orig_title", "genres", "premiered", "updated_at").First(&m, "id = ?", id).Error
	if err != nil {
		return m, model.Library{}, err
	}
	var lib model.Library
	if err = db.First(&lib, "id = ?", m.LibraryID).Error; err != nil {
		return m, lib, err
	}
	if m.MediaType != "movie" || m.TMDbID <= 0 || (lib.Type != "movie" && lib.Type != "other") {
		return m, lib, ErrAgentMetadataUnsupported
	}
	return m, lib, nil
}
func metadataSnapshot(db *gorm.DB, m model.Media, lib model.Library) (string, error) {
	var stored map[string]interface{}
	if err := db.Model(&model.Media{}).Select("overview", "orig_title", "genres", "premiered").Take(&stored, "id = ?", m.ID).Error; err != nil {
		return "", err
	}
	nulls := map[string]bool{}
	for _, f := range agentMetadataFields {
		if stored[f.field] == nil {
			nulls[f.field] = true
		}
	}
	raw, _ := json.Marshal(agentMetadataSnapshot{Library: scanSnapshot(lib), ID: m.ID, LibraryID: m.LibraryID, Title: m.Title, TMDbID: m.TMDbID, UpdatedAt: m.UpdatedAt, Values: metadataValues(m), Nulls: nulls})
	return string(raw), nil
}
func metadataRunView(run model.AgentRun) AgentRunView {
	var snapshot agentMetadataSnapshot
	var proposal agentMetadataProposal
	_ = json.Unmarshal([]byte(run.Snapshot), &snapshot)
	_ = json.Unmarshal([]byte(run.Proposal), &proposal)
	return AgentRunView{AgentRun: run, Paths: []string{}, MediaID: snapshot.ID, MediaTitle: snapshot.Title, SourceTitle: proposal.SourceTitle,
		SourceURL: fmt.Sprintf("https://www.themoviedb.org/movie/%d", snapshot.TMDbID), Changes: proposal.Changes,
		Effect: "仅补全预览中的空缺文字资料；保留已有资料、片名、图片、评分及儿童分级。原文件和 NFO 不变，完成后可在字段未被再次修改时撤销。"}
}
func (s *AgentScanService) PreviewMetadata(ctx context.Context, uid, id string) (AgentRunView, error) {
	db := s.db.WithContext(ctx)
	user, err := agentScanAdmin(db, uid)
	if err != nil {
		return AgentRunView{}, err
	}
	if s.metadata == nil {
		return AgentRunView{}, ErrAgentRunUnavailable
	}
	m, lib, err := loadAgentMetadata(db, id)
	if err != nil {
		return AgentRunView{}, err
	}
	before, err := metadataSnapshot(db, m, lib)
	if err != nil {
		return AgentRunView{}, err
	}
	values := metadataValues(m)
	missing := false
	for _, f := range agentMetadataFields {
		if strings.TrimSpace(values[f.field]) == "" {
			missing = true
		}
	}
	if !missing {
		return AgentRunView{}, ErrAgentMetadataNoChanges
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	detail, err := s.metadata.FetchAgentMovieMetadata(fetchCtx, m.TMDbID)
	if err != nil || fetchCtx.Err() != nil || detail == nil || detail.ID != m.TMDbID {
		return AgentRunView{}, ErrAgentMetadataSource
	}
	genres := []string{}
	for _, g := range detail.Genres {
		if v := strings.TrimSpace(g.Name); v != "" {
			genres = append(genres, v)
		}
	}
	date := ""
	if _, e := time.Parse("2006-01-02", detail.ReleaseDate); e == nil {
		date = detail.ReleaseDate
	}
	source := map[string]string{"overview": strings.TrimSpace(detail.Overview), "orig_title": strings.TrimSpace(detail.OriginalTitle), "genres": strings.Join(genres, ","), "premiered": date}
	proposal := agentMetadataProposal{SourceTitle: strings.TrimSpace(detail.Title), Changes: []AgentMetadataChange{}}
	if len([]rune(proposal.SourceTitle)) > 512 {
		return AgentRunView{}, ErrAgentMetadataSource
	}
	for _, f := range agentMetadataFields {
		after := source[f.field]
		limit := 2048
		if f.field == "overview" {
			limit = 16000
		}
		if len([]rune(after)) > limit {
			return AgentRunView{}, ErrAgentMetadataSource
		}
		if strings.TrimSpace(values[f.field]) == "" && after != "" {
			proposal.Changes = append(proposal.Changes, AgentMetadataChange{f.field, f.label, values[f.field], after})
		}
	}
	if len(proposal.Changes) == 0 {
		return AgentRunView{}, ErrAgentMetadataNoChanges
	}
	raw, _ := json.Marshal(proposal)
	now := s.now()
	run := model.AgentRun{ID: uuid.NewString(), UserID: uid, TokenVersion: user.TokenVersion, Kind: "fill_movie_metadata", LibraryID: lib.ID, LibraryName: lib.Name, Snapshot: before, Proposal: string(raw), Status: "draft", ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now, UpdatedAt: now, Message: "请核对影片、来源和每项变化，确认后保存空缺资料"}
	run.Digest = scanDigest(run)
	if err = db.Create(&run).Error; err != nil {
		return AgentRunView{}, err
	}
	return metadataRunView(run), nil
}
func (s *AgentScanService) UndoMetadata(ctx context.Context, uid, id, digest string) (AgentRunView, error) {
	return s.confirmMetadata(ctx, uid, id, digest, true)
}
func (s *AgentScanService) confirmMetadata(ctx context.Context, uid, id, digest string, undo bool) (AgentRunView, error) {
	db := s.db.WithContext(ctx)
	var run model.AgentRun
	err := db.Transaction(func(tx *gorm.DB) error {
		user, err := agentScanAdmin(tx, uid)
		if err != nil {
			return err
		}
		if err = tx.Model(&model.AgentRun{}).Where("id = ? AND user_id = ?", id, uid).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; err != nil {
			return err
		}
		if err = tx.Where("id = ? AND user_id = ?", id, uid).First(&run).Error; err != nil {
			return err
		}
		if run.Kind != "fill_movie_metadata" || digest == "" || digest != run.Digest || scanDigest(run) != digest {
			return ErrAgentPreviewChanged
		}
		if user.TokenVersion != run.TokenVersion {
			return ErrAgentPreviewChanged
		}
		if undo {
			if run.Status == "reverted" {
				return nil
			}
			if run.Status != "completed" {
				return ErrAgentPreviewChanged
			}
		} else {
			if run.Status != "draft" {
				return nil
			}
			if !s.now().Before(run.ExpiresAt) {
				return ErrAgentPreviewChanged
			}
		}
		var snapshot agentMetadataSnapshot
		var proposal agentMetadataProposal
		if json.Unmarshal([]byte(run.Snapshot), &snapshot) != nil || json.Unmarshal([]byte(run.Proposal), &proposal) != nil || len(proposal.Changes) == 0 {
			return ErrAgentPreviewChanged
		}
		m, lib, err := loadAgentMetadata(tx, snapshot.ID)
		if err != nil {
			return ErrAgentPreviewChanged
		}
		if m.LibraryID != snapshot.LibraryID || m.TMDbID != snapshot.TMDbID {
			return ErrAgentPreviewChanged
		}
		if !undo {
			current, err := metadataSnapshot(tx, m, lib)
			if err != nil {
				return err
			}
			if current != run.Snapshot {
				return ErrAgentPreviewChanged
			}
		}
		values := metadataValues(m)
		patch := map[string]interface{}{}
		for _, change := range proposal.Changes {
			if _, allowed := values[change.Field]; !allowed || strings.TrimSpace(change.Before) != "" || strings.TrimSpace(change.After) == "" {
				return ErrAgentPreviewChanged
			}
			expected, value := change.Before, change.After
			if undo {
				expected, value = change.After, change.Before
			}
			if values[change.Field] != expected {
				return ErrAgentPreviewChanged
			}
			patch[change.Field] = value
			if undo && snapshot.Nulls[change.Field] {
				patch[change.Field] = nil
			}
		}
		update := tx.Model(&model.Media{}).Where("id = ? AND library_id = ? AND tm_db_id = ? AND updated_at = ?", m.ID, m.LibraryID, m.TMDbID, m.UpdatedAt)
		for _, change := range proposal.Changes {
			expected := change.Before
			if undo {
				expected = change.After
			}
			if !undo && snapshot.Nulls[change.Field] {
				update = update.Where(change.Field + " IS NULL")
			} else {
				update = update.Where(change.Field+" = ?", expected)
			}
		}
		result := update.Updates(patch)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAgentPreviewChanged
		}
		now := s.now()
		status, message := "completed", fmt.Sprintf("已补全《%s》的 %d 项空缺资料，已有资料和原文件保持原状。", m.Title, len(patch))
		if undo {
			status, message = "reverted", "本次补全已撤销，恢复了预览前的空缺字段。"
		}
		updates := map[string]interface{}{"status": status, "message": message, "completed_at": now}
		if !undo {
			updates["started_at"] = now
		}
		if err = tx.Model(&model.AgentRun{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&run, "id = ?", id).Error
	})
	if err != nil {
		return AgentRunView{}, err
	}
	return metadataRunView(run), nil
}

// This delegation keeps Full/Lite on their existing, configured TMDb client.
func (s *LibraryService) FetchAgentMovieMetadata(ctx context.Context, id int) (*TMDbMovieDetail, error) {
	if s.metadata == nil {
		return nil, ErrAgentRunUnavailable
	}
	return s.metadata.FetchAgentMovieMetadata(ctx, id)
}
