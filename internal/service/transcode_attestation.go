package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
)

const (
	artifactAttestationProvisional = "provisional"
	artifactAttestationVerified    = "verified"
)

var liveArtifactAttestationMu sync.Mutex

func requiresProducedMediaAttestation(artifact *model.TranscodeArtifactRecord) bool {
	return artifact != nil && artifact.EncodingPlanVersion != "" && artifact.EncodingPlanHash != "" && artifact.EncodingPlanJSON != ""
}

func (s *TranscodeService) attestOwnedArtifactForPublish(job *TranscodeJob) error {
	if s == nil || s.executionRepo == nil || job == nil || job.ExecutionJob == nil || job.CurrentAttempt == nil || job.CurrentArtifact == nil {
		return fmt.Errorf("transcode attestation inputs are incomplete")
	}
	artifact := job.CurrentArtifact
	if !requiresProducedMediaAttestation(artifact) {
		return nil
	}
	manifestPath := filepath.Join(artifact.TempPath, "stream.m3u8")
	ctx := job.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	value, version, hash, canonical, err := s.verifyArtifactAttestation(ctx, artifact, manifestPath, transcodeattestation.ScopeComplete)
	if err != nil {
		return err
	}
	startMS, endMS := attestationTimelineBounds(value)
	attestedAt := time.Now()
	owned, err := s.executionRepo.RecordOwnedArtifactAttestation(
		job.ExecutionJob.ID,
		job.CurrentAttempt.ID,
		artifact.ID,
		job.leaseToken,
		version,
		artifactAttestationVerified,
		hash,
		canonical,
		startMS,
		endMS,
		attestedAt,
	)
	if err != nil {
		return fmt.Errorf("persist artifact attestation: %w", err)
	}
	if !owned {
		return fmt.Errorf("artifact attestation ownership lost")
	}
	applyArtifactAttestation(artifact, version, artifactAttestationVerified, hash, canonical, startMS, endMS, attestedAt)
	return nil
}

func (s *TranscodeService) ensureProvisionalArtifactAttestation(artifact *model.TranscodeArtifactRecord) error {
	if s == nil || s.executionRepo == nil || artifact == nil {
		return fmt.Errorf("transcode attestation inputs are incomplete")
	}
	if artifact.AttestationStatus == artifactAttestationProvisional || artifact.AttestationStatus == artifactAttestationVerified {
		_, err := decodeArtifactAttestation(artifact)
		return err
	}
	if !requiresProducedMediaAttestation(artifact) || artifact.Status != "staging" || artifact.TempPath == "" {
		return fmt.Errorf("artifact is not ready for provisional attestation")
	}

	liveArtifactAttestationMu.Lock()
	defer liveArtifactAttestationMu.Unlock()
	// Another playlist request may have completed the gate while this caller was
	// waiting. Re-read the row before starting a second ffprobe process.
	current, err := s.executionRepo.FindActiveArtifactByEncodingPlanForAttestation(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		artifact.Kind,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		time.Now(),
	)
	if err != nil {
		return err
	}
	if current.AttestationStatus == artifactAttestationProvisional || current.AttestationStatus == artifactAttestationVerified {
		*artifact = *current
		_, err := decodeArtifactAttestation(artifact)
		return err
	}

	manifestPath := filepath.Join(current.TempPath, "stream.m3u8")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	value, version, hash, canonical, err := s.verifyArtifactAttestation(ctx, current, manifestPath, transcodeattestation.ScopeFirstSegment)
	if err != nil {
		return err
	}
	startMS, endMS := attestationTimelineBounds(value)
	attestedAt := time.Now()
	stored, err := s.executionRepo.RecordCurrentArtifactAttestation(
		current.ID,
		version,
		artifactAttestationProvisional,
		hash,
		canonical,
		startMS,
		endMS,
		attestedAt,
	)
	if err != nil {
		return fmt.Errorf("persist provisional artifact attestation: %w", err)
	}
	if !stored {
		return fmt.Errorf("artifact lost readiness ownership before attestation")
	}
	applyArtifactAttestation(current, version, artifactAttestationProvisional, hash, canonical, startMS, endMS, attestedAt)
	*artifact = *current
	return nil
}

func (s *TranscodeService) verifyArtifactAttestation(
	ctx context.Context,
	artifact *model.TranscodeArtifactRecord,
	manifestPath,
	scope string,
) (transcodeattestation.Attestation, string, string, string, error) {
	verifier := transcodeattestation.Verifier{FFprobePath: s.cfg.App.FFprobePath}
	value, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        manifestPath,
		EncodingPlanVersion: artifact.EncodingPlanVersion,
		EncodingPlanHash:    artifact.EncodingPlanHash,
		EncodingPlanJSON:    artifact.EncodingPlanJSON,
		Scope:               scope,
	})
	if err != nil {
		return transcodeattestation.Attestation{}, "", "", "", fmt.Errorf("verify produced media: %w", err)
	}
	version, hash, canonical, err := transcodeattestation.Identity(value)
	if err != nil {
		return transcodeattestation.Attestation{}, "", "", "", err
	}
	return value, version, hash, canonical, nil
}

func decodeArtifactAttestation(artifact *model.TranscodeArtifactRecord) (transcodeattestation.Attestation, error) {
	if artifact == nil || artifact.AttestationVersion == "" || artifact.AttestationHash == "" || artifact.AttestationJSON == "" {
		return transcodeattestation.Attestation{}, fmt.Errorf("artifact attestation is missing")
	}
	var value transcodeattestation.Attestation
	if err := json.Unmarshal([]byte(artifact.AttestationJSON), &value); err != nil {
		return transcodeattestation.Attestation{}, fmt.Errorf("decode artifact attestation: %w", err)
	}
	version, hash, canonical, err := transcodeattestation.Identity(value)
	if err != nil {
		return transcodeattestation.Attestation{}, err
	}
	if version != artifact.AttestationVersion || hash != artifact.AttestationHash || canonical != artifact.AttestationJSON {
		return transcodeattestation.Attestation{}, fmt.Errorf("artifact attestation identity is invalid")
	}
	if artifact.AttestationStatus == artifactAttestationVerified && value.Scope != transcodeattestation.ScopeComplete {
		return transcodeattestation.Attestation{}, fmt.Errorf("verified artifact has incomplete attestation")
	}
	if artifact.AttestationStatus == artifactAttestationProvisional && value.Scope != transcodeattestation.ScopeFirstSegment {
		return transcodeattestation.Attestation{}, fmt.Errorf("provisional artifact has invalid attestation scope")
	}
	if err := transcodeattestation.VerifyAgainstEncodingPlan(
		value,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.EncodingPlanJSON,
	); err != nil {
		return transcodeattestation.Attestation{}, err
	}
	return value, nil
}

func attestationTimelineBounds(value transcodeattestation.Attestation) (int64, int64) {
	return value.First.Timeline.Video.StartMS, value.Last.Timeline.Video.EndMS
}

func applyArtifactAttestation(
	artifact *model.TranscodeArtifactRecord,
	version,
	status,
	hash,
	canonical string,
	startMS,
	endMS int64,
	attestedAt time.Time,
) {
	artifact.AttestationVersion = version
	artifact.AttestationStatus = status
	artifact.AttestationHash = hash
	artifact.AttestationJSON = canonical
	artifact.TimelineStartMS = startMS
	artifact.TimelineEndMS = endMS
	artifact.AttestedAt = &attestedAt
}
