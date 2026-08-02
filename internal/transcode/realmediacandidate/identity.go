package realmediacandidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

func Identity(contract Contract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
	if err := contract.ValidateFor(spec, manifest); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(contract)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal real-media candidate evidence: %w", err)
	}
	digest := sha256.Sum256(content)
	return contract.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func BuildCaseEvidence(index int, caseSpec transcodecorpus.CaseSpec, asset transcodecorpus.AssetEvidence, evidence transcodereorder.CaseEvidence) (CaseEvidence, error) {
	if err := evidence.Validate(); err != nil {
		return CaseEvidence{}, err
	}
	base := BaseEvidence(evidence)
	if err := base.Validate(); err != nil {
		return CaseEvidence{}, err
	}
	assetHash, err := canonicalHash(asset)
	if err != nil {
		return CaseEvidence{}, err
	}
	baseHash, err := canonicalHash(base)
	if err != nil {
		return CaseEvidence{}, err
	}
	reorderHash, err := canonicalHash(evidence)
	if err != nil {
		return CaseEvidence{}, err
	}
	if len(evidence.Candidates) == 0 || len(evidence.Candidates[0].Runs) == 0 {
		return CaseEvidence{}, fmt.Errorf("real-media candidate case %s has no runs", caseSpec.ID)
	}
	boundary := evidence.Candidates[0].Runs[0].Base.Boundary
	result := CaseEvidence{
		Source: SourceIdentity{
			AssetIndex:        index,
			CaseID:            asset.CaseID,
			RelativePath:      asset.RelativePath,
			SHA256:            asset.SHA256,
			SizeBytes:         asset.SizeBytes,
			AssetEvidenceHash: assetHash,
		},
		RequiredEvidence: append([]string(nil), caseSpec.RequiredEvidence...),
		TimestampPlan: EvidenceIdentity{
			Version: boundary.TimestampPlanVersion,
			Hash:    boundary.TimestampPlanHash,
		},
		TimeBaseCandidate: EvidenceIdentity{
			Version: transcodetimebase.SchemaVersion,
			Hash:    baseHash,
		},
		ReorderCandidate: EvidenceIdentity{
			Version: transcodereorder.SchemaVersion,
			Hash:    reorderHash,
		},
		Evidence: evidence,
	}
	if err := result.ValidateFor(index, caseSpec, asset); err != nil {
		return CaseEvidence{}, err
	}
	return result, nil
}

func canonicalHash(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal evidence identity: %w", err)
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}
