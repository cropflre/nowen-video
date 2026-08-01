package certification

import (
	"strings"
	"testing"

	transcodetimeline "github.com/nowen-video/nowen-video/internal/transcode/timeline"
)

func TestMarshalReportRequiresFailClosedHandoff(t *testing.T) {
	report := validReportFixture()
	content, err := MarshalReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"schema_version": "transcode-fixture-certification-v1"`) ||
		!strings.Contains(string(content), `"discontinuity_required": true`) {
		t.Fatalf("unexpected report JSON: %s", content)
	}

	report.Handoff.SeamlessAllowed = true
	report.Handoff.Contract.SeamlessAllowed = true
	report.Handoff.DiscontinuityRequired = false
	report.Handoff.Contract.DiscontinuityRequired = false
	if _, err := MarshalReport(report); err == nil {
		t.Fatal("fixture report authorized seamless playback")
	}
}

func TestReportRejectsOriginAndProjectionMismatch(t *testing.T) {
	report := validReportFixture()
	report.Continuation.TimelineOriginMS = 0
	if err := report.Validate(); err == nil {
		t.Fatal("invalid continuation origin was accepted")
	}

	report = validReportFixture()
	report.Handoff.Status = "gap"
	if err := report.Validate(); err == nil {
		t.Fatal("handoff projection mismatch was accepted")
	}
}

func TestFixtureEncodingAndTimestampPlansAreValid(t *testing.T) {
	if err := fixtureEncodingPlan().Validate(); err != nil {
		t.Fatalf("fixture encoding plan is invalid: %v", err)
	}
}

func validReportFixture() Report {
	contract := transcodetimeline.Contract{
		Status:                transcodetimeline.StatusAligned,
		DecisionReason:        transcodetimeline.DecisionClientCertificationPending,
		SeamlessAllowed:       false,
		DiscontinuityRequired: true,
	}
	return Report{
		SchemaVersion: ReportSchemaVersion,
		FixtureID:     FixtureIDSoftwareCFR,
		Tools: ToolReport{
			FFmpegPath:     "/usr/bin/ffmpeg",
			FFmpegVersion:  "ffmpeg version fixture",
			FFprobePath:    "/usr/bin/ffprobe",
			FFprobeVersion: "ffprobe version fixture",
		},
		Source: SourceReport{
			SHA256:     "source-hash",
			DurationMS: 8000,
			Width:      320,
			Height:     180,
			FPSMilli:   30000,
			SampleRate: 48000,
		},
		EncodingPlan: IdentityReport{Version: "encoding-v1", Hash: "encoding-hash", JSON: "{}"},
		TimestampPlan: IdentityReport{Version: "timestamp-v1", Hash: "timestamp-hash", JSON: "{}"},
		Startup: ArtifactReport{
			TimelineOriginMS:   0,
			AttestationVersion: "attestation-v1",
			AttestationHash:    "startup-hash",
		},
		Continuation: ArtifactReport{
			TimelineOriginMS:   4000,
			AttestationVersion: "attestation-v1",
			AttestationHash:    "continuation-hash",
		},
		Handoff: HandoffReport{
			Version:               "handoff-v2",
			Hash:                  "handoff-hash",
			Status:                contract.Status,
			DecisionReason:        contract.DecisionReason,
			SeamlessAllowed:       contract.SeamlessAllowed,
			DiscontinuityRequired: contract.DiscontinuityRequired,
			Contract:              contract,
		},
	}
}
