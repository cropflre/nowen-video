package service

import (
	"errors"
	"fmt"
	"time"

	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
)

var ErrArtifactCleanupNotRetryable = errors.New("artifact cleanup is not retryable")

// RetryArtifactCleanup is the operator-facing recovery command used by the
// unified task center. It never bypasses Artifact Store path validation and it
// never deletes metadata directly. The command only requeues blocked/retrying
// work and immediately enters the normal Cleanup Lease flow.
func (s *ArtifactMaintenanceService) RetryArtifactCleanup(artifactID string) error {
	if s == nil || s.executionRepo == nil {
		return fmt.Errorf("Artifact 清理服务不可用")
	}
	now := time.Now()
	artifact, requeued, err := s.executionRepo.RequeueArtifactCleanup(artifactID, now)
	if err != nil {
		return fmt.Errorf("重新排队 Artifact 清理失败: %w", err)
	}
	if !requeued || artifact == nil {
		return fmt.Errorf("%w: artifact=%s", ErrArtifactCleanupNotRetryable, artifactID)
	}

	_, cleanupErr := s.cleanupArtifactRecord(artifact)
	if cleanupErr == nil {
		return nil
	}

	// cleanupArtifactRecord persists retry_wait or blocked before returning an
	// error. Treat that as an accepted operator attempt; the refreshed task view
	// exposes the new durable state and evidence. Only failures that did not
	// produce a durable cleanup record escape as infrastructure errors.
	stored, lookupErr := s.executionRepo.FindArtifactCleanupOperation(artifactID)
	if lookupErr == nil && stored != nil {
		switch stored.CleanupState {
		case repository.ArtifactCleanupRetryWait, repository.ArtifactCleanupBlocked:
			s.logger.Warnf("管理员重试 Artifact 清理后仍需后续处置 artifact=%s state=%s: %v", artifactID, stored.CleanupState, cleanupErr)
			return nil
		}
	}
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		// The operation is no longer active because another cleanup owner completed
		// the tombstone after requeue. This is an idempotent success.
		return nil
	}
	if lookupErr != nil {
		return fmt.Errorf("读取 Artifact 清理重试结果失败: %w", lookupErr)
	}
	return cleanupErr
}

var ErrLegacyArtifactRollbackUnavailable = errors.New("legacy artifact rollback is unavailable")

func (s *ArtifactMaintenanceService) RollbackLegacyArtifactMigration(artifactID string) error {
	if s == nil || s.executionRepo == nil {
		return fmt.Errorf("Legacy Artifact 迁移服务不可用")
	}
	rolledBack, err := s.executionRepo.RollbackLegacyArtifactCleanup(artifactID, time.Now())
	if err != nil {
		return fmt.Errorf("回滚 Legacy Artifact 清理失败: %w", err)
	}
	if !rolledBack {
		return fmt.Errorf("%w: artifact=%s", ErrLegacyArtifactRollbackUnavailable, artifactID)
	}
	return nil
}
