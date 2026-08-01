package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodetimeline "github.com/nowen-video/nowen-video/internal/transcode/timeline"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

var startupHandoffAttestationGroup singleflight.Group

// StartupHandoffDecision is server-internal bridge policy. It is intentionally
// narrower than the canonical timeline contract and exposes no packet details
// to clients. Schema v1 always requires an HLS discontinuity.
type StartupHandoffDecision struct {
	SchemaVersion         string
	ContractHash          string
	Status                string
	SeamlessAllowed       bool
	DiscontinuityRequired bool
	DecisionReason        string
}

func (s *TranscodeService) ResolveReadableStartupContinuationWithHandoff(
	media *model.Media,
	startup *StartupStreamDescriptor,
) (*model.TranscodeArtifactRecord, *StartupHandoffDecision, error) {
	artifact, err := s.ResolveReadableStartupContinuation(media, startup)
	if err != nil {
		return nil, nil, err
	}
	decision, err := s.evaluateStartupHandoff(startup, artifact)
	if err != nil {
		return nil, nil, err
	}
	return artifact, decision, nil
}

func (s *TranscodeService) evaluateStartupHandoff(
	startup *StartupStreamDescriptor,
	continuation *model.TranscodeArtifactRecord,
) (*StartupHandoffDecision, error) {
	if s == nil || s.executionRepo == nil || startup == nil || continuation == nil ||
		startup.ArtifactID == "" || continuation.ID == "" || startup.MediaID == "" ||
		startup.ProfileID == "" {
		return nil, fmt.Errorf("startup handoff inputs are incomplete")
	}

	key := startup.ArtifactID + "|" + continuation.ID
	resolved, err, _ := startupHandoffAttestationGroup.Do(key, func() (any, error) {
		if existing, findErr := s.executionRepo.FindHandoffAttestation(
			startup.ArtifactID,
			continuation.ID,
			transcodetimeline.SchemaVersion,
		); findErr == nil {
			contract, decodeErr := decodeHandoffContract(existing)
			if decodeErr == nil &&
				contract.StartupAttestationVersion == startup.AttestationVersion &&
				contract.StartupAttestationHash == startup.AttestationHash &&
				contract.ContinuationAttestationVersion == continuation.AttestationVersion &&
				contract.ContinuationAttestationHash == continuation.AttestationHash {
				return handoffDecision(contract, existing.ContractHash), nil
			}
		} else if findErr != nil && findErr != gorm.ErrRecordNotFound {
			return nil, findErr
		}

		startupEvidence, decodeErr := decodeStartupDescriptorAttestation(startup)
		if decodeErr != nil {
			return nil, decodeErr
		}
		continuationEvidence, decodeErr := decodeArtifactAttestation(continuation)
		if decodeErr != nil {
			return nil, decodeErr
		}
		contract, evaluateErr := transcodetimeline.Evaluate(
			startupEvidence,
			startup.AttestationVersion,
			startup.AttestationHash,
			continuationEvidence,
			continuation.AttestationVersion,
			continuation.AttestationHash,
		)
		if evaluateErr != nil {
			return nil, fmt.Errorf("evaluate startup handoff timeline: %w", evaluateErr)
		}
		version, hash, canonical, identityErr := transcodetimeline.Identity(contract)
		if identityErr != nil {
			return nil, identityErr
		}
		now := time.Now()
		record := &model.TranscodeHandoffAttestationRecord{
			MediaID:                        startup.MediaID,
			ProfileID:                      startup.ProfileID,
			StartupArtifactID:              startup.ArtifactID,
			ContinuationArtifactID:         continuation.ID,
			SchemaVersion:                  version,
			EncodingPlanVersion:            contract.EncodingPlanVersion,
			EncodingPlanHash:               contract.EncodingPlanHash,
			StartupAttestationVersion:      contract.StartupAttestationVersion,
			StartupAttestationHash:         contract.StartupAttestationHash,
			ContinuationAttestationVersion: contract.ContinuationAttestationVersion,
			ContinuationAttestationHash:    contract.ContinuationAttestationHash,
			Status:                         contract.Status,
			ContractHash:                   hash,
			ContractJSON:                   canonical,
			VideoPresentationDeltaMicros:   contract.Video.PresentationDeltaMicros,
			VideoDecodeDeltaMicros:         contract.Video.DecodeDeltaMicros,
			AudioPresentationDeltaMicros:   contract.Audio.PresentationDeltaMicros,
			AudioDecodeDeltaMicros:         contract.Audio.DecodeDeltaMicros,
			SeamlessAllowed:                contract.SeamlessAllowed,
			DiscontinuityRequired:          contract.DiscontinuityRequired,
			DecisionReason:                 contract.DecisionReason,
			EvaluatedAt:                    now,
			CreatedAt:                      now,
			UpdatedAt:                      now,
		}
		if persistErr := s.executionRepo.UpsertHandoffAttestation(record); persistErr != nil {
			return nil, fmt.Errorf("persist startup handoff attestation: %w", persistErr)
		}
		return handoffDecision(contract, hash), nil
	})
	if err != nil {
		return nil, err
	}
	decision, ok := resolved.(*StartupHandoffDecision)
	if !ok || decision == nil {
		return nil, fmt.Errorf("startup handoff returned an invalid decision")
	}
	return decision, nil
}

func decodeHandoffContract(record *model.TranscodeHandoffAttestationRecord) (transcodetimeline.Contract, error) {
	if record == nil || record.SchemaVersion == "" || record.ContractHash == "" || record.ContractJSON == "" {
		return transcodetimeline.Contract{}, fmt.Errorf("handoff attestation is missing")
	}
	var contract transcodetimeline.Contract
	if err := json.Unmarshal([]byte(record.ContractJSON), &contract); err != nil {
		return transcodetimeline.Contract{}, fmt.Errorf("decode handoff attestation: %w", err)
	}
	version, hash, canonical, err := transcodetimeline.Identity(contract)
	if err != nil {
		return transcodetimeline.Contract{}, err
	}
	if version != record.SchemaVersion || hash != record.ContractHash || canonical != record.ContractJSON {
		return transcodetimeline.Contract{}, fmt.Errorf("handoff attestation identity is invalid")
	}
	if record.Status != contract.Status || record.SeamlessAllowed != contract.SeamlessAllowed ||
		record.DiscontinuityRequired != contract.DiscontinuityRequired || record.DecisionReason != contract.DecisionReason {
		return transcodetimeline.Contract{}, fmt.Errorf("handoff attestation projection is invalid")
	}
	return contract, nil
}

func handoffDecision(contract transcodetimeline.Contract, hash string) *StartupHandoffDecision {
	return &StartupHandoffDecision{
		SchemaVersion:         contract.SchemaVersion,
		ContractHash:          hash,
		Status:                contract.Status,
		SeamlessAllowed:       contract.SeamlessAllowed,
		DiscontinuityRequired: contract.DiscontinuityRequired,
		DecisionReason:        contract.DecisionReason,
	}
}
