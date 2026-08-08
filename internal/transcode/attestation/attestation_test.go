package attestation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
)

func TestIdentityIsDeterministicAndMatchesEncodingPlan(t *testing.T) {
	planVersion, planHash, planJSON := testEncodingPlanIdentity(t)
	value := testAttestation(planVersion, planHash)

	versionA, hashA, canonicalA, err := Identity(value)
	if err != nil {
		t.Fatalf("Identity() error = %v", err)
	}
	versionB, hashB, canonicalB, err := Identity(value)
	if err != nil {
		t.Fatalf("Identity() second error = %v", err)
	}
	if versionA != SchemaVersion || versionA != versionB || hashA != hashB || canonicalA != canonicalB {
		t.Fatalf("identity is not deterministic")
	}
	if err := VerifyAgainstEncodingPlan(value, planVersion, planHash, planJSON); err != nil {
		t.Fatalf("VerifyAgainstEncodingPlan() error = %v", err)
	}
}

func TestVerifyAgainstEncodingPlanRejectsActualPixelFormatMismatch(t *testing.T) {
	planVersion, planHash, planJSON := testEncodingPlanIdentity(t)
	value := testAttestation(planVersion, planHash)
	value.Last.Video.PixelFormat = "yuv420p10le"

	if err := VerifyAgainstEncodingPlan(value, planVersion, planHash, planJSON); err == nil {
		t.Fatalf("expected pixel-format mismatch")
	}
}

func TestBridgeCompatibleRequiresActualStreamIdentity(t *testing.T) {
	planVersion, planHash, _ := testEncodingPlanIdentity(t)
	startup := testAttestation(planVersion, planHash)
	continuation := testAttestation(planVersion, planHash)
	continuation.First.Audio.SampleRate = 44100

	if err := BridgeCompatible(startup, continuation); err == nil {
		t.Fatalf("expected bridge audio identity mismatch")
	}
}

func TestVerifierProbesFirstAndLastSegments(t *testing.T) {
	planVersion, planHash, planJSON := testEncodingPlanIdentity(t)
	directory := t.TempDir()
	manifestPath := filepath.Join(directory, "stream.m3u8")
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2.0,\nseg000.ts\n#EXTINF:2.0,\nseg001.ts\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	runner := &fakeRunner{responses: map[string][]byte{
		"seg000.ts": []byte(testProbeJSON(126000, 306000)),
		"seg001.ts": []byte(testProbeJSON(306000, 486000)),
	}}
	verifier := Verifier{FFprobePath: "/usr/bin/ffprobe", Runner: runner}
	value, err := verifier.Verify(context.Background(), VerifyRequest{
		ManifestPath:        manifestPath,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		EncodingPlanJSON:    planJSON,
		Scope:               ScopeComplete,
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if value.SegmentCount != 2 || value.First.Name != "seg000.ts" || value.Last.Name != "seg001.ts" {
		t.Fatalf("unexpected segment evidence: %+v", value)
	}
	if value.First.Timeline.Video.FirstPTS != 126000 || value.Last.Timeline.Video.EndPTS != 576000 {
		t.Fatalf("unexpected timeline checkpoints: first=%+v last=%+v", value.First.Timeline.Video, value.Last.Timeline.Video)
	}
	if len(runner.calls) != 2 || !strings.HasSuffix(runner.calls[0], "seg000.ts") || !strings.HasSuffix(runner.calls[1], "seg001.ts") {
		t.Fatalf("unexpected ffprobe calls: %v", runner.calls)
	}
}

func TestVerifierRejectsUnsafeManifestURI(t *testing.T) {
	planVersion, planHash, planJSON := testEncodingPlanIdentity(t)
	manifestPath := filepath.Join(t.TempDir(), "stream.m3u8")
	if err := os.WriteFile(manifestPath, []byte("#EXTM3U\n../seg000.ts\n"), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	_, err := (Verifier{Runner: &fakeRunner{}}).Verify(context.Background(), VerifyRequest{
		ManifestPath:        manifestPath,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		EncodingPlanJSON:    planJSON,
		Scope:               ScopeComplete,
	})
	if err == nil {
		t.Fatalf("expected unsafe manifest rejection")
	}
}

type fakeRunner struct {
	responses map[string][]byte
	calls     []string
}

func (r *fakeRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("missing ffprobe args")
	}
	segment := filepath.Base(args[len(args)-1])
	r.calls = append(r.calls, segment)
	response, ok := r.responses[segment]
	if !ok {
		return nil, fmt.Errorf("no response for %s", segment)
	}
	return response, nil
}

func testEncodingPlanIdentity(t *testing.T) (string, string, string) {
	t.Helper()
	plan := encodingplan.Plan{
		SchemaVersion: encodingplan.SchemaVersion,
		ProfileID:     "720p",
		Transport: encodingplan.TransportPlan{
			Protocol:          "hls",
			Container:         "mpegts",
			SegmentFormat:     "mpegts",
			SegmentDurationMS: 2000,
		},
		Video: encodingplan.VideoPlan{
			Codec:                "h264",
			Width:                1280,
			Height:               720,
			PixelFormatContract:  "yuv420p-8bit",
			FrameRatePolicy:      "source",
			SourceFrameRateMilli: 25000,
			GOPSize:              50,
			KeyframeIntervalMS:   2000,
			ForceKeyframes:       true,
			SceneCut:             false,
			ColorPolicy:          "source_sdr",
			ColorPrimaries:       "source",
			Transfer:             "source",
			Matrix:               "source",
		},
		Audio: encodingplan.AudioPlan{
			Codec:            "aac",
			Bitrate:          "128k",
			Channels:         2,
			Track:            -1,
			SampleRatePolicy: "source",
		},
	}
	version, hash, canonical, err := encodingplan.Identity(plan)
	if err != nil {
		t.Fatalf("encodingplan.Identity() error = %v", err)
	}
	return version, hash, canonical
}

func testAttestation(planVersion, planHash string) Attestation {
	segment := SegmentEvidence{
		Name: "seg000.ts",
		Video: StreamIdentity{
			CodecName:   "h264",
			Profile:     "High",
			Level:       31,
			Width:       1280,
			Height:      720,
			PixelFormat: "yuv420p",
			TimeBase:    "1/90000",
		},
		Audio: StreamIdentity{
			CodecName:  "aac",
			Channels:   2,
			SampleRate: 48000,
			TimeBase:   "1/90000",
		},
		Timeline: Timeline{
			Video: PacketRange{FirstPTS: 126000, FirstDTS: 118800, LastPTS: 306000, LastDTS: 298800, EndPTS: 313200, StartMS: 1400, EndMS: 3480},
			Audio: PacketRange{FirstPTS: 126000, FirstDTS: 126000, LastPTS: 304080, LastDTS: 304080, EndPTS: 306000, StartMS: 1400, EndMS: 3400},
		},
	}
	return Attestation{
		SchemaVersion:       SchemaVersion,
		Scope:               ScopeComplete,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		SegmentCount:        1,
		First:               segment,
		Last:                segment,
	}
}

func testProbeJSON(firstPTS, lastPTS int64) string {
	return fmt.Sprintf(`{
  "streams": [
    {"index":0,"codec_name":"h264","codec_type":"video","profile":"High","level":31,"width":1280,"height":720,"pix_fmt":"yuv420p","avg_frame_rate":"25/1","r_frame_rate":"25/1","time_base":"1/90000"},
    {"index":1,"codec_name":"aac","codec_type":"audio","profile":"LC","channels":2,"sample_rate":"48000","time_base":"1/90000"}
  ],
  "packets": [
    {"stream_index":0,"pts":%d,"dts":%d,"duration":90000,"pts_time":"%.3f","dts_time":"%.3f","duration_time":"1.000"},
    {"stream_index":1,"pts":%d,"dts":%d,"duration":1920,"pts_time":"%.3f","dts_time":"%.3f","duration_time":"0.021333"},
    {"stream_index":0,"pts":%d,"dts":%d,"duration":90000,"pts_time":"%.3f","dts_time":"%.3f","duration_time":"1.000"},
    {"stream_index":1,"pts":%d,"dts":%d,"duration":1920,"pts_time":"%.3f","dts_time":"%.3f","duration_time":"0.021333"}
  ]
}`,
		firstPTS, firstPTS-7200, float64(firstPTS)/90000, float64(firstPTS-7200)/90000,
		firstPTS, firstPTS, float64(firstPTS)/90000, float64(firstPTS)/90000,
		lastPTS, lastPTS-7200, float64(lastPTS)/90000, float64(lastPTS-7200)/90000,
		lastPTS, lastPTS, float64(lastPTS)/90000, float64(lastPTS)/90000,
	)
}
