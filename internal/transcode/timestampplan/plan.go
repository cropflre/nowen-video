package timestampplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const SchemaVersion = "hls-timestamp-normalization-v1"

const (
	StrategyCopyTSStartAtZero = "copyts_start_at_zero"
	SeekModeInputAccurate     = "input_accurate"
	AvoidNegativeTSDisabled   = "disabled"
	FPSModePassthrough        = "passthrough"
	BackendSoftware           = "none"
)

// Plan is the immutable timestamp policy shared by Startup and Continuation.
// The per-Job timeline origin is deliberately excluded from this identity: the
// Startup origin is zero while Continuation begins at the Startup boundary.
type Plan struct {
	SchemaVersion          string   `json:"schema_version"`
	Strategy               string   `json:"strategy"`
	SeekMode               string   `json:"seek_mode"`
	CopyTimestamps         bool     `json:"copy_timestamps"`
	StartAtZero            bool     `json:"start_at_zero"`
	AvoidNegativeTS        string   `json:"avoid_negative_ts"`
	FPSMode                string   `json:"fps_mode"`
	CertifiedBackends      []string `json:"certified_backends"`
	OriginLowerToleranceMS int64    `json:"origin_lower_tolerance_ms"`
	OriginUpperToleranceMS int64    `json:"origin_upper_tolerance_ms"`
}

func Default() Plan {
	return Plan{
		SchemaVersion:          SchemaVersion,
		Strategy:               StrategyCopyTSStartAtZero,
		SeekMode:               SeekModeInputAccurate,
		CopyTimestamps:         true,
		StartAtZero:            true,
		AvoidNegativeTS:        AvoidNegativeTSDisabled,
		FPSMode:                FPSModePassthrough,
		CertifiedBackends:      []string{BackendSoftware},
		OriginLowerToleranceMS: 250,
		OriginUpperToleranceMS: 3000,
	}
}

func (p Plan) Validate() error {
	if p.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported timestamp plan schema %q", p.SchemaVersion)
	}
	if p.Strategy != StrategyCopyTSStartAtZero || p.SeekMode != SeekModeInputAccurate {
		return fmt.Errorf("unsupported timestamp normalization strategy")
	}
	if !p.CopyTimestamps || !p.StartAtZero {
		return fmt.Errorf("timestamp plan must preserve and zero-normalize input timestamps")
	}
	if p.AvoidNegativeTS != AvoidNegativeTSDisabled {
		return fmt.Errorf("timestamp plan must disable muxer timestamp shifting")
	}
	if p.FPSMode != FPSModePassthrough {
		return fmt.Errorf("timestamp plan must preserve packet timing")
	}
	if len(p.CertifiedBackends) != 1 || strings.TrimSpace(p.CertifiedBackends[0]) != BackendSoftware {
		return fmt.Errorf("timestamp plan v1 certifies software encoding only")
	}
	if p.OriginLowerToleranceMS < 0 || p.OriginLowerToleranceMS > 1000 {
		return fmt.Errorf("timestamp lower origin tolerance is invalid")
	}
	if p.OriginUpperToleranceMS <= 0 || p.OriginUpperToleranceMS > 5000 {
		return fmt.Errorf("timestamp upper origin tolerance is invalid")
	}
	return nil
}

func (p Plan) CanonicalJSON() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal timestamp plan: %w", err)
	}
	return string(content), nil
}

func (p Plan) Hash() (string, error) {
	canonical, err := p.CanonicalJSON()
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func Identity(p Plan) (version, hash, canonical string, err error) {
	canonical, err = p.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return p.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}

func (p Plan) SupportsBackend(backend string) bool {
	if err := p.Validate(); err != nil {
		return false
	}
	return strings.TrimSpace(backend) == BackendSoftware
}

// VerifyObservedStart proves that the produced first packets retained the
// Job-owned timeline origin. A small negative allowance covers encoder priming;
// the positive allowance covers the deterministic MPEG-TS mux delay. A reset
// Continuation (for example 1.4s instead of 31.4s) is rejected decisively.
func (p Plan) VerifyObservedStart(originMS, videoStartMS, audioStartMS int64) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if originMS < 0 {
		return fmt.Errorf("timeline origin cannot be negative")
	}
	lower := originMS - p.OriginLowerToleranceMS
	upper := originMS + p.OriginUpperToleranceMS
	for kind, value := range map[string]int64{"video": videoStartMS, "audio": audioStartMS} {
		if value < lower || value > upper {
			return fmt.Errorf("%s first packet %dms is outside normalized origin window [%d,%d]ms", kind, value, lower, upper)
		}
	}
	return nil
}
