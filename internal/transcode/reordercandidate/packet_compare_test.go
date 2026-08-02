package reordercandidate

import "testing"

func TestPacketOrderEquivalentWithinTicksKeepsExactDefault(t *testing.T) {
	left := packetEvidenceForToleranceTest(t, []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 9, DTS: 3},
		{PTS: 3, DTS: 6},
		{PTS: 6, DTS: 9},
	})
	right := packetEvidenceForToleranceTest(t, []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 10, DTS: 4},
		{PTS: 4, DTS: 7},
		{PTS: 7, DTS: 10},
	})
	if PacketOrderEquivalentWithinTicks(left, right, 0) {
		t.Fatal("zero-tick comparison accepted quantized packet evidence")
	}
	if !PacketOrderEquivalentWithinTicks(left, right, 1) {
		t.Fatal("one-tick comparison rejected adjacent packet quantization")
	}
}

func TestPacketOrderEquivalentWithinTicksRejectsExcessDrift(t *testing.T) {
	left := packetEvidenceForToleranceTest(t, []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 9, DTS: 3},
		{PTS: 3, DTS: 6},
		{PTS: 6, DTS: 9},
	})
	right := packetEvidenceForToleranceTest(t, []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 11, DTS: 5},
		{PTS: 5, DTS: 8},
		{PTS: 8, DTS: 11},
	})
	if PacketOrderEquivalentWithinTicks(left, right, 1) {
		t.Fatal("one-tick comparison accepted two-tick packet drift")
	}
}

func TestPacketOrderEquivalentWithinTicksRejectsSemanticDrift(t *testing.T) {
	left := packetEvidenceForToleranceTest(t, []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 9, DTS: 3},
		{PTS: 3, DTS: 6},
		{PTS: 6, DTS: 9},
	})
	right := packetEvidenceForToleranceTest(t, []PacketTimestamp{
		{PTS: 0, DTS: 0},
		{PTS: 4, DTS: 4},
		{PTS: 7, DTS: 7},
		{PTS: 10, DTS: 10},
	})
	if PacketOrderEquivalentWithinTicks(left, right, 1) {
		t.Fatal("packet comparison accepted changed reorder semantics")
	}
}

func packetEvidenceForToleranceTest(t *testing.T, packets []PacketTimestamp) PacketOrderEvidence {
	t.Helper()
	evidence, err := NewPacketOrderEvidence("test", "1/90000", packets)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}
