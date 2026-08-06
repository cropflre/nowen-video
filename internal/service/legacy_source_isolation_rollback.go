package service

import (
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

// RollbackIsolation restores the archived table name after checking the exact
// isolation record, schema hash and row count. Rollback remains available even
// after the removal plan expires because it is the emergency downgrade path.
func (s *LegacySourceRetirementService) RollbackIsolation(
	source string,
	request LegacySourceIsolationRollbackRequest,
	reviewerID,
	reviewerName string,
) (*model.LegacySourceIsolationRollbackRecord, error) {
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
	request.ExpectedIsolationID = strings.TrimSpace(request.ExpectedIsolationID)
	request.ExpectedSchemaHash = strings.TrimSpace(request.ExpectedSchemaHash)
	request.Confirmation = strings.TrimSpace(request.Confirmation)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.ExpectedIsolationID == "" || request.ExpectedSchemaHash == "" || request.Reason == "" {
		return nil, fmt.Errorf("%w: expected_isolation_id, expected_schema_hash and reason are required", ErrLegacySourceRetirementInvalid)
	}
	if request.Confirmation != LegacySourceIsolationRollbackConfirmation {
		return nil, fmt.Errorf("%w: confirmation must equal %q", ErrLegacySourceRetirementInvalid, LegacySourceIsolationRollbackConfirmation)
	}
	if err := s.repo.EnsureLegacySourceIsolationSchema(); err != nil {
		return nil, fmt.Errorf("ensure isolation schema: %w", err)
	}

	var rolledBack *model.LegacySourceIsolationRollbackRecord
	err = s.repo.InTransaction(func(txRepo *repository.TranscodeExecutionRepo) error {
		isolation, err := txRepo.LegacySourceIsolationByID(resolvedSource, request.ExpectedIsolationID, true)
		if err != nil {
			return fmt.Errorf("read source isolation: %w", err)
		}
		if isolation == nil {
			return ErrLegacySourceIsolationNotFound
		}
		if isolation.ProtocolVersion != model.LegacySourceIsolationProtocolVersion ||
			isolation.Status != LegacySourceIsolationStatusIsolated ||
			isolation.SchemaHash != request.ExpectedSchemaHash {
			return fmt.Errorf("%w: isolation evidence does not match", ErrLegacySourceIsolationBlocked)
		}
		existing, err := txRepo.LegacySourceIsolationRollbackByIsolationID(resolvedSource, isolation.ID)
		if err != nil {
			return fmt.Errorf("read source isolation rollback: %w", err)
		}
		if existing != nil {
			tables, err := txRepo.LegacySourceTableState()
			if err != nil {
				return err
			}
			if tables.OriginalTablePresent && !tables.ArchiveTablePresent && existing.SchemaHash == request.ExpectedSchemaHash {
				rolledBack = existing
				return nil
			}
			return fmt.Errorf("%w: rollback record and table state disagree", ErrLegacySourceIsolationConflict)
		}

		tables, err := txRepo.LegacySourceTableState()
		if err != nil {
			return err
		}
		if tables.OriginalTablePresent || !tables.ArchiveTablePresent {
			return fmt.Errorf("%w: expected archive table only", ErrLegacySourceIsolationConflict)
		}
		rows, err := txRepo.LegacySourceRowCount(repository.LegacySourceArchiveTable)
		if err != nil {
			return fmt.Errorf("count archived source rows: %w", err)
		}
		schema, err := txRepo.LegacySourceArchiveSchemaSnapshot()
		if err != nil {
			return fmt.Errorf("collect archived source schema: %w", err)
		}
		_, schemaHash, err := marshalHashedEvidence(schema, "archived legacy source schema")
		if err != nil {
			return err
		}
		if rows != isolation.SourceRows || schemaHash != isolation.SchemaHash || schemaHash != request.ExpectedSchemaHash {
			return ErrLegacySourceRetirementEvidenceStale
		}

		if err := txRepo.RestoreLegacySourceFromArchive(); err != nil {
			return fmt.Errorf("%w: restore legacy source: %v", ErrLegacySourceIsolationConflict, err)
		}
		postTables, err := txRepo.LegacySourceTableState()
		if err != nil {
			return err
		}
		if !postTables.OriginalTablePresent || postTables.ArchiveTablePresent {
			return fmt.Errorf("%w: original table state is invalid after restore", ErrLegacySourceIsolationConflict)
		}
		postRows, err := txRepo.LegacySourceRowCount(repository.LegacySourceOriginalTable)
		if err != nil {
			return err
		}
		postSchema, err := txRepo.LegacySourceSchemaSnapshot()
		if err != nil {
			return err
		}
		_, postSchemaHash, err := marshalHashedEvidence(postSchema, "restored legacy source schema")
		if err != nil {
			return err
		}
		if postRows != rows || postSchemaHash != isolation.SchemaHash {
			return ErrLegacySourceRetirementEvidenceStale
		}

		now := s.now()
		record := &model.LegacySourceIsolationRollbackRecord{
			ProtocolVersion: model.LegacySourceIsolationProtocolVersion,
			Source:          resolvedSource,
			IsolationID:     isolation.ID,
			RemovalPlanID:   isolation.RemovalPlanID,
			SchemaHash:      isolation.SchemaHash,
			SourceRows:      postRows,
			OriginalTable:   repository.LegacySourceOriginalTable,
			ArchiveTable:    repository.LegacySourceArchiveTable,
			ReviewerID:      strings.TrimSpace(reviewerID),
			ReviewerName:    strings.TrimSpace(reviewerName),
			Reason:          request.Reason,
			RolledBackAt:    now,
			CreatedAt:       now,
		}
		if err := txRepo.CreateLegacySourceIsolationRollback(record); err != nil {
			return fmt.Errorf("persist source isolation rollback: %w", err)
		}
		rolledBack = record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rolledBack, nil
}
