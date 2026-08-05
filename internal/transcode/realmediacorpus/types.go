package realmediacorpus

const (
	SpecSchemaVersion     = "real-media-corpus-spec-v1"
	ManifestSchemaVersion = "real-media-corpus-manifest-v1"

	TierDeterministicContainer = "deterministic_container"
	TierCuratedExternal        = "curated_external"

	ContainerMP4      = "mp4"
	ContainerMatroska = "matroska"
	ContainerMPEGTS   = "mpegts"

	FrameRateCFR = "cfr"
	FrameRateVFR = "vfr"

	CodecH264 = "h264"
	CodecAAC  = "aac"
	CodecOpus = "opus"

	EvidenceTimestampPlan = "timestamp_plan"
	EvidenceCadence       = "output_cadence"
	EvidencePacketOrder   = "packet_order"
	EvidenceAttestation   = "produced_media_attestation"
	EvidenceBoundary      = "boundary_packet"
	EvidenceAVSync        = "av_boundary_sync"
)

// Spec is the immutable intent for a file-backed certification corpus. It is
// intentionally separate from the historical synthetic FixtureSpec so new
// containers and timeline features cannot silently change old fixture meaning.
type Spec struct {
	SchemaVersion         string     `json:"schema_version"`
	Cases                 []CaseSpec `json:"cases"`
	SeamlessAllowed       bool       `json:"seamless_allowed"`
	DiscontinuityRequired bool       `json:"discontinuity_required"`
}

type CaseSpec struct {
	ID               string     `json:"id"`
	Description      string     `json:"description"`
	Purpose          string     `json:"purpose"`
	Tier             string     `json:"tier"`
	Source           SourcePlan `json:"source"`
	BoundaryMicros   int64      `json:"boundary_micros"`
	RequiredEvidence []string   `json:"required_evidence"`
}

type SourcePlan struct {
	Container string       `json:"container"`
	Video     VideoPlan    `json:"video"`
	Audio     AudioPlan    `json:"audio"`
	Timeline  TimelinePlan `json:"timeline"`
}

type VideoPlan struct {
	Codec           string     `json:"codec"`
	Profile         string     `json:"profile"`
	PixelFormat     string     `json:"pixel_format"`
	Width           int        `json:"width"`
	Height          int        `json:"height"`
	FrameRateMode   string     `json:"frame_rate_mode"`
	FrameRates      []Rational `json:"frame_rates"`
	GOPSize         int        `json:"gop_size"`
	BFrames         int        `json:"b_frames"`
	ReferenceFrames int        `json:"reference_frames"`
	OpenGOP         bool       `json:"open_gop"`
	Interlaced      bool       `json:"interlaced"`
	HDR             bool       `json:"hdr"`
	ColorPrimaries  string     `json:"color_primaries"`
	ColorTransfer   string     `json:"color_transfer"`
	ColorMatrix     string     `json:"color_matrix"`
}

type AudioPlan struct {
	Codec      string `json:"codec"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	Layout     string `json:"layout"`
	TrackCount int    `json:"track_count"`
}

type TimelinePlan struct {
	DurationMicros int64 `json:"duration_micros"`
	OriginMicros   int64 `json:"origin_micros"`
	HasEditList    bool  `json:"has_edit_list"`
	Discontinuous  bool  `json:"discontinuous"`
}

type Rational struct {
	Numerator   int64 `json:"numerator"`
	Denominator int64 `json:"denominator"`
}

// Manifest binds one resolved corpus execution to immutable bytes and observed
// probe evidence. Relative paths are diagnostic only; CaseID and SHA-256 are
// the authoritative source identities.
type Manifest struct {
	SchemaVersion         string          `json:"schema_version"`
	SpecVersion           string          `json:"spec_version"`
	SpecHash              string          `json:"spec_hash"`
	GeneratorVersion      string          `json:"generator_version"`
	GenerationRepeatCount int             `json:"generation_repeat_count"`
	FFmpegVersion         string          `json:"ffmpeg_version"`
	FFprobeVersion        string          `json:"ffprobe_version"`
	Assets                []AssetEvidence `json:"assets"`
	SeamlessAllowed       bool            `json:"seamless_allowed"`
	DiscontinuityRequired bool            `json:"discontinuity_required"`
}

type AssetEvidence struct {
	CaseID        string        `json:"case_id"`
	RelativePath  string        `json:"relative_path"`
	CommandSHA256 string        `json:"command_sha256"`
	SHA256        string        `json:"sha256"`
	RepeatSHA256  []string      `json:"repeat_sha256"`
	SizeBytes     int64         `json:"size_bytes"`
	Probe         ProbeEvidence `json:"probe"`
}

type ProbeEvidence struct {
	Container                   string     `json:"container"`
	DurationMicros              int64      `json:"duration_micros"`
	StartMicros                 int64      `json:"start_micros"`
	VideoCodec                  string     `json:"video_codec"`
	VideoProfile                string     `json:"video_profile"`
	PixelFormat                 string     `json:"pixel_format"`
	Width                       int        `json:"width"`
	Height                      int        `json:"height"`
	ColorPrimaries              string     `json:"color_primaries"`
	ColorTransfer               string     `json:"color_transfer"`
	ColorMatrix                 string     `json:"color_matrix"`
	FrameRateMode               string     `json:"frame_rate_mode"`
	ObservedRates               []Rational `json:"observed_rates"`
	VideoTimeBase               Rational   `json:"video_time_base"`
	FrameCount                  int        `json:"frame_count"`
	KeyFrameCount               int        `json:"key_frame_count"`
	MaxKeyFrameInterval         int        `json:"max_key_frame_interval"`
	MaxPresentationReorderDepth int        `json:"max_presentation_reorder_depth"`
	MaxCompositionOffsetMicros  int64      `json:"max_composition_offset_micros"`
	AudioCodec                  string     `json:"audio_codec"`
	AudioSampleRate             int        `json:"audio_sample_rate"`
	AudioChannels               int        `json:"audio_channels"`
	AudioTrackCount             int        `json:"audio_track_count"`
	AudioTimeBase               Rational   `json:"audio_time_base"`
	HasBFrameReorder            bool       `json:"has_b_frame_reorder"`
	HasEditList                 bool       `json:"has_edit_list"`
}
