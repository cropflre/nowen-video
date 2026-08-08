package certification

import (
	"encoding/json"
	"fmt"

	transcodevfrisolation "github.com/nowen-video/nowen-video/internal/transcode/vfrisolation"
)

const VFRIsolationMatrixSchemaVersion = "ffmpeg-vfr-layer-isolation-matrix-v1"

const vfrIsolationCaseID = "source-vfr-24-30-origin-zero-v1"

type VFRIsolationMatrixReport struct {
	SchemaVersion   string                         `json:"schema_version"`
	ContractVersion string                         `json:"contract_version"`
	ContractHash    string                         `json:"contract_hash"`
	Evidence        transcodevfrisolation.Contract `json:"evidence"`
}

var vfrIsolationVariantSpecs = []transcodevfrisolation.VariantSpec{
	{
		ID: "production-hls-v1", Description: "Production Timestamp Plan and HLS MPEG-TS output",
		Layer: "production_baseline", Container: "hls-mpegts", FPSMode: "passthrough", EncoderTimeBase: "auto",
	},
	{
		ID: "fps-mode-vfr-hls-v1", Description: "Change only output fps_mode from passthrough to vfr",
		Layer: "output_sync_policy", Container: "hls-mpegts", FPSMode: "vfr", EncoderTimeBase: "auto",
	},
	{
		ID: "fps-mode-cfr-hls-v1", Description: "Change only output fps_mode from passthrough to cfr",
		Layer: "output_sync_policy", Container: "hls-mpegts", FPSMode: "cfr", EncoderTimeBase: "auto",
	},
	{
		ID: "encoder-time-base-avtb-hls-v1", Description: "Keep HLS path and force encoder time base to AVTB",
		Layer: "encoder_time_base", Container: "hls-mpegts", FPSMode: "passthrough", EncoderTimeBase: "1/1000000",
	},
	{
		ID: "encoder-time-base-90k-hls-v1", Description: "Keep HLS path and force encoder time base to MPEG-TS clock",
		Layer: "encoder_time_base", Container: "hls-mpegts", FPSMode: "passthrough", EncoderTimeBase: "1/90000",
	},
	{
		ID: "matroska-default-v1", Description: "Change only encoded output container from HLS MPEG-TS to Matroska",
		Layer: "encoded_container", Container: "matroska", FPSMode: "passthrough", EncoderTimeBase: "auto",
	},
	{
		ID: "matroska-avtb-v1", Description: "Matroska output with AVTB encoder time base",
		Layer: "encoder_time_base", Container: "matroska", FPSMode: "passthrough", EncoderTimeBase: "1/1000000",
	},
	{
		ID: "matroska-remux-mpegts-v1", Description: "Pure stream copy from default Matroska to MPEG-TS",
		Layer: "mpegts_muxer", Container: "mpegts", FPSMode: "not_applicable", EncoderTimeBase: "copy",
		CopyOnly: true, ParentVariantID: "matroska-default-v1",
	},
	{
		ID: "matroska-remux-hls-v1", Description: "Pure stream copy from default Matroska to HLS MPEG-TS",
		Layer: "hls_muxer", Container: "hls-mpegts", FPSMode: "not_applicable", EncoderTimeBase: "copy",
		CopyOnly: true, ParentVariantID: "matroska-default-v1",
	},
}

func AvailableVFRIsolationVariants() []transcodevfrisolation.VariantSpec {
	return append([]transcodevfrisolation.VariantSpec(nil), vfrIsolationVariantSpecs...)
}

func (r VFRIsolationMatrixReport) Validate() error {
	if r.SchemaVersion != VFRIsolationMatrixSchemaVersion {
		return fmt.Errorf("unsupported VFR isolation matrix schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.Validate(); err != nil {
		return err
	}
	version, hash, _, err := transcodevfrisolation.Identity(r.Evidence)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("VFR isolation contract identity is invalid")
	}
	if len(r.Evidence.Variants) != len(vfrIsolationVariantSpecs) {
		return fmt.Errorf("VFR isolation variant matrix is incomplete")
	}
	for index, variant := range r.Evidence.Variants {
		if variant.Spec != vfrIsolationVariantSpecs[index] {
			return fmt.Errorf("VFR isolation variant order or policy drifted at index %d", index)
		}
	}
	return nil
}

func MarshalVFRIsolationMatrixReport(report VFRIsolationMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal VFR isolation matrix: %w", err)
	}
	return append(content, '\n'), nil
}
