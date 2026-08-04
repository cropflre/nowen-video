package service

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
	transcodestorageestimate "github.com/nowen-video/nowen-video/internal/transcode/storageestimate"
)

var ErrTranscodeStorageReservationUnavailable = errors.New("transcode storage reservation is unavailable")

type TranscodeStorageReservationStatus struct {
	ActiveCount             int64   `json:"active_count"`
	ActiveBytes             int64   `json:"active_bytes"`
	ReservedBytes           int64   `json:"reserved_bytes"`
	ObservedBytes           int64   `json:"observed_bytes"`
	RemainingBytes          int64   `json:"remaining_bytes"`
	WaitingCount            int64   `json:"waiting_count"`
	AvailableHeadroom       int64   `json:"available_headroom_bytes"`
	CalibrationSamples      int64   `json:"calibration_samples"`
	AverageActualToEstimate float64 `json:"average_actual_to_estimate"`
	AverageAbsoluteError    float64 `json:"average_absolute_error"`
	UnderpredictedCount     int64   `json:"underpredicted_count"`
}

func (s *TranscodeService) initializeStorageReservations() error {
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.executionRepo == nil {
		return fmt.Errorf("transcode storage reservation dependencies are unavailable")
	}
	if err := model.AutoMigrateTranscodeStorageReservation(s.repo.DB()); err != nil {
		return err
	}
	now := time.Now()
	published, err := s.executionRepo.ReconcilePublishedStorageReservations(now)
	if err != nil {
		return err
	}
	released, err := s.executionRepo.ReconcileReleasedStorageReservations(now)
	if err != nil {
		return err
	}
	if published > 0 && s.logger != nil {
		s.logger.Infof("启动时已补偿 %d 条已发布转码 Reservation 校准证据", published)
	}
	if released > 0 && s.logger != nil {
		s.logger.Infof("启动时已释放 %d 条终态转码空间 Reservation", released)
	}
	return nil
}

// ensureJobStorageReservation is called immediately before the database Claim.
// Queued work therefore remains durable without monopolizing capacity, while a
// process can never start FFmpeg without owning a persisted peak-space budget.
func (s *TranscodeService) ensureJobStorageReservation(jobID string) error {
	if s == nil || s.executionRepo == nil {
		return fmt.Errorf("%w: service unavailable", ErrTranscodeStorageReservationUnavailable)
	}
	if existing, err := s.executionRepo.HasActiveJobStorageReservation(jobID); err != nil {
		return err
	} else if existing {
		return nil
	}

	job, err := s.executionRepo.FindJobByID(jobID)
	if err != nil {
		return err
	}
	media, err := s.loadReservationMedia(job.MediaID)
	if err != nil {
		return err
	}
	estimate, err := estimateTranscodeJobStorage(job, media)
	if err != nil {
		return err
	}
	budget, err := s.storageReservationBudget(time.Now())
	if err != nil {
		return err
	}
	reservation, err := s.executionRepo.AcquireJobStorageReservation(
		job.ID,
		estimate.EstimatedBytes,
		budget,
		time.Now(),
	)
	if err != nil {
		return err
	}
	if s.logger != nil {
		s.logger.Infof(
			"已获取转码空间 Reservation job=%s media=%s profile=%s bytes=%d duration=%dms fallback=%s",
			job.ID,
			job.MediaID,
			job.ProfileID,
			reservation.ReservedBytes,
			estimate.DurationMS,
			estimate.Fallback,
		)
	}
	return nil
}

func (s *TranscodeService) loadReservationMedia(mediaID string) (*model.Media, error) {
	if s == nil || s.repo == nil || s.repo.DB() == nil {
		return nil, fmt.Errorf("media repository is unavailable")
	}
	var media model.Media
	if err := s.repo.DB().First(&media, "id = ?", mediaID).Error; err != nil {
		return nil, fmt.Errorf("load reservation media %s: %w", mediaID, err)
	}
	return &media, nil
}

func estimateTranscodeJobStorage(
	job *model.TranscodeJobRecord,
	media *model.Media,
) (transcodestorageestimate.Result, error) {
	if job == nil || media == nil {
		return transcodestorageestimate.Result{}, fmt.Errorf("storage estimate inputs are incomplete")
	}
	profile, ok := transcodeprofile.Runtime(job.ProfileID)
	if !ok {
		return transcodestorageestimate.Result{}, fmt.Errorf("unknown transcode profile %q", job.ProfileID)
	}
	durationMS := job.DurationMS
	if durationMS <= 0 {
		totalMS := int64(math.Round(media.Duration * 1000))
		if totalMS <= 0 && media.Runtime > 0 {
			totalMS = int64(media.Runtime) * 60 * 1000
		}
		if totalMS > job.StartMS {
			durationMS = totalMS - job.StartMS
		}
	}
	return transcodestorageestimate.Estimate(transcodestorageestimate.Input{
		VideoBitrate: profile.VideoBitrate,
		AudioBitrate: profile.AudioBitrate,
		DurationMS:   durationMS,
		SourceBytes:  media.FileSize,
	})
}

func (s *TranscodeService) storageReservationBudget(now time.Time) (repository.TranscodeStorageReservationBudget, error) {
	status := s.runDiskPressureGovernorTick(now, false)
	if status.AdmissionBlocked {
		return repository.TranscodeStorageReservationBudget{}, fmt.Errorf(
			"%w: disk pressure level=%s reasons=%v",
			ErrTranscodeStorageReservationUnavailable,
			status.Level,
			status.Reasons,
		)
	}
	policy := s.diskPressurePolicy().Normalized()
	available := ^uint64(0)
	bounded := false

	if status.TotalBytes > 0 {
		highUsedLimit := uint64(float64(status.TotalBytes) * policy.HighWatermarkPct / 100)
		headroom := uint64(0)
		if status.UsedBytes < highUsedLimit {
			headroom = highUsedLimit - status.UsedBytes
		}
		available = minReservationHeadroom(available, headroom)
		bounded = true
	}
	freeHeadroom := uint64(0)
	if status.FreeBytes > policy.MinFreeBytes {
		freeHeadroom = status.FreeBytes - policy.MinFreeBytes
	}
	available = minReservationHeadroom(available, freeHeadroom)
	bounded = true

	if policy.MaxStoreBytes > 0 {
		storeHeadroom := uint64(0)
		if status.StoreBytes < policy.MaxStoreBytes {
			storeHeadroom = policy.MaxStoreBytes - status.StoreBytes
		}
		available = minReservationHeadroom(available, storeHeadroom)
		bounded = true
	}
	if !bounded {
		available = 0
	}
	maxInt64 := ^uint64(0) >> 1
	if available > maxInt64 {
		available = maxInt64
	}
	return repository.TranscodeStorageReservationBudget{
		AvailableBytes: int64(available),
		SampledAt:      status.SampledAt,
	}, nil
}

func minReservationHeadroom(current, candidate uint64) uint64 {
	if candidate < current {
		return candidate
	}
	return current
}

func (s *TranscodeService) GetStorageReservationStatus() TranscodeStorageReservationStatus {
	status := TranscodeStorageReservationStatus{}
	if s == nil || s.executionRepo == nil {
		return status
	}
	if summary, err := s.executionRepo.StorageReservationSummary(); err == nil {
		status.ActiveCount = summary.ActiveCount
		status.ActiveBytes = summary.ActiveBytes
		status.ReservedBytes = summary.ReservedBytes
		status.ObservedBytes = summary.ObservedBytes
		status.RemainingBytes = summary.RemainingBytes
		status.WaitingCount = summary.WaitingCount
		status.CalibrationSamples = summary.CalibrationSamples
		status.AverageActualToEstimate = summary.AverageActualToEstimate
		status.AverageAbsoluteError = summary.AverageAbsoluteError
		status.UnderpredictedCount = summary.UnderpredictedCount
	} else if s.logger != nil {
		s.logger.Debugf("读取转码空间 Reservation 统计失败: %v", err)
	}
	if budget, err := s.storageReservationBudget(time.Now()); err == nil {
		status.AvailableHeadroom = budget.AvailableBytes - status.RemainingBytes
		if status.AvailableHeadroom < 0 {
			status.AvailableHeadroom = 0
		}
	}
	return status
}

func transcodeReserveQueueCandidate(queue *transcodePriorityQueue, jobID string) error {
	owner := transcodePressureOwner(queue)
	if owner == nil {
		// Direct queue fixtures predate the service owner registry. Production
		// queues are always registered during recoverPendingTasks before workers
		// start, so this compatibility path is limited to isolated unit tests.
		return nil
	}
	return owner.ensureJobStorageReservation(jobID)
}

func isTranscodeReservationCapacityError(err error) bool {
	return errors.Is(err, repository.ErrTranscodeStorageReservationCapacity)
}
