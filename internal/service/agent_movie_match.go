package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

var ErrAgentMatchInput = errors.New("请使用 1 至 120 字的影片名称，年份可留空或填写 1888 至 2100")
var ErrAgentMatchUnsupported = errors.New("目前只支持尚未关联 TMDb 的电影；已有关联的影片可直接预览资料补全")
var ErrAgentMatchSelection = errors.New("请从本次搜索结果中选择候选，再明确确认匹配")
var ErrAgentMatchUndo = errors.New("影片身份或资料已变化，无法安全撤销匹配；请先撤销后续资料补全，或在影片资料页面核对")

type AgentMovieMatchProvider interface {
	SearchAgentMovieMatches(context.Context, string, int) ([]TMDbMovie, error)
}
type AgentMovieMatchCandidate struct {
	ID            int      `json:"id"`
	Title         string   `json:"title"`
	OriginalTitle string   `json:"original_title"`
	Year          int      `json:"year"`
	Overview      string   `json:"overview"`
	SourceURL     string   `json:"source_url"`
	Evidence      []string `json:"evidence"`
}
type agentMovieMatchProposal struct {
	Query      string                     `json:"query"`
	Year       int                        `json:"year"`
	Candidates []AgentMovieMatchCandidate `json:"candidates"`
	SelectedID int                        `json:"selected_id"`
}

func movieMatchRunView(run model.AgentRun) AgentRunView {
	var snapshot agentMetadataSnapshot
	var proposal agentMovieMatchProposal
	_ = json.Unmarshal([]byte(run.Snapshot), &snapshot)
	_ = json.Unmarshal([]byte(run.Proposal), &proposal)
	view := AgentRunView{AgentRun: run, Paths: []string{}, MediaID: snapshot.ID, MediaTitle: snapshot.Title, MediaYear: snapshot.Year, MatchQuery: proposal.Query, MatchYear: proposal.Year, MatchCandidates: proposal.Candidates,
		Effect: "只保存此电影的 TMDb 关联；片名、现有资料、图片、儿童分级和原文件保持原状。关联后可另行预览资料补全，不会自动覆盖字段。"}
	for _, c := range proposal.Candidates {
		if c.ID == proposal.SelectedID {
			candidate := c
			view.SelectedMatch = &candidate
			view.SourceTitle = c.Title
			view.SourceURL = c.SourceURL
			break
		}
	}
	return view
}
func matchTitle(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }
func movieMatchCandidates(m model.Media, results []TMDbMovie) []AgentMovieMatchCandidate {
	out := []AgentMovieMatchCandidate{}
	seen := map[int]bool{}
	for i, r := range results {
		if i >= 20 || len(out) >= 6 {
			break
		}
		title, original := strings.TrimSpace(r.Title), strings.TrimSpace(r.OriginalTitle)
		if r.ID <= 0 || seen[r.ID] || title == "" || len([]rune(title)) > 512 || len([]rune(original)) > 512 {
			continue
		}
		seen[r.ID] = true
		year := 0
		if date, e := time.Parse("2006-01-02", r.ReleaseDate); e == nil {
			year = date.Year()
		}
		evidence := []string{}
		if matchTitle(m.Title) == matchTitle(title) || (m.OrigTitle != "" && matchTitle(m.OrigTitle) == matchTitle(original)) {
			evidence = append(evidence, "片名或原名一致，仍需核对")
		} else {
			evidence = append(evidence, "片名有差异，请核对原名和简介")
		}
		if year == 0 || m.Year == 0 {
			evidence = append(evidence, "本地或来源缺少年份，无法对照")
		} else if year == m.Year {
			evidence = append(evidence, "年份一致")
		} else {
			evidence = append(evidence, fmt.Sprintf("年份不同：本地 %d，来源 %d", m.Year, year))
		}
		overview := []rune(strings.TrimSpace(r.Overview))
		if len(overview) > 500 {
			overview = append(overview[:500], []rune("…")...)
		}
		out = append(out, AgentMovieMatchCandidate{ID: r.ID, Title: title, OriginalTitle: original, Year: year, Overview: string(overview), SourceURL: fmt.Sprintf("https://www.themoviedb.org/movie/%d", r.ID), Evidence: evidence})
	}
	return out
}
func (s *AgentScanService) PreviewMovieMatch(ctx context.Context, uid, id, query string, year *int) (AgentRunView, error) {
	db := s.db.WithContext(ctx)
	user, err := agentScanAdmin(db, uid)
	if err != nil {
		return AgentRunView{}, err
	}
	if s.matcher == nil {
		return AgentRunView{}, ErrAgentRunUnavailable
	}
	m, lib, err := loadAgentMovie(db, id)
	if err != nil {
		return AgentRunView{}, err
	}
	if m.TMDbID > 0 {
		return AgentRunView{}, ErrAgentMatchUnsupported
	}
	query = strings.TrimSpace(query)
	if query == "" {
		query = strings.TrimSpace(m.Title)
	}
	searchYear := m.Year
	if year != nil {
		searchYear = *year
	}
	if len([]rune(query)) < 1 || len([]rune(query)) > 120 || (searchYear != 0 && (searchYear < 1888 || searchYear > 2100)) {
		return AgentRunView{}, ErrAgentMatchInput
	}
	snapshot, err := metadataSnapshot(db, m, lib)
	if err != nil {
		return AgentRunView{}, err
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	results, err := s.matcher.SearchAgentMovieMatches(fetchCtx, query, searchYear)
	if err != nil || fetchCtx.Err() != nil {
		return AgentRunView{}, ErrAgentMetadataSource
	}
	proposal := agentMovieMatchProposal{Query: query, Year: searchYear, Candidates: movieMatchCandidates(m, results)}
	raw, _ := json.Marshal(proposal)
	now := s.now()
	run := model.AgentRun{ID: uuid.NewString(), UserID: uid, TokenVersion: user.TokenVersion, Kind: "match_movie", LibraryID: lib.ID, LibraryName: lib.Name, Snapshot: snapshot, Proposal: string(raw), Status: "draft", ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now, UpdatedAt: now, Message: "已找到候选，请核对片名、年份和简介；选择不会修改影片资料"}
	if len(proposal.Candidates) == 0 {
		run.Message = "没有找到可用候选。可以修改片名、留空年份重新搜索，或取消；不会自动匹配其他影片。"
	}
	run.Digest = scanDigest(run)
	err = db.Transaction(func(tx *gorm.DB) error {
		// Serialize preview replacement with confirm/select, including on SQLite.
		if e := tx.Model(&model.User{}).Where("id = ?", uid).UpdateColumn("token_version", gorm.Expr("token_version")).Error; e != nil {
			return e
		}
		currentUser, e := agentScanAdmin(tx, uid)
		if e != nil {
			return e
		}
		if currentUser.TokenVersion != user.TokenVersion {
			return ErrAgentPreviewChanged
		}
		current, currentLib, e := loadAgentMovie(tx, id)
		if e != nil {
			return ErrAgentPreviewChanged
		}
		currentSnapshot, e := metadataSnapshot(tx, current, currentLib)
		if e != nil {
			return e
		}
		if currentSnapshot != snapshot {
			return ErrAgentPreviewChanged
		}
		var previous []model.AgentRun
		if e = tx.Where("user_id = ? AND library_id = ? AND kind = ? AND status = ?", uid, lib.ID, "match_movie", "draft").Find(&previous).Error; e != nil {
			return e
		}
		for _, old := range previous {
			var snap agentMetadataSnapshot
			_ = json.Unmarshal([]byte(old.Snapshot), &snap)
			if snap.ID == id {
				if e = tx.Model(&model.AgentRun{}).Where("id = ? AND status = ?", old.ID, "draft").Updates(map[string]interface{}{"status": "cancelled", "message": "已重新搜索，请使用最新候选预览"}).Error; e != nil {
					return e
				}
			}
		}
		return tx.Create(&run).Error
	})
	if err != nil {
		return AgentRunView{}, err
	}
	return movieMatchRunView(run), nil
}
func (s *AgentScanService) SelectMovieMatch(ctx context.Context, uid, id, digest string, candidateID int) (AgentRunView, error) {
	var run model.AgentRun
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, e := agentScanAdmin(tx, uid)
		if e != nil {
			return e
		}
		if e = tx.Model(&model.AgentRun{}).Where("id = ? AND user_id = ?", id, uid).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; e != nil {
			return e
		}
		if e = tx.Where("id = ? AND user_id = ?", id, uid).First(&run).Error; e != nil {
			return e
		}
		if run.Kind != "match_movie" || run.Status != "draft" || digest == "" || digest != run.Digest || scanDigest(run) != digest || user.TokenVersion != run.TokenVersion || !s.now().Before(run.ExpiresAt) {
			return ErrAgentPreviewChanged
		}
		var snapshot agentMetadataSnapshot
		var proposal agentMovieMatchProposal
		if json.Unmarshal([]byte(run.Snapshot), &snapshot) != nil || json.Unmarshal([]byte(run.Proposal), &proposal) != nil {
			return ErrAgentPreviewChanged
		}
		found := false
		for _, candidate := range proposal.Candidates {
			if candidateID > 0 && candidate.ID == candidateID {
				found = true
			}
		}
		if !found {
			return ErrAgentMatchSelection
		}
		m, lib, e := loadAgentMovie(tx, snapshot.ID)
		if e != nil {
			return ErrAgentPreviewChanged
		}
		before, e := metadataSnapshot(tx, m, lib)
		if e != nil {
			return e
		}
		if before != run.Snapshot {
			return ErrAgentPreviewChanged
		}
		proposal.SelectedID = candidateID
		raw, _ := json.Marshal(proposal)
		run.Proposal = string(raw)
		run.Digest = scanDigest(run)
		if e = tx.Model(&model.AgentRun{}).Where("id = ?", id).Updates(map[string]interface{}{"proposal": run.Proposal, "digest": run.Digest, "message": "已选择候选，请核对来源后点击确认匹配；尚未修改影片"}).Error; e != nil {
			return e
		}
		return tx.First(&run, "id = ?", id).Error
	})
	if err != nil {
		return AgentRunView{}, err
	}
	return movieMatchRunView(run), nil
}
func (s *AgentScanService) confirmMovieMatch(ctx context.Context, uid, id, digest string, undo bool) (AgentRunView, error) {
	var run model.AgentRun
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, e := agentScanAdmin(tx, uid)
		if e != nil {
			return e
		}
		if e = tx.Model(&model.AgentRun{}).Where("id = ? AND user_id = ?", id, uid).UpdateColumn("updated_at", gorm.Expr("updated_at")).Error; e != nil {
			return e
		}
		if e = tx.Where("id = ? AND user_id = ?", id, uid).First(&run).Error; e != nil {
			return e
		}
		if run.Kind != "match_movie" || digest == "" || digest != run.Digest || scanDigest(run) != digest || user.TokenVersion != run.TokenVersion {
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
		var proposal agentMovieMatchProposal
		if json.Unmarshal([]byte(run.Snapshot), &snapshot) != nil || json.Unmarshal([]byte(run.Proposal), &proposal) != nil {
			return ErrAgentPreviewChanged
		}
		selected := false
		for _, c := range proposal.Candidates {
			if c.ID > 0 && c.ID == proposal.SelectedID {
				selected = true
			}
		}
		if !selected {
			return ErrAgentMatchSelection
		}
		m, lib, e := loadAgentMovie(tx, snapshot.ID)
		if e != nil {
			return ErrAgentPreviewChanged
		}
		current, e := metadataSnapshot(tx, m, lib)
		if e != nil {
			return e
		}
		var value interface{} = proposal.SelectedID
		if !undo {
			if current != run.Snapshot || snapshot.TMDbID > 0 {
				return ErrAgentPreviewChanged
			}
		} else {
			var after agentMetadataSnapshot
			_ = json.Unmarshal([]byte(current), &after)
			delete(after.Nulls, "tm_db_id")
			originalNulls := map[string]bool{}
			for k, v := range snapshot.Nulls {
				if k != "tm_db_id" {
					originalNulls[k] = v
				}
			}
			if m.LibraryID != snapshot.LibraryID || m.TMDbID != proposal.SelectedID || m.Title != snapshot.Title || m.Year != snapshot.Year || !reflect.DeepEqual(metadataValues(m), snapshot.Values) || !reflect.DeepEqual(after.Nulls, originalNulls) {
				return ErrAgentMatchUndo
			}
			value = snapshot.TMDbID
			if snapshot.Nulls["tm_db_id"] {
				value = nil
			}
		}
		update := tx.Model(&model.Media{}).Where("id = ? AND library_id = ? AND updated_at = ?", m.ID, m.LibraryID, m.UpdatedAt)
		if !undo && snapshot.Nulls["tm_db_id"] {
			update = update.Where("tm_db_id IS NULL")
		} else {
			update = update.Where("tm_db_id = ?", m.TMDbID)
		}
		result := update.Update("tm_db_id", value)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrAgentPreviewChanged
		}
		now := s.now()
		status, message := "completed", "影片关联已保存，现有资料保持原状。可以继续预览资料补全。"
		if undo {
			status, message = "reverted", "本次影片关联已撤销，原有资料和文件保持原状。"
		}
		updates := map[string]interface{}{"status": status, "message": message, "completed_at": now}
		if !undo {
			updates["started_at"] = now
		}
		if e = tx.Model(&model.AgentRun{}).Where("id = ?", id).Updates(updates).Error; e != nil {
			return e
		}
		return tx.First(&run, "id = ?", id).Error
	})
	if err != nil {
		return AgentRunView{}, err
	}
	return movieMatchRunView(run), nil
}
func (s *LibraryService) SearchAgentMovieMatches(ctx context.Context, query string, year int) ([]TMDbMovie, error) {
	if s.metadata == nil {
		return nil, ErrAgentRunUnavailable
	}
	return s.metadata.SearchAgentMovieMatches(ctx, query, year)
}
