package reordercandidate

import "testing"

func TestPacketOrderEvidenceDetectsBFrameReorder(t *testing.T) {
	packets := []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 9_000, DTS: 3_000},
		{PTS: 3_000, DTS: 6_000},
		{PTS: 6_000, DTS: 9_000},
		{PTS: 12_000, DTS: 12_000},
	}
	evidence, err := NewPacketOrderEvidence("test", "1/90000", packets)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.PacketCount != len(packets) || evidence.ReorderedPacketCount != 3 {
		t.Fatalf("unexpected packet counters: %+v", evidence)
	}
	if evidence.AdjacentPTSInversionCount != 1 || evidence.MaxPresentationReorderDepth == 0 {
		t.Fatalf("reorder was not detected: %+v", evidence)
	}
	if evidence.DTSNonMonotonicCount != 0 || evidence.DTSDuplicateCount != 0 {
		t.Fatalf("DTS should be strictly monotonic: %+v", evidence)
	}
	if len(evidence.DTSDeltaHistogram) != 1 || evidence.DTSDeltaHistogram[0].DeltaTicks != 3_000 || evidence.DTSDeltaHistogram[0].Count != 4 {
		t.Fatalf("unexpected DTS histogram: %+v", evidence.DTSDeltaHistogram)
	}
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPacketOrderEvidenceRecordsDuplicateDTS(t *testing.T) {
	evidence, err := NewPacketOrderEvidence("test", "1/90000", []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 6_000, DTS: 3_000},
		{PTS: 3_000, DTS: 3_000},
		{PTS: 9_000, DTS: 6_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if evidence.DTSDuplicateCount != 1 {
		t.Fatalf("duplicate DTS was not recorded: %+v", evidence)
	}
}

func TestPacketOrderSignatureIgnoresKind(t *testing.T) {
	evidence, err := NewPacketOrderEvidence("left", "1/90000", []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 9_000, DTS: 3_000},
		{PTS: 3_000, DTS: 6_000},
	})
	if err != nil {
		t.Fatal(err)
	}
	other := evidence
	other.Kind = "right"
	if PacketOrderSignature(evidence) != PacketOrderSignature(other) {
		t.Fatal("packet-order signature should ignore evidence kind")
	}
}
