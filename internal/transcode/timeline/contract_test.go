package timeline

import (
	"testing"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
)

func TestEvaluateAlignedTimelineStillRequiresDiscontinuity(t *testing.T) {
	startup := timelineAttestation("plan", 126000, 306000)
	continuation := timelineAttestation("plan", 396000, 576000)
	// Startup final packet duration is 90000 ticks, therefore EndPTS is 396000
	// and the continuation begins exactly at the next packet boundary.
	contract, err := Evaluate(startup, "att-v1", "startup-hash", continuation, "att-v1", "continuation-hash")
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if contract.Status != StatusAligned || contract.Video.Status != StatusAligned || contract.Audio.Status != StatusAligned {
		t.Fatalf("unexpected aligned contract: %+v", contract)
	}
	if contract.SeamlessAllowed || !contract.DiscontinuityRequired || contract.DecisionReason != DecisionClientCertificationPending {
		t.Fatalf("v1 must retain discontinuity after alignment: %+v", contract)
	}
	versionA, hashA, canonicalA, err := Identity(contract)
	if err != nil {
		t.Fatalf("Identity() error = %v", err)
	}
	versionB, hashB, canonicalB, err := Identity(contract)
	if err != nil {
		t.Fatalf("Identity() second error = %v", err)
	}
	if versionA != SchemaVersion || versionA != versionB || hashA != hashB || canonicalA != canonicalB {
		t.Fatal("timeline identity is not deterministic")
	}
}

func TestEvaluateClassifiesGapAndOverlap(t *testing.T) {
	startup := timelineAttestation("plan", 126000, 306000)

	gap := timelineAttestation("plan", 486000, 666000)
	gapContract, err := Evaluate(startup, "att-v1", "startup", gap, "att-v1", "gap")
	if err != nil {
		t.Fatalf("gap Evaluate() error = %v", err)
	}
	if gapContract.Status != StatusGap || gapContract.Video.PresentationDeltaMicros <= 0 || gapContract.DecisionReason != DecisionTimelineGap {
		t.Fatalf("unexpected gap contract: %+v", gapContract)
	}

	overlap := timelineAttestation("plan", 126000, 306000)
	overlapContract, err := Evaluate(startup, "att-v1", "startup", overlap, "att-v1", "overlap")
	if err != nil {
		t.Fatalf("overlap Evaluate() error = %v", err)
	}
	if overlapContract.Status != StatusOverlap || overlapContract.Video.PresentationDeltaMicros >= 0 || overlapContract.DecisionReason != DecisionTimelineOverlap {
		t.Fatalf("unexpected overlap contract: %+v", overlapContract)
	}
}

func TestEvaluateRejectsStreamIdentityMismatch(t *testing.T) {
	startup := timelineAttestation("plan", 126000, 306000)
	continuation := timelineAttestation("plan", 396000, 576000)
	continuation.First.Audio.SampleRate = 44100
	if _, err := Evaluate(startup, "att-v1", "startup", continuation, "att-v1", "continuation"); err == nil {
		t.Fatal("expected incompatible stream identity rejection")
	}
}

func TestContractV1CannotAuthorizeSeamlessPlayback(t *testing.T) {
	startup := timelineAttestation("plan", 126000, 306000)
	continuation := timelineAttestation("plan", 396000, 576000)
	contract, err := Evaluate(startup, "att-v1", "startup", continuation, "att-v1", "continuation")
	if err != nil {
		t.Fatal(err)
	}
	contract.SeamlessAllowed = true
	contract.DiscontinuityRequired = false
	if err := contract.Validate(); err == nil {
		t.Fatal("timeline v1 unexpectedly authorized seamless playback")
	}
}

func timelineAttestation(planHash string, firstPTS, lastPTS int64) transcodeattestation.Attestation {
	video := transcodeattestation.PacketRange{
		FirstPTS: firstPTS,
		FirstDTS: firstPTS - 7200,
		LastPTS:  lastPTS,
		LastDTS:  lastPTS - 7200,
		EndPTS:   lastPTS + 90000,
		StartMS:  firstPTS * 1000 / 90000,
		EndMS:    (lastPTS + 90000) * 1000 / 90000,
	}
	audio := transcodeattestation.PacketRange{
		FirstPTS: firstPTS,
		FirstDTS: firstPTS,
		LastPTS:  lastPTS,
		LastDTS:  lastPTS,
		EndPTS:   lastPTS + 90000,
		StartMS:  firstPTS * 1000 / 90000,
		EndMS:    (lastPTS + 90000) * 1000 / 90000,
	}
	segment := transcodeattestation.SegmentEvidence{
		Name: "seg0000.ts",
		Video: transcodeattestation.StreamIdentity{
			CodecName:   "h264",
			Width:       1280,
			Height:      720,
			PixelFormat: "yuv420p",
			TimeBase:    "1/90000",
		},
		Audio: transcodeattestation.StreamIdentity{
			CodecName:  "aac",
			Channels:   2,
			SampleRate: 48000,
			TimeBase:   "1/90000",
		},
		Timeline: transcodeattestation.Timeline{Video: video, Audio: audio},
	}
	return transcodeattestation.Attestation{
		SchemaVersion:       transcodeattestation.SchemaVersion,
		Scope:               transcodeattestation.ScopeComplete,
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    planHash,
		SegmentCount:        1,
		First:               segment,
		Last:                segment,
	}
}
