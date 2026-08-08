package service

import (
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

// Isolate performs the first reversible schema action in the retirement
// protocol. It validates the approved, unexpired removal plan again inside one
// database transaction, renames the source table, verifies the archived table,
// and appends an audit record. It never drops a table or deletes a row.
func (s *LegacySourceRetirementService) Isolate(
	source string,
	request LegacySourceIsolationRequest,
	reviewerID,
	reviewerName string,
) (*model.LegacySourceIsolationRecord, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: repository unavailable", ErrLegacySourceRetirementInvalid)
	}
	if err := validateLegacySourceReviewer(reviewerID); err != nil {
		return nil, err
	}
	resolvedSource, err := normalizeLegacySource(source)
	if err != nil {
		return nil, err
	}
	request.ExpectedPlanID = strings.TrimSpace(request.ExpectedPlanID)
	request.ExpectedEvidenceHash = strings.TrimSpace(request.ExpectedEvidenceHash)
	request.ExpectedSchemaHash = strings.TrimSpace(request.ExpectedSchemaHash)
	request.Confirmation = strings.TrimSpace(request.Confirmation)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.ExpectedPlanID == "" || request.ExpectedEvidenceHash == "" ||
		request.ExpectedSchemaHash == "" || request.Reason == "" {
		return nil, fmt.Errorf("%w: expected_plan_id, expected_evidence_hash, expected_schema_hash and reason are required", ErrLegacySourceRetirementInvalid)
	}
	if request.Confirmation != LegacySourceIsolationConfirmation {
		return nil, fmt.Errorf("%w: confirmation must equal %q", ErrLegacySourceRetirementInvalid, LegacySourceIsolationConfirmation)
	}
	if err := s.repo.EnsureLegacySourceIsolationSchema(); err != nil {
		return nil, fmt.Errorf("ensure isolation schema: %w", err)
	}

	var isolated *model.LegacySourceIsolationRecord
	err = s.repo.InTransaction(func(txRepo *repository.TranscodeExecutionRepo) error {
		txService := NewLegacySourceRetirementService(txRepo)
		txService.clock = s.clock
		now := txService.now()

		plan, err := txRepo.LegacySourceRemovalPlanByID(resolvedSource, request.ExpectedPlanID, true)
		if err != nil {
			return fmt.Errorf("read removal plan: %w", err)
		}
		if plan == nil {
			return ErrLegacySourceIsolationNotFound
		}
		if err := validateIsolationPlanIdentity(plan, request); err != nil {
			return err
		}

		existing, err := txRepo.LegacySourceIsolationByPlanID(resolvedSource, plan.ID)
		if err != nil {
			return fmt.Errorf("read existing source isolation: %w", err)
		}
		if existing != nil {
			rollback, err := txRepo.LegacySourceIsolationRollbackByIsolationID(resolvedSource, existing.ID)
			if err != nil {
				return fmt.Errorf("read isolation rollback: %w", err)
			}
			tables, err := txRepo.LegacySourceTableState()
			if err != nil {
				return err
			}
			if rollback == nil && !tables.OriginalTablePresent && tables.ArchiveTablePresent &&
				existing.EvidenceHash == request.ExpectedEvidenceHash && existing.SchemaHash == request.ExpectedSchemaHash {
				isolated = existing
				return nil
			}
			return fmt.Errorf("%w: removal plan was already consumed", ErrLegacySourceIsolationConflict)
		}

		if plan.ExpiresAt == nil || !now.Before(*plan.ExpiresAt) {
			return fmt.Errorf("%w: removal plan expired", ErrLegacySourceIsolationBlocked)
		}
		latestPlan, err := txRepo.LatestLegacySourceRemovalPlanForIsolation(resolvedSource, true)
		if err != nil {
			return fmt.Errorf("read latest removal plan: %w", err)
		}
		if latestPlan == nil || latestPlan.ID != plan.ID {
			return fmt.Errorf("%w: removal plan is no longer the latest plan", ErrLegacySourceIsolationBlocked)
		}

		tables, err := txRepo.LegacySourceTableState()
		if err != nil {
			return fmt.Errorf("read source table state: %w", err)
		}
		if !tables.OriginalTablePresent || tables.ArchiveTablePresent {
			return fmt.Errorf("%w: expected original table only", ErrLegacySourceIsolationConflict)
		}

		report, _, err := txService.collectReport(resolvedSource, now)
		if err != nil {
			return err
		}
		planRequest := LegacySourceRemovalPlanRequest{
			ExpectedEvidenceHash: request.ExpectedEvidenceHash,
			ExpectedDecisionID:   plan.RetirementDecisionID,
			Reason:               plan.Reason,
		}
		if err := validateLegacySourceRemovalPlanReport(report, planRequest); err != nil {
			return err
		}
		if report.LatestRemovalPlan == nil || report.LatestRemovalPlan.ID != plan.ID {
			return fmt.Errorf("%w: removal plan changed", ErrLegacySourceIsolationBlocked)
		}
		if report.EvidenceHash != plan.EvidenceHash || report.Generation != plan.Generation ||
			report.SourceRows != plan.SourceRows {
			return ErrLegacySourceRetirementEvidenceStale
		}
		schema, _, schemaHash, err := txService.collectSchemaEvidence()
		if err != nil {
			return err
		}
		if !schema.TablePresent || schemaHash != plan.SchemaHash || schemaHash != request.ExpectedSchemaHash {
			return ErrLegacySourceRetirementEvidenceStale
		}
		rows, err := txRepo.LegacySourceRowCount(repository.LegacySourceOriginalTable)
		if err != nil {
			return fmt.Errorf("count legacy source rows: %w", err)
		}
		if rows != plan.SourceRows {
			return ErrLegacySourceRetirementEvidenceStale
		}

		// The rename is followed by post-DDL verification in the same transaction.
		// If a concurrent legacy writer changed rows or schema after the final
		// evidence read, returning an error rolls the rename back.
		if err := txRepo.RenameLegacySourceToArchive(); err != nil {
			return fmt.Errorf("%w: rename legacy source: %v", ErrLegacySourceIsolationConflict, err)
		}
		postTables, err := txRepo.LegacySourceTableState()
		if err != nil {
			return err
		}
		if postTables.OriginalTablePresent || !postTables.ArchiveTablePresent {
			return fmt.Errorf("%w: archive table state is invalid after rename", ErrLegacySourceIsolationConflict)
		}
		archiveRows, err := txRepo.LegacySourceRowCount(repository.LegacySourceArchiveTable)
		if err != nil {
			return fmt.Errorf("count archived source rows: %w", err)
		}
		archiveSchema, err := txRepo.LegacySourceArchiveSchemaSnapshot()
		if err != nil {
			return fmt.Errorf("collect archived source schema: %w", err)
		}
		_, archiveSchemaHash, err := marshalHashedEvidence(archiveSchema, "archived legacy source schema")
		if err != nil {
			return err
		}
		if archiveRows != rows || archiveRows != plan.SourceRows || archiveSchemaHash != plan.SchemaHash {
			return ErrLegacySourceRetirementEvidenceStale
		}

		decision := report.LatestDecision
		record := &model.LegacySourceIsolationRecord{
			ProtocolVersion:        model.LegacySourceIsolationProtocolVersion,
			Source:                 resolvedSource,
			Generation:             plan.Generation,
			Status:                 LegacySourceIsolationStatusIsolated,
			RemovalPlanID:          plan.ID,
			RetirementDecisionID:   plan.RetirementDecisionID,
			RetirementDecisionHash: plan.RetirementDecisionHash,
			EvidenceHash:           plan.EvidenceHash,
			SchemaHash:             plan.SchemaHash,
			SourceRows:             archiveRows,
			OriginalTable:          repository.LegacySourceOriginalTable,
			ArchiveTable:           repository.LegacySourceArchiveTable,
			BackupReference:        decision.BackupReference,
			BackupChecksum:         decision.BackupChecksum,
			ReviewerID:             strings.TrimSpace(reviewerID),
			ReviewerName:           strings.TrimSpace(reviewerName),
			Reason:                 request.Reason,
			IsolatedAt:             now,
			CreatedAt:              now,
		}
		if err := txRepo.CreateLegacySourceIsolation(record); err != nil {
			return fmt.Errorf("persist source isolation: %w", err)
		}
		isolated = record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return isolated, nil
}

func validateIsolationPlanIdentity(plan *model.LegacySourceRemovalPlanRecord, request LegacySourceIsolationRequest) error {
	if plan.ProtocolVersion != model.LegacySourceRemovalPlanProtocolVersion ||
		plan.Status != LegacySourceRemovalPlanStatusPrepared {
		return fmt.Errorf("%w: removal plan protocol or status is invalid", ErrLegacySourceIsolationBlocked)
	}
	if plan.EvidenceHash != request.ExpectedEvidenceHash || plan.SchemaHash != request.ExpectedSchemaHash {
		return ErrLegacySourceRetirementEvidenceStale
	}
	return nil
}
