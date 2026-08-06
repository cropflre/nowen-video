package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

const (
	LegacySourceRetirementDecisionApprove = "approve"
	LegacySourceRetirementDecisionDefer   = "defer"
	LegacySourceRetirementDecisionReject  = "reject"
)

var (
	ErrLegacySourceRetirementNotFound       = errors.New("legacy source retirement state not found")
	ErrLegacySourceRetirementInvalid        = errors.New("invalid legacy source retirement review")
	ErrLegacySourceRetirementEvidenceStale  = errors.New("legacy source retirement evidence changed")
	ErrLegacySourceRetirementBlocked        = errors.New("legacy source retirement approval is blocked")
)

type LegacySourceBackupVerification struct {
	Verified         bool       `json:"verified"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	RestoreTestedAt  *time.Time `json:"restore_tested_at,omitempty"`
	Reference        string     `json:"reference"`
	Checksum         string     `json:"checksum"`
}

type LegacySourceRetirementReviewRequest struct {
	Decision             string                               `json:"decision"`
	ExpectedEvidenceHash string                               `json:"expected_evidence_hash"`
	Reason               string                               `json:"reason"`
	Backup               LegacySourceBackupVerification       `json:"backup"`
}

type LegacySourceRetirementReport struct {
	ProtocolVersion       string                                      `json:"protocol_version"`
	Source                string                                      `json:"source"`
	GeneratedAt           time.Time                                   `json:"generated_at"`
	Generation            int64                                       `json:"generation"`
	MigrationStatus       string                                      `json:"migration_status"`
	TargetRows            int64                                       `json:"target_rows"`
	ScannedRows           int64                                       `json:"scanned_rows"`
	ObservationStartedAt  *time.Time                                  `json:"observation_started_at,omitempty"`
	ObservationEligibleAt *time.Time                                  `json:"observation_eligible_at,omitempty"`
	ObservationSatisfied  bool                                        `json:"observation_satisfied"`
	SourceTablePresent    bool                                        `json:"source_table_present"`
	SourceRows            int64                                       `json:"source_rows"`
	UnmigratedRows        int64                                       `json:"unmigrated_rows"`
	RollbackOpenArtifacts int64                                       `json:"rollback_open_artifacts"`
	RollbackLatestUntil   *time.Time                                  `json:"rollback_latest_until,omitempty"`
	RollbackWindowClosed  bool                                        `json:"rollback_window_closed"`
	ReadyForBackupReview  bool                                        `json:"ready_for_backup_review"`
	Blockers              []string                                    `json:"blockers"`
	EvidenceHash          string                                      `json:"evidence_hash"`
	LatestDecision        *model.LegacySourceRetirementDecisionRecord `json:"latest_decision,omitempty"`
}

type legacySourceRetirementEvidence struct {
	ProtocolVersion       string     `json:"protocol_version"`
	Source                string     `json:"source"`
	Generation            int64      `json:"generation"`
	MigrationStatus       string     `json:"migration_status"`
	CursorUpdatedAt       *time.Time `json:"cursor_updated_at,omitempty"`
	CursorID              string     `json:"cursor_id"`
	HighWaterUpdatedAt    *time.Time `json:"high_water_updated_at,omitempty"`
	HighWaterID           string     `json:"high_water_id"`
	TargetRows            int64      `json:"target_rows"`
	ScannedRows           int64      `json:"scanned_rows"`
	ObservationStartedAt  *time.Time `json:"observation_started_at,omitempty"`
	ObservationEligibleAt *time.Time `json:"observation_eligible_at,omitempty"`
	ObservationSatisfied  bool       `json:"observation_satisfied"`
	SourceTablePresent    bool       `json:"source_table_present"`
	SourceRows            int64      `json:"source_rows"`
	UnmigratedRows        int64      `json:"unmigrated_rows"`
	RollbackOpenArtifacts int64      `json:"rollback_open_artifacts"`
	RollbackLatestUntil   *time.Time `json:"rollback_latest_until,omitempty"`
	RollbackWindowClosed  bool       `json:"rollback_window_closed"`
}

type LegacySourceRetirementService struct {
	repo  *repository.TranscodeExecutionRepo
	clock func() time.Time
}

func NewLegacySourceRetirementService(repo *repository.TranscodeExecutionRepo) *LegacySourceRetirementService {
	return &LegacySourceRetirementService{
		repo: repo,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func (s *LegacySourceRetirementService) Report(source string) (*LegacySourceRetirementReport, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: repository unavailable", ErrLegacySourceRetirementInvalid)
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("%w: source is required", ErrLegacySourceRetirementInvalid)
	}
	now := s.clock().UTC()
	state, err := s.repo.LegacyProjectionMigrationState(source)
	if err != nil {
		return nil, fmt.Errorf("read migration state: %w", err)
	}
	if state == nil {
		return nil, ErrLegacySourceRetirementNotFound
	}
	inventory, err := s.repo.LegacySourceRetirementInventory(source, now)
	if err != nil {
		return nil, fmt.Errorf("collect retirement inventory: %w", err)
	}
	latestDecision, err := s.repo.LatestLegacySourceRetirementDecision(source)
	if err != nil {
		return nil, fmt.Errorf("read latest retirement decision: %w", err)
	}

	observationSatisfied := state.Status == repository.LegacyProjectionMigrationCompleted &&
		state.QuiescentSince != nil && state.SourceRetireAfter != nil && !now.Before(*state.SourceRetireAfter)
	rollbackClosed := inventory.RollbackOpenArtifacts == 0
	blockers := make([]string, 0, 6)
	if !inventory.SourceTablePresent {
		blockers = append(blockers, "legacy_source_absent")
	}
	if state.Status != repository.LegacyProjectionMigrationCompleted {
		blockers = append(blockers, "migration_not_completed")
	}
	if state.QuiescentSince == nil || state.SourceRetireAfter == nil {
		blockers = append(blockers, "observation_window_missing")
	} else if now.Before(*state.SourceRetireAfter) {
		blockers = append(blockers, "observation_window_open")
	}
	if inventory.UnmigratedRows > 0 {
		blockers = append(blockers, "unmigrated_rows_present")
	}
	if !rollbackClosed {
		blockers = append(blockers, "rollback_window_open")
	}

	evidence := legacySourceRetirementEvidence{
		ProtocolVersion:       model.LegacySourceRetirementProtocolVersion,
		Source:                source,
		Generation:            state.Generation,
		MigrationStatus:       state.Status,
		CursorUpdatedAt:       state.CursorUpdatedAt,
		CursorID:              state.CursorID,
		HighWaterUpdatedAt:    state.HighWaterUpdatedAt,
		HighWaterID:           state.HighWaterID,
		TargetRows:            state.TargetRows,
		ScannedRows:           state.ScannedRows,
		ObservationStartedAt:  state.QuiescentSince,
		ObservationEligibleAt: state.SourceRetireAfter,
		ObservationSatisfied:  observationSatisfied,
		SourceTablePresent:    inventory.SourceTablePresent,
		SourceRows:            inventory.SourceRows,
		UnmigratedRows:        inventory.UnmigratedRows,
		RollbackOpenArtifacts: inventory.RollbackOpenArtifacts,
		RollbackLatestUntil:   inventory.RollbackLatestUntil,
		RollbackWindowClosed:  rollbackClosed,
	}
	evidenceJSON, evidenceHash, err := marshalLegacySourceRetirementEvidence(evidence)
	if err != nil {
		return nil, err
	}
	_ = evidenceJSON

	return &LegacySourceRetirementReport{
		ProtocolVersion:       evidence.ProtocolVersion,
		Source:                source,
		GeneratedAt:           now,
		Generation:            state.Generation,
		MigrationStatus:       state.Status,
		TargetRows:            state.TargetRows,
		ScannedRows:           state.ScannedRows,
		ObservationStartedAt:  state.QuiescentSince,
		ObservationEligibleAt: state.SourceRetireAfter,
		ObservationSatisfied:  observationSatisfied,
		SourceTablePresent:    inventory.SourceTablePresent,
		SourceRows:            inventory.SourceRows,
		UnmigratedRows:        inventory.UnmigratedRows,
		RollbackOpenArtifacts: inventory.RollbackOpenArtifacts,
		RollbackLatestUntil:   inventory.RollbackLatestUntil,
		RollbackWindowClosed:  rollbackClosed,
		ReadyForBackupReview:  len(blockers) == 0,
		Blockers:              blockers,
		EvidenceHash:          evidenceHash,
		LatestDecision:        latestDecision,
	}, nil
}

func (s *LegacySourceRetirementService) Review(
	source string,
	request LegacySourceRetirementReviewRequest,
	reviewerID,
	reviewerName string,
) (*model.LegacySourceRetirementDecisionRecord, error) {
	request.Decision = strings.ToLower(strings.TrimSpace(request.Decision))
	request.ExpectedEvidenceHash = strings.TrimSpace(request.ExpectedEvidenceHash)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.Decision != LegacySourceRetirementDecisionApprove &&
		request.Decision != LegacySourceRetirementDecisionDefer &&
		request.Decision != LegacySourceRetirementDecisionReject {
		return nil, fmt.Errorf("%w: decision must be approve, defer or reject", ErrLegacySourceRetirementInvalid)
	}
	if request.ExpectedEvidenceHash == "" {
		return nil, fmt.Errorf("%w: expected_evidence_hash is required", ErrLegacySourceRetirementInvalid)
	}
	if request.Decision != LegacySourceRetirementDecisionApprove && request.Reason == "" {
		return nil, fmt.Errorf("%w: reason is required for defer or reject", ErrLegacySourceRetirementInvalid)
	}

	report, err := s.Report(source)
	if err != nil {
		return nil, err
	}
	if request.ExpectedEvidenceHash != report.EvidenceHash {
		return nil, ErrLegacySourceRetirementEvidenceStale
	}
	if request.Decision == LegacySourceRetirementDecisionApprove {
		if !report.ReadyForBackupReview {
			return nil, fmt.Errorf("%w: %s", ErrLegacySourceRetirementBlocked, strings.Join(report.Blockers, ","))
		}
		if err := validateLegacySourceBackup(request.Backup, report.GeneratedAt); err != nil {
			return nil, err
		}
	}

	state, err := s.repo.LegacyProjectionMigrationState(report.Source)
	if err != nil {
		return nil, fmt.Errorf("reload migration state: %w", err)
	}
	inventory, err := s.repo.LegacySourceRetirementInventory(report.Source, report.GeneratedAt)
	if err != nil {
		return nil, fmt.Errorf("reload retirement inventory: %w", err)
	}
	evidence := legacySourceRetirementEvidence{
		ProtocolVersion:       report.ProtocolVersion,
		Source:                report.Source,
		Generation:            report.Generation,
		MigrationStatus:       report.MigrationStatus,
		CursorUpdatedAt:       state.CursorUpdatedAt,
		CursorID:              state.CursorID,
		HighWaterUpdatedAt:    state.HighWaterUpdatedAt,
		HighWaterID:           state.HighWaterID,
		TargetRows:            report.TargetRows,
		ScannedRows:           report.ScannedRows,
		ObservationStartedAt:  report.ObservationStartedAt,
		ObservationEligibleAt: report.ObservationEligibleAt,
		ObservationSatisfied:  report.ObservationSatisfied,
		SourceTablePresent:    report.SourceTablePresent,
		SourceRows:            report.SourceRows,
		UnmigratedRows:        report.UnmigratedRows,
		RollbackOpenArtifacts: report.RollbackOpenArtifacts,
		RollbackLatestUntil:   report.RollbackLatestUntil,
		RollbackWindowClosed:  report.RollbackWindowClosed,
	}
	evidenceJSON, evidenceHash, err := marshalLegacySourceRetirementEvidence(evidence)
	if err != nil {
		return nil, err
	}
	if evidenceHash != report.EvidenceHash || inventory.UnmigratedRows != report.UnmigratedRows || inventory.RollbackOpenArtifacts != report.RollbackOpenArtifacts {
		return nil, ErrLegacySourceRetirementEvidenceStale
	}

	now := s.clock().UTC()
	record := &model.LegacySourceRetirementDecisionRecord{
		ProtocolVersion:       report.ProtocolVersion,
		Source:                report.Source,
		Generation:            report.Generation,
		Decision:              request.Decision,
		EvidenceHash:          evidenceHash,
		EvidenceJSON:          string(evidenceJSON),
		ObservationStartedAt:  report.ObservationStartedAt,
		ObservationEligibleAt: report.ObservationEligibleAt,
		MigrationStatus:       report.MigrationStatus,
		TargetRows:            report.TargetRows,
		ScannedRows:           report.ScannedRows,
		UnmigratedRows:        report.UnmigratedRows,
		RollbackOpenArtifacts: report.RollbackOpenArtifacts,
		RollbackLatestUntil:   report.RollbackLatestUntil,
		BackupVerified:        request.Backup.Verified,
		BackupVerifiedAt:      request.Backup.VerifiedAt,
		BackupRestoreTestedAt: request.Backup.RestoreTestedAt,
		BackupReference:       strings.TrimSpace(request.Backup.Reference),
		BackupChecksum:        strings.TrimSpace(request.Backup.Checksum),
		ReviewerID:            strings.TrimSpace(reviewerID),
		ReviewerName:          strings.TrimSpace(reviewerName),
		Reason:                request.Reason,
		ReviewedAt:            now,
		CreatedAt:             now,
	}
	if err := s.repo.CreateLegacySourceRetirementDecision(record); err != nil {
		return nil, fmt.Errorf("persist retirement decision: %w", err)
	}
	return record, nil
}

func validateLegacySourceBackup(backup LegacySourceBackupVerification, now time.Time) error {
	if !backup.Verified || backup.VerifiedAt == nil || backup.RestoreTestedAt == nil ||
		strings.TrimSpace(backup.Reference) == "" || strings.TrimSpace(backup.Checksum) == "" {
		return fmt.Errorf("%w: approval requires verified backup reference, checksum, verification time and restore-test time", ErrLegacySourceRetirementInvalid)
	}
	futureLimit := now.Add(5 * time.Minute)
	if backup.VerifiedAt.After(futureLimit) || backup.RestoreTestedAt.After(futureLimit) {
		return fmt.Errorf("%w: backup evidence cannot be in the future", ErrLegacySourceRetirementInvalid)
	}
	return nil
}

func marshalLegacySourceRetirementEvidence(evidence legacySourceRetirementEvidence) ([]byte, string, error) {
	payload, err := json.Marshal(evidence)
	if err != nil {
		return nil, "", fmt.Errorf("marshal retirement evidence: %w", err)
	}
	digest := sha256.Sum256(payload)
	return payload, hex.EncodeToString(digest[:]), nil
}
