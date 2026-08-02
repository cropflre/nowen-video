package certification

import (
	"encoding/json"
	"fmt"

	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

const RealMediaCandidateMatrixSchemaVersion = "ffmpeg-real-media-corpus-candidate-matrix-v1"

type RealMediaCandidateMatrixReport struct {
	SchemaVersion   string                          `json:"schema_version"`
	Spec            transcodecorpus.Spec            `json:"spec"`
	Manifest        transcodecorpus.Manifest        `json:"manifest"`
	ContractVersion string                          `json:"contract_version"`
	ContractHash    string                          `json:"contract_hash"`
	Evidence        transcodecandidate.Contract     `json:"evidence"`
}

func (r RealMediaCandidateMatrixReport) Validate() error {
	if r.SchemaVersion != RealMediaCandidateMatrixSchemaVersion {
		return fmt.Errorf("unsupported real-media candidate matrix schema %q", r.SchemaVersion)
	}
	if err := r.Manifest.ValidateFor(r.Spec); err != nil {
		return err
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcodecandidate.Identity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("real-media candidate contract identity is invalid")
	}
	return nil
}

func MarshalRealMediaCandidateMatrixReport(report RealMediaCandidateMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal real-media candidate matrix: %w", err)
	}
	return append(content, '\n'), nil
}
