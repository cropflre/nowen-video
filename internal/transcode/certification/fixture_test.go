package certification

import (
	"path/filepath"
	"strings"
	"testing"

	transcodetimeline "github.com/nowen-video/nowen-video/internal/transcode/timeline"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestFixtureHLSArgsUseProductionBuilderAndTimestampPlan(t *testing.T) {
	args, err := fixtureHLSArgs(
		"/media/source.mp4",
		"/cache/continuation",
		transcodetimestamp.Default(),
		30,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{
		"-y -copyts -start_at_zero -ss 30.00",
		"-force_key_frames expr:gte(t,n_forced*2)",
		"-hls_flags independent_segments+append_list+program_date_time",
		"-hls_playlist_type event",
		"-avoid_negative_ts disabled -fps_mode passthrough",
		"-start_number 15",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("fixture command missing %q: %s", expected, joined)
		}
	}
	if strings.Index(joined, "-copyts") > strings.Index(joined, "-i /media/source.mp4") {
		t.Fatalf("timestamp policy must precede the input: %s", joined)
	}
	expectedOutput := filepath.Join("/cache/continuation", "stream.m3u8")
	if args[len(args)-1] != expectedOutput {
		t.Fatalf("unexpected output path %q", args[len(args)-1])
	}
}

func TestFixtureStartupUsesBoundedVODProjection(t *testing.T) {
	args, err := fixtureHLSArgs(
		"/media/source.mp4",
		"/cache/startup",
		transcodetimestamp.Default(),
		0,
		30,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{
		"-hls_flags independent_segments+program_date_time",
		"-t 30 -hls_playlist_type vod",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("startup fixture command missing %q: %s", expected, joined)
		}
	}
	if strings.Contains(joined, "append_list") || strings.Contains(joined, "-hls_playlist_type event") {
		t.Fatalf("startup fixture retained continuation-only HLS options: %s", joined)
	}
}

func TestReportValidationKeepsFixtureFailClosed(t *testing.T) {
	report := validReport()
	if err := report.Validate(); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
	report.Handoff.SeamlessAllowed = true
	report.Handoff.DiscontinuityRequired = false
	if err := report.Validate(); err == nil {
		t.Fatal("uncertified fixture authorized seamless playback")
	}
}

func TestMarshalReportIsStableAndTerminated(t *testing.T) {
	content, err := MarshalReport(validReport())
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 || content[len(content)-1] != '\n' {
		t.Fatal("fixture report is not newline terminated")
	}
	if !strings.Contains(string(content), `"schema_version": "ffmpeg-handoff-fixture-report-v1"`) {
		t.Fatalf("fixture schema missing from report: %s", content)
	}
}

func validReport() Report {
	artifact := ArtifactReport{
		AttestationVersion: "hls-produced-media-attestation-v1",
		AttestationHash:    "attestation",
		SegmentCount:       1,
		VideoStartMS:       1400,
		AudioStartMS:       1379,
		VideoEndMS:         3400,
		AudioEndMS:         3379,
	}
	return Report{
		SchemaVersion:        ReportSchemaVersion,
		FixtureID:            FixtureCFR48K,
		Backend:              transcodetimestamp.BackendSoftware,
		FFmpegVersion:        "ffmpeg version fixture",
		FFprobeVersion:       "ffprobe version fixture",
		EncodingPlanVersion:  "hls-encoding-plan-v1",
		EncodingPlanHash:     "encoding-plan",
		TimestampPlanVersion: transcodetimestamp.SchemaVersion,
		TimestampPlanHash:    "timestamp-plan",
		Startup:              artifact,
		Continuation:         artifact,
		Handoff: HandoffReport{
			ContractVersion:       transcodetimeline.SchemaVersion,
			ContractHash:          "handoff",
			Status:                transcodetimeline.StatusAligned,
			DecisionReason:        transcodetimeline.DecisionClientCertificationPending,
			DiscontinuityRequired: true,
		},
	}
}
