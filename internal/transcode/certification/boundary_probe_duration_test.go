package certification

import (
	"strings"
	"testing"
)

func TestBoundaryProbeInfersTerminalPacketDurationFromDTS(t *testing.T) {
	document := boundaryProbeDocument{
		Packets: []boundaryProbePacket{
			{StreamIndex: 0, PTS: "0", DTS: "0", Duration: "3000"},
			{StreamIndex: 0, PTS: "6000", DTS: "3000", Duration: "3000"},
			{StreamIndex: 0, PTS: "3000", DTS: "6000"},
		},
	}
	evidence, err := document.streamEvidence(boundaryProbeStream{
		Index:       0,
		CodecType:   "video",
		AverageRate: "30/1",
		TimeBase:    "1/90000",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := evidence.Packets[len(evidence.Packets)-1].Duration; got != 3000 {
		t.Fatalf("terminal duration = %d, want 3000", got)
	}
}

func TestBoundaryProbeRejectsMissingDurationBeforeStreamTail(t *testing.T) {
	document := boundaryProbeDocument{
		Packets: []boundaryProbePacket{
			{StreamIndex: 0, PTS: "0", DTS: "0", Duration: "3000"},
			{StreamIndex: 0, PTS: "6000", DTS: "3000"},
			{StreamIndex: 0, PTS: "3000", DTS: "6000", Duration: "3000"},
		},
	}
	_, err := document.streamEvidence(boundaryProbeStream{
		Index:       0,
		CodecType:   "video",
		AverageRate: "30/1",
		TimeBase:    "1/90000",
	})
	if err == nil || !strings.Contains(err.Error(), "before stream tail") {
		t.Fatalf("expected non-terminal duration failure, got %v", err)
	}
}

func TestBoundaryProbeRejectsTerminalDurationInferenceWithoutStrictDTS(t *testing.T) {
	document := boundaryProbeDocument{
		Packets: []boundaryProbePacket{
			{StreamIndex: 0, PTS: "0", DTS: "0", Duration: "3000"},
			{StreamIndex: 0, PTS: "3000", DTS: "0"},
		},
	}
	_, err := document.streamEvidence(boundaryProbeStream{
		Index:       0,
		CodecType:   "video",
		AverageRate: "30/1",
		TimeBase:    "1/90000",
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be inferred from DTS") {
		t.Fatalf("expected strict-DTS inference failure, got %v", err)
	}
}
