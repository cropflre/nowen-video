package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	transcodediskpressure "github.com/nowen-video/nowen-video/internal/transcode/diskpressure"
	"github.com/shirou/gopsutil/v3/disk"
)

const (
	transcodePressureEvaluationInterval  = 30 * time.Second
	transcodePressureAccessGrace         = 15 * time.Minute
	transcodePressurePublishedGrace      = 24 * time.Hour
	transcodePressureAccessWriteInterval = 30 * time.Second
	transcodePressureMaxReclaimBatches   = 4
)

var ErrTranscodeStoragePressure = errors.New("transcode artifact store is under disk pressure")

type TranscodeDiskPressureStatus struct {
	transcodediskpressure.Snapshot
	LastError          string    `json:"last_error,omitempty"`
	LastReclaimAt      time.Time `json:"last_reclaim_at,omitempty"`
	LastReclaimedBytes int64     `json:"last_reclaimed_bytes"`
	LastReclaimedRows  int       `json:"last_reclaimed_rows"`
}

type artifactAccessTouch struct {
	artifactID string
	at         time.Time
}

type transcodeDiskPressureState struct {
	mu             sync.Mutex
	status         TranscodeDiskPressureStatus
	lastEvaluation time.Time
	touches        chan artifactAccessTouch
}

var transcodeDiskPressureStates sync.Map
var transcodeDiskPressureOwners sync.Map

func (s *TranscodeService) diskPressureState() *transcodeDiskPressureState {
	if s == nil {
		return &transcodeDiskPressureState{}
	}
	if existing, ok := transcodeDiskPressureStates.Load(s); ok {
		return existing.(*transcodeDiskPressureState)
	}
	state := &transcodeDiskPressureState{touches: make(chan artifactAccessTouch, 256)}
	actual, loaded := transcodeDiskPressureStates.LoadOrStore(s, state)
	if loaded {
		return actual.(*transcodeDiskPressureState)
	}
	go s.artifactAccessTouchLoop(state)
	return state
}

func (s *TranscodeService) initializeDiskPressureGovernor() {
	if s == nil || s.jobs == nil {
		return
	}
	transcodeDiskPressureOwners.Store(s.jobs, s)
	if err := s.initializeStorageReservations(); err != nil {
		panic(fmt.Sprintf("initialize transcode storage reservations: %v", err))
	}
	s.runDiskPressureGovernorTick(time.Now(), true)
}

func transcodePressureOwner(queue *transcodePriorityQueue) *TranscodeService {
	if queue == nil {
		return nil
	}
	owner, ok := transcodeDiskPressureOwners.Load(queue)
	if !ok {
		return nil
	}
	service, _ := owner.(*TranscodeService)
	return service
}

func transcodeQueueAdmissionError(queue *transcodePriorityQueue) error {
	owner := transcodePressureOwner(queue)
	if owner == nil {
		return nil
	}
	return owner.checkDiskPressureAdmission()
}

func transcodeQueueClaimAllowed(queue *transcodePriorityQueue) bool {
	return transcodeQueueAdmissionError(queue) == nil
}

func (s *TranscodeService) checkDiskPressureAdmission() error {
	status := s.runDiskPressureGovernorTick(time.Now(), false)
	if !status.AdmissionBlocked {
		return nil
	}
	return fmt.Errorf(
		"%w: level=%s free=%d store=%d reasons=%v",
		ErrTranscodeStoragePressure,
		status.Level,
		status.FreeBytes,
		status.StoreBytes,
		status.Reasons,
	)
}

func (s *TranscodeService) GetDiskPressureStatus() TranscodeDiskPressureStatus {
	return s.runDiskPressureGovernorTick(time.Now(), false)
}

func (s *TranscodeService) runDiskPressureGovernorTick(now time.Time, force bool) TranscodeDiskPressureStatus {
	state := s.diskPressureState()
	state.mu.Lock()
	defer state.mu.Unlock()

	if !force && !state.lastEvaluation.IsZero() && now.Sub(state.lastEvaluation) < transcodePressureEvaluationInterval {
		return state.status
	}
	state.lastEvaluation = now

	sample, err := s.sampleDiskPressure(now)
	if err != nil {
		state.status = TranscodeDiskPressureStatus{
			Snapshot: transcodediskpressure.Snapshot{
				Level:            transcodediskpressure.LevelCritical,
				Reasons:          []string{"disk_sample_unavailable"},
				AdmissionBlocked: true,
				QueuePaused:      true,
				SampledAt:        now,
			},
			LastError: err.Error(),
		}
		if s.logger != nil {
			s.logger.Warnf("读取 Artifact Store 磁盘水位失败，转码准入已关闭: %v", err)
		}
		return state.status
	}

	previous := state.status.Level
	policy := s.diskPressurePolicy()
	snapshot := transcodediskpressure.Evaluate(previous, sample, policy)
	status := TranscodeDiskPressureStatus{Snapshot: snapshot}
	if snapshot.Level != transcodediskpressure.LevelNormal {
		rows, bytes, reclaimErr := s.reclaimDiskPressure(snapshot, now)
		status.LastReclaimAt = now
		status.LastReclaimedRows = rows
		status.LastReclaimedBytes = bytes
		if reclaimErr != nil {
			status.LastError = reclaimErr.Error()
			if s.logger != nil {
				s.logger.Warnf("Artifact Store 压力回收未完全成功: %v", reclaimErr)
			}
		}
		if refreshed, refreshErr := s.sampleDiskPressure(time.Now()); refreshErr == nil {
			status.Snapshot = transcodediskpressure.Evaluate(snapshot.Level, refreshed, policy)
		} else if status.LastError == "" {
			status.LastError = refreshErr.Error()
		}
	}
	state.status = status
	return state.status
}

func (s *TranscodeService) diskPressurePolicy() transcodediskpressure.Config {
	cfg := transcodediskpressure.DefaultConfig()
	if s != nil && s.cfg != nil && s.cfg.Cache.MaxDiskUsageMB > 0 {
		cfg.MaxStoreBytes = uint64(s.cfg.Cache.MaxDiskUsageMB) * 1024 * 1024
	}
	return cfg
}

func (s *TranscodeService) sampleDiskPressure(now time.Time) (transcodediskpressure.Sample, error) {
	if s == nil || s.artifactStore == nil || s.artifactStore.Root() == "" {
		return transcodediskpressure.Sample{}, fmt.Errorf("artifact store is unavailable")
	}
	usage, err := disk.Usage(s.artifactStore.Root())
	if err != nil {
		return transcodediskpressure.Sample{}, err
	}
	storeBytes := s.GetCacheDiskUsage()
	if storeBytes < 0 {
		storeBytes = 0
	}
	return transcodediskpressure.Sample{
		TotalBytes: usage.Total,
		FreeBytes:  usage.Free,
		UsedBytes:  usage.Used,
		StoreBytes: uint64(storeBytes),
		SampledAt:  now,
	}, nil
}

func (s *TranscodeService) reclaimDiskPressure(
	snapshot transcodediskpressure.Snapshot,
	now time.Time,
) (int, int64, error) {
	if s == nil || s.executionRepo == nil {
		return 0, 0, fmt.Errorf("artifact cleanup repository is unavailable")
	}
	protectedAfter := now.Add(-transcodePressureAccessGrace)
	totalRows := 0
	var totalBytes int64
	var firstErr error

	terminal, err := s.executionRepo.QueueTerminalArtifactsForPressure(protectedAfter, now, 500)
	if err != nil {
		firstErr = err
	} else {
		totalRows += terminal.Queued
		totalBytes += terminal.Bytes
	}
	if terminal.Queued > 0 {
		_, _, cleanupErr := s.cleanupTerminalArtifactBatch(protectedAfter, now)
		if cleanupErr != nil && firstErr == nil {
			firstErr = cleanupErr
		}
		s.InvalidateCacheDiskUsage()
	}

	current, sampleErr := s.sampleDiskPressure(time.Now())
	if sampleErr != nil {
		if firstErr == nil {
			firstErr = sampleErr
		}
		return totalRows, totalBytes, firstErr
	}
	currentSnapshot := transcodediskpressure.Evaluate(snapshot.Level, current, s.diskPressurePolicy())
	publishedBefore := now.Add(-transcodePressurePublishedGrace)

	for batch := 0; batch < transcodePressureMaxReclaimBatches && currentSnapshot.Level != transcodediskpressure.LevelNormal; batch++ {
		target := int64(currentSnapshot.ReclaimTargetBytes)
		if target <= 0 {
			target = 256 * 1024 * 1024
		}
		queued, queueErr := s.executionRepo.ExpirePublishedArtifactsForPressure(
			protectedAfter,
			publishedBefore,
			target,
			time.Now(),
			100,
		)
		if queueErr != nil {
			if firstErr == nil {
				firstErr = queueErr
			}
			break
		}
		if queued.Queued == 0 {
			break
		}
		totalRows += queued.Queued
		totalBytes += queued.Bytes
		_, _, cleanupErr := s.cleanupTerminalArtifactBatch(protectedAfter, time.Now())
		if cleanupErr != nil && firstErr == nil {
			firstErr = cleanupErr
		}
		s.InvalidateCacheDiskUsage()
		refreshed, refreshErr := s.sampleDiskPressure(time.Now())
		if refreshErr != nil {
			if firstErr == nil {
				firstErr = refreshErr
			}
			break
		}
		currentSnapshot = transcodediskpressure.Evaluate(currentSnapshot.Level, refreshed, s.diskPressurePolicy())
	}

	if totalRows > 0 && s.logger != nil {
		s.logger.Warnf(
			"Artifact Store 压力回收完成 rows=%d bytes=%d level=%s free=%d",
			totalRows,
			totalBytes,
			currentSnapshot.Level,
			currentSnapshot.FreeBytes,
		)
	}
	return totalRows, totalBytes, firstErr
}

func (s *TranscodeService) TouchArtifactAccess(artifactID string) {
	if s == nil || s.executionRepo == nil || artifactID == "" {
		return
	}
	state := s.diskPressureState()
	select {
	case state.touches <- artifactAccessTouch{artifactID: artifactID, at: time.Now()}:
	default:
		if s.logger != nil {
			s.logger.Debugf("Artifact 访问触摸队列已满 artifact=%s", artifactID)
		}
	}
}

func (s *TranscodeService) artifactAccessTouchLoop(state *transcodeDiskPressureState) {
	lastWrite := make(map[string]time.Time)
	for touch := range state.touches {
		if previous := lastWrite[touch.artifactID]; !previous.IsZero() && touch.at.Sub(previous) < transcodePressureAccessWriteInterval {
			continue
		}
		writeBefore := touch.at.Add(-transcodePressureAccessWriteInterval)
		updated, err := s.executionRepo.TouchArtifactAccess(touch.artifactID, touch.at, writeBefore)
		if err != nil {
			if s.logger != nil {
				s.logger.Debugf("持久化 Artifact 最近访问失败 artifact=%s: %v", touch.artifactID, err)
			}
			continue
		}
		if updated {
			lastWrite[touch.artifactID] = touch.at
		}
	}
}
