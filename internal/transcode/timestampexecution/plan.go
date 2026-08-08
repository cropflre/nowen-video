package timestampexecution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const SchemaVersion = "hls-timestamp-execution-plan-v2"

const (
	StrategyContinuationPTSShift = "continuation_pts_shift"
	SeekPrecisionMicroseconds    = "microseconds"
	BackendSoftware              = "none"
)

// Plan is an immutable, certification-only candidate for shaping the
// Continuation timeline. It deliberately binds the already deployed timestamp
// normalization contract instead of silently changing its meaning.
//
// v2 cannot authorize runtime adoption or removal of the HLS discontinuity.
// A later persisted production plan must be introduced only after real-media
// and client certification proves a selected policy safe.
type Plan struct {
	SchemaVersion            string   `json:"schema_version"`
	Strategy                 string   `json:"strategy"`
	BaseTimestampPlanVersion string   `json:"base_timestamp_plan_version"`
	BaseTimestampPlanHash    string   `json:"base_timestamp_plan_hash"`
	SeekPrecision            string   `json:"seek_precision"`
	VideoPTSShiftMicros      int64    `json:"video_pts_shift_micros"`
	AudioPTSShiftMicros      int64    `json:"audio_pts_shift_micros"`
	CertifiedBackends        []string `json:"certified_backends"`
	CertificationOnly        bool     `json:"certification_only"`
	SeamlessAllowed          bool     `json:"seamless_allowed"`
	DiscontinuityRequired    bool     `json:"discontinuity_required"`
}

func New(videoShiftMicros, audioShiftMicros int64) (Plan, error) {
	baseVersion, baseHash, _, err := transcodetimestamp.Identity(transcodetimestamp.Default())
	if err != nil {
		return Plan{}, fmt.Errorf("timestamp execution base identity: %w", err)
	}
	plan := Plan{
		SchemaVersion:            SchemaVersion,
		Strategy:                 StrategyContinuationPTSShift,
		BaseTimestampPlanVersion: baseVersion,
		BaseTimestampPlanHash:    baseHash,
		SeekPrecision:            SeekPrecisionMicroseconds,
		VideoPTSShiftMicros:      videoShiftMicros,
		AudioPTSShiftMicros:      audioShiftMicros,
		CertifiedBackends:        []string{BackendSoftware},
		CertificationOnly:        true,
		DiscontinuityRequired:    true,
	}
	if err := plan.Validate(); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func Baseline() Plan {
	plan, err := New(0, 0)
	if err != nil {
		panic(err)
	}
	return plan
}

func (p Plan) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported timestamp execution schema %q", p.SchemaVersion)
	}
	if p.Strategy != StrategyContinuationPTSShift {
		return fmt.Errorf("unsupported timestamp execution strategy %q", p.Strategy)
	}
	baseVersion, baseHash, _, err := transcodetimestamp.Identity(transcodetimestamp.Default())
	if err != nil {
		return err
	}
	if p.BaseTimestampPlanVersion != baseVersion || p.BaseTimestampPlanHash != baseHash {
		return fmt.Errorf("timestamp execution plan does not bind the canonical base plan")
	}
	if p.SeekPrecision != SeekPrecisionMicroseconds {
		return fmt.Errorf("timestamp execution v2 requires microsecond seek precision")
	}
	if p.VideoPTSShiftMicros < 0 || p.AudioPTSShiftMicros < 0 {
		return fmt.Errorf("timestamp execution shifts cannot be negative")
	}
	if p.VideoPTSShiftMicros > 250_000 || p.AudioPTSShiftMicros > 250_000 {
		return fmt.Errorf("timestamp execution shifts exceed the certification safety bound")
	}
	if len(p.CertifiedBackends) != 1 || strings.TrimSpace(p.CertifiedBackends[0]) != BackendSoftware {
		return fmt.Errorf("timestamp execution v2 certifies software encoding only")
	}
	if !p.CertificationOnly {
		return fmt.Errorf("timestamp execution v2 is certification-only")
	}
	if p.SeamlessAllowed || !p.DiscontinuityRequired {
		return fmt.Errorf("timestamp execution v2 cannot authorize seamless handoff")
	}
	return nil
}

func (p Plan) CanonicalJSON() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal timestamp execution plan: %w", err)
	}
	return string(content), nil
}

func Identity(p Plan) (version, hash, canonical string, err error) {
	canonical, err = p.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return p.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}
