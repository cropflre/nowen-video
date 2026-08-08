package service

import (
	"errors"
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
)

const (
	LegacySourceIsolationStatusIsolated       = "isolated"
	LegacySourceIsolationConfirmation         = "ISOLATE transcode_tasks AS legacy_transcode_tasks_retired_v1"
	LegacySourceIsolationRollbackConfirmation = "RESTORE legacy_transcode_tasks_retired_v1 AS transcode_tasks"
)

var (
	ErrLegacySourceIsolationNotFound = errors.New("legacy source isolation state not found")
	ErrLegacySourceIsolationBlocked  = errors.New("legacy source isolation is blocked")
	ErrLegacySourceIsolationConflict = errors.New("legacy source isolation table state conflicts")
)

type LegacySourceIsolationRequest struct {
	ExpectedPlanID       string `json:"expected_plan_id"`
	ExpectedEvidenceHash string `json:"expected_evidence_hash"`
	ExpectedSchemaHash   string `json:"expected_schema_hash"`
	Confirmation         string `json:"confirmation"`
	Reason               string `json:"reason"`
}

type LegacySourceIsolationRollbackRequest struct {
	ExpectedIsolationID string `json:"expected_isolation_id"`
	ExpectedSchemaHash  string `json:"expected_schema_hash"`
	Confirmation        string `json:"confirmation"`
	Reason              string `json:"reason"`
}

type LegacySourceIsolationState struct {
	ProtocolVersion      string                                     `json:"protocol_version"`
	Source               string                                     `json:"source"`
	OriginalTable        string                                     `json:"original_table"`
	ArchiveTable         string                                     `json:"archive_table"`
	OriginalTablePresent bool                                       `json:"original_table_present"`
	ArchiveTablePresent  bool                                       `json:"archive_table_present"`
	IsolationActive      bool                                       `json:"isolation_active"`
	LatestIsolation      *model.LegacySourceIsolationRecord         `json:"latest_isolation,omitempty"`
	LatestRollback       *model.LegacySourceIsolationRollbackRecord `json:"latest_rollback,omitempty"`
}

func (s *LegacySourceRetirementService) IsolationState(source string) (*LegacySourceIsolationState, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("%w: repository unavailable", ErrLegacySourceRetirementInvalid)
	}
	resolvedSource, err := normalizeLegacySource(source)
	if err != nil {
		return nil, err
	}
	if err := s.repo.EnsureLegacySourceIsolationSchema(); err != nil {
		return nil, fmt.Errorf("ensure isolation schema: %w", err)
	}
	tables, err := s.repo.LegacySourceTableState()
	if err != nil {
		return nil, fmt.Errorf("read legacy source table state: %w", err)
	}
	isolation, err := s.repo.LatestLegacySourceIsolation(resolvedSource)
	if err != nil {
		return nil, fmt.Errorf("read latest source isolation: %w", err)
	}
	rollback, err := s.repo.LatestLegacySourceIsolationRollback(resolvedSource)
	if err != nil {
		return nil, fmt.Errorf("read latest source isolation rollback: %w", err)
	}
	active := isolation != nil &&
		(rollback == nil || rollback.IsolationID != isolation.ID) &&
		!tables.OriginalTablePresent && tables.ArchiveTablePresent
	return &LegacySourceIsolationState{
		ProtocolVersion:      model.LegacySourceIsolationProtocolVersion,
		Source:               resolvedSource,
		OriginalTable:        repository.LegacySourceOriginalTable,
		ArchiveTable:         repository.LegacySourceArchiveTable,
		OriginalTablePresent: tables.OriginalTablePresent,
		ArchiveTablePresent:  tables.ArchiveTablePresent,
		IsolationActive:      active,
		LatestIsolation:      isolation,
		LatestRollback:       rollback,
	}, nil
}
