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
	LegacySourceRemovalPlanStatusPrepared = "prepared"
	legacySourceRemovalPlanLifetime       = 24 * time.Hour
)

var (
	ErrLegacySourceRetirementNotFound      = errors.New("legacy source retirement state not found")
	ErrLegacySourceRetirementInvalid       = errors.New("invalid legacy source retirement review")
	ErrLegacySourceRetirementEvidenceStale = errors.New("legacy source retirement evidence changed")
	ErrLegacySourceRetirementBlocked       = errors.New("legacy source retirement approval is blocked")
	ErrLegacySourceRemovalPlanBlocked      = errors.New("legacy source removal plan is blocked")
)

type LegacySourceBackupVerification struct {
	Verified        bool       `json:"verified"`
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	RestoreTestedAt *time.Time `json:"restore_tested_at,omitempty"`
	Reference       string     `json:"reference"`
	Checksum        string     `json:"checksum"`
}

type LegacySourceRetirementReviewRequest struct {
	Decision             string                         `json:"decision"`
	ExpectedEvidenceHash string                         `json:"expected_evidence_hash"`
	Reason               string                         `json:"reason"`
	Backup               LegacySourceBackupVerification `json:"backup"`
}

type LegacySourceRemovalPlanRequest struct {
	ExpectedEvidenceHash string `json:"expected_evidence_hash"`
	ExpectedDecisionID   string `json:"expected_decision_id"`
	Reason               string `json:"reason"`
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
	LatestRemovalPlan     *model.LegacySourceRemovalPlanRecord         `json:"latest_removal_plan,omitempty"`
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
		repo:  repo,
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func (s *LegacySourceRetirementService) Report(source string) (*LegacySourceRetirementReport, error) {
	report, _, err := s.collectReport(source, s.now())
	return report, err
}

func (s *LegacySourceRetirementService) Review(
	source string,
	request LegacySourceRetirementReviewRequest,
	reviewerID,
	reviewerName string,
) (*model.LegacySourceRetirementDecisionRecord, error) {
	if err := validateLegacySourceReviewer(reviewerID); err != nil {
		return nil, err
	}
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

	initialReport, _, err := s.collectReport(source, s.now())
	if err != nil {
		return nil, err
	}
	if request.ExpectedEvidenceHash != initialReport.EvidenceHash {
		return nil, ErrLegacySourceRetirementEvidenceStale
	}
	if request.Decision == LegacySourceRetirementDecisionApprove {
		if err := validateLegacySourceApproval(initialReport, request.Backup); err != nil {
			return nil, err
		}
	}

	// Recompute the complete snapshot immediately before persistence. Do not mix
	// fields from the first report with freshly loaded cursor or inventory data.
	freshReport, freshEvidenceJSON, err := s.collectReport(source, s.now())
	if err != nil {
		return nil, err
	}
	if freshReport.EvidenceHash != request.ExpectedEvidenceHash {
		return nil, ErrLegacySourceRetirementEvidenceStale
	}
	if request.Decision == LegacySourceRetirementDecisionApprove {
		if err := validateLegacySourceApproval(freshReport, request.Backup); err != nil {
			return nil, err
		}
	}

	now := freshReport.GeneratedAt
	record := &model.LegacySourceRetirementDecisionRecord{
		ProtocolVersion:       freshReport.ProtocolVersion,
		Source:                freshReport.Source,
		Generation:            freshReport.Generation,
		Decision:              request.Decision,
		EvidenceHash:          freshReport.EvidenceHash,
		EvidenceJSON:          string(freshEvidenceJSON),
		ObservationStartedAt:  freshReport.ObservationStartedAt,
		ObservationEligibleAt: freshReport.ObservationEligibleAt,
		MigrationStatus:       freshReport.MigrationStatus,
		TargetRows:            freshReport.TargetRows,
		ScannedRows:           freshReport.ScannedRows,
		UnmigratedRows:        freshReport.UnmigratedRows,
		RollbackOpenArtifacts: freshReport.RollbackOpenArtifacts,
		RollbackLatestUntil:   freshReport.RollbackLatestUntil,
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

// PrepareRemovalPlan creates an immutable, short-lived handoff package for a
// later schema migration. It does not delete rows or execute DROP TABLE.
func (s *LegacySourceRetirementService) PrepareRemovalPlan(
	source string,
	request LegacySourceRemovalPlanRequest,
	reviewerID,
	reviewerName string,
) (*model.LegacySourceRemovalPlanRecord, error) {
	if err := validateLegacySourceReviewer(reviewerID); err != nil {
		return nil, err
	}
	request.ExpectedEvidenceHash = strings.TrimSpace(request.ExpectedEvidenceHash)
	request.ExpectedDecisionID = strings.TrimSpace(request.ExpectedDecisionID)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.ExpectedEvidenceHash == "" || request.ExpectedDecisionID == "" || request.Reason == "" {
		return nil, fmt.Errorf("%w: expected_evidence_hash, expected_decision_id and reason are required", ErrLegacySourceRetirementInvalid)
	}

	initialReport, _, err := s.collectReport(source, s.now())
	if err != nil {
		return nil, err
	}
	if err := validateLegacySourceRemovalPlanReport(initialReport, request); err != nil {
		return nil, err
	}
	initialSchema, initialSchemaJSON, initialSchemaHash, err := s.collectSchemaEvidence()
	if err != nil {
		return nil, err
	}
	if !initialSchema.TablePresent || len(initialSchema.Columns) == 0 {
		return nil, fmt.Errorf("%w: legacy source schema is unavailable", ErrLegacySourceRemovalPlanBlocked)
	}

	// Re-read both data evidence and schema evidence so the plan cannot combine a
	// reviewed retirement hash with a different table shape.
	freshReport, freshEvidenceJSON, err := s.collectReport(source, s.now())
	if err != nil {
		return nil, err
	}
	if err := validateLegacySourceRemovalPlanReport(freshReport, request); err != nil {
		return nil, err
	}
	freshSchema, freshSchemaJSON, freshSchemaHash, err := s.collectSchemaEvidence()
	if err != nil {
		return nil, err
	}
	if freshReport.EvidenceHash != initialReport.EvidenceHash ||
		freshSchemaHash != initialSchemaHash ||
		!freshSchema.TablePresent || len(freshSchema.Columns) == 0 {
		return nil, ErrLegacySourceRetirementEvidenceStale
	}
	_ = initialSchemaJSON

	preparedAt := freshReport.GeneratedAt
	expiresAt := preparedAt.Add(legacySourceRemovalPlanLifetime)
	decision := freshReport.LatestDecision
	plan := &model.LegacySourceRemovalPlanRecord{
		ProtocolVersion:        model.LegacySourceRemovalPlanProtocolVersion,
		Source:                 freshReport.Source,
		Generation:             freshReport.Generation,
		Status:                 LegacySourceRemovalPlanStatusPrepared,
		RetirementDecisionID:   decision.ID,
		RetirementDecisionHash: decision.EvidenceHash,
		EvidenceHash:           freshReport.EvidenceHash,
		EvidenceJSON:           string(freshEvidenceJSON),
		SchemaHash:             freshSchemaHash,
		SchemaJSON:             string(freshSchemaJSON),
		SourceRows:             freshReport.SourceRows,
		ReviewerID:             strings.TrimSpace(reviewerID),
		ReviewerName:           strings.TrimSpace(reviewerName),
		Reason:                 request.Reason,
		PreparedAt:             preparedAt,
		ExpiresAt:              &expiresAt,
		CreatedAt:              preparedAt,
	}
	if err := s.repo.CreateLegacySourceRemovalPlan(plan); err != nil {
		return nil, fmt.Errorf("persist legacy source removal plan: %w", err)
	}
	return plan, nil
}

func (s *LegacySourceRetirementService) collectReport(source string, now time.Time) (*LegacySourceRetirementReport, []byte, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("%w: repository unavailable", ErrLegacySourceRetirementInvalid)
	}
	resolvedSource, err := normalizeLegacySource(source)
	if err != nil {
		return nil, nil, err
	}
	now = now.UTC()
	state, err := s.repo.LegacyProjectionMigrationState(resolvedSource)
	if err != nil {
		return nil, nil, fmt.Errorf("read migration state: %w", err)
	}
	if state == nil {
		return nil, nil, ErrLegacySourceRetirementNotFound
	}
	inventory, err := s.repo.LegacySourceRetirementInventory(resolvedSource, now)
	if err != nil {
		return nil, nil, fmt.Errorf("collect retirement inventory: %w", err)
	}
	latestDecision, err := s.repo.LatestLegacySourceRetirementDecision(resolvedSource)
	if err != nil {
		return nil, nil, fmt.Errorf("read latest retirement decision: %w", err)
	}
	latestPlan, err := s.repo.LatestLegacySourceRemovalPlan(resolvedSource)
	if err != nil {
		return nil, nil, fmt.Errorf("read latest source removal plan: %w", err)
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
		Source:                resolvedSource,
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
	evidenceJSON, evidenceHash, err := marshalHashedEvidence(evidence, "retirement evidence")
	if err != nil {
		return nil, nil, err
	}

	return &LegacySourceRetirementReport{
		ProtocolVersion:       evidence.ProtocolVersion,
		Source:                resolvedSource,
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
		LatestRemovalPlan:     latestPlan,
	}, evidenceJSON, nil
}

func (s *LegacySourceRetirementService) collectSchemaEvidence() (repository.LegacySourceSchemaSnapshot, []byte, string, error) {
	snapshot, err := s.repo.LegacySourceSchemaSnapshot()
	if err != nil {
		return snapshot, nil, "", fmt.Errorf("collect legacy source schema: %w", err)
	}
	payload, hash, err := marshalHashedEvidence(snapshot, "legacy source schema")
	return snapshot, payload, hash, err
}

func validateLegacySourceApproval(report *LegacySourceRetirementReport, backup LegacySourceBackupVerification) error {
	if !report.ReadyForBackupReview {
		return fmt.Errorf("%w: %s", ErrLegacySourceRetirementBlocked, strings.Join(report.Blockers, ","))
	}
	return validateLegacySourceBackup(backup, report.GeneratedAt)
}

func validateLegacySourceRemovalPlanReport(report *LegacySourceRetirementReport, request LegacySourceRemovalPlanRequest) error {
	if request.ExpectedEvidenceHash != report.EvidenceHash {
		return ErrLegacySourceRetirementEvidenceStale
	}
	if !report.ReadyForBackupReview {
		return fmt.Errorf("%w: %s", ErrLegacySourceRemovalPlanBlocked, strings.Join(report.Blockers, ","))
	}
	decision := report.LatestDecision
	if decision == nil || decision.Decision != LegacySourceRetirementDecisionApprove {
		return fmt.Errorf("%w: latest retirement decision is not approved", ErrLegacySourceRemovalPlanBlocked)
	}
	if decision.ID != request.ExpectedDecisionID {
		return ErrLegacySourceRetirementEvidenceStale
	}
	if decision.ProtocolVersion != model.LegacySourceRetirementProtocolVersion ||
		decision.Generation != report.Generation ||
		decision.EvidenceHash != report.EvidenceHash ||
		!decision.BackupVerified || decision.BackupVerifiedAt == nil || decision.BackupRestoreTestedAt == nil {
		return fmt.Errorf("%w: approved decision does not match fresh evidence", ErrLegacySourceRemovalPlanBlocked)
	}
	return nil
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
	if backup.RestoreTestedAt.Before(*backup.VerifiedAt) {
		return fmt.Errorf("%w: restore-test time cannot precede backup verification", ErrLegacySourceRetirementInvalid)
	}
	return nil
}

func validateLegacySourceReviewer(reviewerID string) error {
	if strings.TrimSpace(reviewerID) == "" {
		return fmt.Errorf("%w: authenticated reviewer is required", ErrLegacySourceRetirementInvalid)
	}
	return nil
}

func normalizeLegacySource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("%w: source is required", ErrLegacySourceRetirementInvalid)
	}
	if source != repository.LegacyTranscodeArtifactMigrationSource {
		return "", fmt.Errorf("%w: unsupported legacy source %q", ErrLegacySourceRetirementInvalid, source)
	}
	return source, nil
}

func (s *LegacySourceRetirementService) now() time.Time {
	if s == nil || s.clock == nil {
		return time.Now().UTC()
	}
	return s.clock().UTC()
}

func marshalHashedEvidence(value any, label string) ([]byte, string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, "", fmt.Errorf("marshal %s: %w", label, err)
	}
	digest := sha256.Sum256(payload)
	return payload, hex.EncodeToString(digest[:]), nil
}
