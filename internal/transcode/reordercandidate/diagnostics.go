package reordercandidate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CandidateDivergenceDiagnostic returns the first concrete evidence difference
// between two candidates. It is intentionally diagnostic-only and does not
// alter candidate equivalence or tolerance policy.
func CandidateDivergenceDiagnostic(a, b CandidateEvidence) string {
	comparison := BuildCandidateComparison(a, b)
	parts := make([]string, 0, 3)
	if !comparison.Base.Equivalent {
		parts = append(parts, fmt.Sprintf("base=%+v", comparison.Base))
	}
	limit := len(a.Runs)
	if len(b.Runs) < limit {
		limit = len(b.Runs)
	}
	for index := 0; index < limit; index++ {
		if PacketOrderSignature(a.Runs[index].StartupPacketOrder) != PacketOrderSignature(b.Runs[index].StartupPacketOrder) {
			parts = append(parts, fmt.Sprintf(
				"startup_run_%02d=%s",
				index+1,
				packetOrderDifference(a.Runs[index].StartupPacketOrder, b.Runs[index].StartupPacketOrder),
			))
			break
		}
	}
	for index := 0; index < limit; index++ {
		if PacketOrderSignature(a.Runs[index].ContinuationPacketOrder) != PacketOrderSignature(b.Runs[index].ContinuationPacketOrder) {
			parts = append(parts, fmt.Sprintf(
				"continuation_run_%02d=%s",
				index+1,
				packetOrderDifference(a.Runs[index].ContinuationPacketOrder, b.Runs[index].ContinuationPacketOrder),
			))
			break
		}
	}
	if len(a.Runs) != len(b.Runs) {
		parts = append(parts, fmt.Sprintf("run_count=%d/%d", len(a.Runs), len(b.Runs)))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("comparison=%+v", comparison)
	}
	return strings.Join(parts, "; ")
}

func packetOrderDifference(left, right PacketOrderEvidence) string {
	left.Kind = ""
	right.Kind = ""
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return fmt.Sprintf("left=%+v right=%+v", left, right)
	}
	return fmt.Sprintf("left=%s right=%s", leftJSON, rightJSON)
}
