package reordercandidate

import (
	"strings"
	"testing"
)

func TestPerceptualFrameComparisonAllowsSmallLossyDifference(t *testing.T) {
	left, err := NewPerceptualFrameSequence([]string{
		strings.Repeat("0", PerceptualHashHexLength),
		strings.Repeat("f", PerceptualHashHexLength),
	})
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewPerceptualFrameSequence([]string{
		"00000000000000000000000000000003",
		strings.Repeat("f", PerceptualHashHexLength),
	})
	if err != nil {
		t.Fatal(err)
	}
	comparison := BuildPerceptualFrameComparison(left, right)
	if !comparison.Equivalent || comparison.MaxHammingDistance != 2 || comparison.ExactHashMatchCount != 1 {
		t.Fatalf("unexpected perceptual comparison: %+v", comparison)
	}
}

func TestPerceptualFrameComparisonRejectsFrameSubstitution(t *testing.T) {
	left, err := NewPerceptualFrameSequence([]string{strings.Repeat("0", PerceptualHashHexLength)})
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewPerceptualFrameSequence([]string{strings.Repeat("f", PerceptualHashHexLength)})
	if err != nil {
		t.Fatal(err)
	}
	comparison := BuildPerceptualFrameComparison(left, right)
	if comparison.Equivalent || comparison.MaxHammingDistance != PerceptualHashBits {
		t.Fatalf("frame substitution was accepted: %+v", comparison)
	}
}

func TestPerceptualFrameSequenceRejectsIdentityDrift(t *testing.T) {
	sequence, err := NewPerceptualFrameSequence([]string{strings.Repeat("0", PerceptualHashHexLength)})
	if err != nil {
		t.Fatal(err)
	}
	sequence.SequenceSHA256 = strings.Repeat("0", 64)
	if err := sequence.Validate(1); err == nil {
		t.Fatal("perceptual sequence identity drift was accepted")
	}
}
