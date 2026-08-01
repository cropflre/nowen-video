package certification

import (
	"strconv"
	"strings"
	"testing"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestBoundaryCasesAreValidUniqueAndRegistered(t *testing.T) {
	cases := AvailableBoundaryCases()
	if len(cases) != 8 {
		t.Fatalf("boundary case count = %d, want 8", len(cases))
	}
	seen := make(map[string]struct{}, len(cases))
	for _, spec := range cases {
		if err := spec.Validate(); err != nil {
			t.Fatalf("boundary case %s is invalid: %v", spec.ID, err)
		}
		registered, ok := LookupBoundaryCase(spec.ID)
		if !ok || registered != spec {
			t.Fatalf("boundary case %s is not registry stable", spec.ID)
		}
		if _, exists := seen[spec.ID]; exists {
			t.Fatalf("duplicate boundary case %s", spec.ID)
		}
		seen[spec.ID] = struct{}{}
	}
	for id, want := range map[string]int64{
		BoundaryCase48KKeyframe:     30_000_000,
		BoundaryCase48KVideoBefore:  29_966_667,
		BoundaryCase48KVideoAfter:   30_033_333,
		BoundaryCase48KAudioBefore:  29_978_667,
		BoundaryCase48KAudioAfter:   30_021_333,
		BoundaryCase44K1Keyframe:    30_000_000,
		BoundaryCase44K1AudioBefore: 29_976_780,
		BoundaryCase44K1AudioAfter:  30_023_220,
	} {
		spec, ok := LookupBoundaryCase(id)
		if !ok || spec.ExpectedBoundaryMicros != want {
			t.Fatalf("boundary %s = %d, want %d", id, spec.ExpectedBoundaryMicros, want)
		}
	}
}

func TestBoundaryHLSArgsRetainMicrosecondSeekAndDuration(t *testing.T) {
	fixture, _ := LookupFixture(FixtureCFR48KZeroLatency)
	continuation, err := boundaryHLSArgs(
		"/media/source.mp4",
		"/cache/continuation",
		transcodetimestamp.Default(),
		fixture,
		30_033_333,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(continuation, " ")
	for _, want := range []string{
		"-copyts -start_at_zero -ss 30.033333",
		"-tune zerolatency",
		"-hls_playlist_type event",
		"-start_number 15",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("continuation command missing %q: %s", want, joined)
		}
	}
	startup, err := boundaryHLSArgs(
		"/media/source.mp4",
		"/cache/startup",
		transcodetimestamp.Default(),
		fixture,
		0,
		30_033_333,
	)
	if err != nil {
		t.Fatal(err)
	}
	joined = strings.Join(startup, " ")
	if !strings.Contains(joined, "-t 30.033333 -hls_playlist_type vod") {
		t.Fatalf("startup duration lost microsecond precision: %s", joined)
	}
	if strings.Contains(joined, "append_list") || strings.Contains(joined, "-hls_playlist_type event") {
		t.Fatalf("startup retained continuation-only HLS options: %s", joined)
	}
}

func TestBoundaryProbeCapturesPacketWindowsAndSideData(t *testing.T) {
	document := boundaryProbeDocument{
		Streams: []boundaryProbeStream{
			{Index: 0, CodecType: transcodeboundary.StreamVideo, AverageRate: "30/1", TimeBase: "1/90000"},
			{Index: 1, CodecType: transcodeboundary.StreamAudio, SampleRate: "48000", TimeBase: "1/90000"},
		},
	}
	for index := 0; index < 8; index++ {
		videoPTS := int64(2_700_000 + index*3000)
		audioPTS := int64(2_700_000 + index*1920)
		document.Packets = append(document.Packets,
			probePacket(0, videoPTS, 3000, "K_", nil),
			probePacket(1, audioPTS, 1920, "K_", func() []boundaryProbePacketSide {
				if index == 0 {
					return []boundaryProbePacketSide{{Type: "Skip Samples", SkipSamples: "1024"}}
				}
				return nil
			}()),
		)
	}
	segment, err := document.evidence("seg0015.ts")
	if err != nil {
		t.Fatal(err)
	}
	videoTail, err := buildSegmentWindow(segment.Video.SegmentName, transcodeboundary.WindowTail, segment.Video)
	if err != nil {
		t.Fatal(err)
	}
	if len(videoTail.Packets) != transcodeboundary.MaxWindowPackets || videoTail.Packets[0].Ordinal != 2 {
		t.Fatalf("unexpected video tail window: %+v", videoTail)
	}
	audioHead, err := buildSegmentWindow(segment.Audio.SegmentName, transcodeboundary.WindowHead, segment.Audio)
	if err != nil {
		t.Fatal(err)
	}
	if len(audioHead.Packets) != transcodeboundary.MaxWindowPackets || audioHead.Packets[0].Ordinal != 0 {
		t.Fatalf("unexpected audio head window: %+v", audioHead)
	}
	if len(audioHead.Packets[0].SideData) != 1 || audioHead.Packets[0].SideData[0].SkipSamples != 1024 {
		t.Fatalf("audio side data was not retained: %+v", audioHead.Packets[0])
	}
}

func TestBuildBoundaryVideoUsesPacketTimingInsteadOfContainerAverageRate(t *testing.T) {
	startup := probeStreamEvidence("seg0014.ts", 0, 3000, 8, 0)
	continuation := probeStreamEvidence("seg0015.ts", 21_000, 3000, 8, 0)
	startup.FrameRateMilli = 30_000
	continuation.FrameRateMilli = 29_970
	evidence, err := buildBoundaryStream(transcodeboundary.StreamVideo, startup, continuation)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.FrameRateMilli != 30_000 {
		t.Fatalf("packet-derived video rate = %d, want 30000", evidence.FrameRateMilli)
	}
	if err := evidenceForContract(evidence).Validate(); err != nil {
		t.Fatalf("packet-derived video evidence is not self-validating: %v", err)
	}
}

func TestBuildBoundaryStreamProjectsAACSamples(t *testing.T) {
	startup := probeStreamEvidence("seg0014.ts", 0, 1920, 8, 48_000)
	continuation := probeStreamEvidence("seg0015.ts", 13_440, 1920, 8, 48_000)
	evidence, err := buildBoundaryStream(transcodeboundary.StreamAudio, startup, continuation)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.AudioDelay == nil {
		t.Fatal("audio delay evidence is missing")
	}
	if evidence.AudioDelay.NominalPacketSamples != 1024 || evidence.AudioDelay.BoundaryDeltaSamples != -1024 {
		t.Fatalf("unexpected AAC sample projection: %+v", evidence.AudioDelay)
	}
	if evidence.Status != transcodeboundary.StatusSinglePacketOverlap || evidence.BoundaryUnitsMilli != -1000 {
		t.Fatalf("unexpected audio boundary classification: %+v", evidence)
	}
}

func TestBoundaryMatrixRejectsMissingCases(t *testing.T) {
	if _, err := BuildBoundaryMatrixReport(nil); err == nil {
		t.Fatal("incomplete boundary matrix was accepted")
	}
}

func probePacket(stream int, pts, duration int64, flags string, sideData []boundaryProbePacketSide) boundaryProbePacket {
	return boundaryProbePacket{
		StreamIndex: stream,
		PTS:         boundaryFlexibleNumber(strconv.FormatInt(pts, 10)),
		DTS:         boundaryFlexibleNumber(strconv.FormatInt(pts, 10)),
		Duration:    boundaryFlexibleNumber(strconv.FormatInt(duration, 10)),
		Flags:       flags,
		SideData:    sideData,
	}
}

func probeStreamEvidence(name string, firstPTS, duration int64, count, sampleRate int) boundaryProbeStreamEvidence {
	packets := make([]boundaryProbePacketEvidence, 0, count)
	for index := 0; index < count; index++ {
		pts := firstPTS + int64(index)*duration
		packets = append(packets, boundaryProbePacketEvidence{PTS: pts, DTS: pts, Duration: duration})
	}
	return boundaryProbeStreamEvidence{
		SegmentName: name,
		TimeBase:    "1/90000",
		SampleRate:  sampleRate,
		Packets:     packets,
	}
}

func evidenceForContract(video transcodeboundary.StreamEvidence) transcodeboundary.Contract {
	contract := transcodeboundary.Contract{
		SchemaVersion:                   transcodeboundary.SchemaVersion,
		CaseID:                         "test-case",
		FixtureID:                      FixtureCFR48KZeroLatency,
		ExpectedBoundaryMicros:         30_000_000,
		FFmpegVersion:                  "ffmpeg test",
		FFprobeVersion:                 "ffprobe test",
		EncodingPlanVersion:            "hls-encoding-plan-v1",
		EncodingPlanHash:               "encoding",
		TimestampPlanVersion:           "hls-timestamp-normalization-v1",
		TimestampPlanHash:              "timestamp",
		StartupAttestationVersion:      "hls-produced-media-attestation-v1",
		StartupAttestationHash:         "startup",
		ContinuationAttestationVersion: "hls-produced-media-attestation-v1",
		ContinuationAttestationHash:    "continuation",
		Video:                          video,
		DiscontinuityRequired:          true,
	}
	audioStartup := probeStreamEvidence("seg0014.ts", 0, 1920, 8, 48_000)
	audioContinuation := probeStreamEvidence("seg0015.ts", 13_440, 1920, 8, 48_000)
	contract.Audio, _ = buildBoundaryStream(transcodeboundary.StreamAudio, audioStartup, audioContinuation)
	return contract
}
