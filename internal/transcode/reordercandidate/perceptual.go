package reordercandidate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/bits"
	"strings"
)

const (
	PerceptualHashHexLength      = 32
	PerceptualHashBits           = 128
	PerceptualMaxHammingDistance = 8
)

type PerceptualFrameSequence struct {
	FrameCount     int      `json:"frame_count"`
	FrameHashes    []string `json:"frame_hashes"`
	SequenceSHA256 string   `json:"sequence_sha256"`
}

type PerceptualFrameComparison struct {
	FrameCount           int  `json:"frame_count"`
	ExactHashMatchCount  int  `json:"exact_hash_match_count"`
	MaxHammingDistance   int  `json:"max_hamming_distance"`
	TotalHammingDistance int  `json:"total_hamming_distance"`
	MeanHammingMilli     int  `json:"mean_hamming_milli"`
	Equivalent           bool `json:"equivalent"`
}

func NewPerceptualFrameSequence(hashes []string) (PerceptualFrameSequence, error) {
	sequence := PerceptualFrameSequence{
		FrameCount:  len(hashes),
		FrameHashes: append([]string(nil), hashes...),
	}
	if sequence.FrameCount == 0 {
		return PerceptualFrameSequence{}, fmt.Errorf("perceptual frame sequence is empty")
	}
	digest := sha256.Sum256([]byte(strings.Join(sequence.FrameHashes, "\n")))
	sequence.SequenceSHA256 = hex.EncodeToString(digest[:])
	if err := sequence.Validate(sequence.FrameCount); err != nil {
		return PerceptualFrameSequence{}, err
	}
	return sequence, nil
}

func (s PerceptualFrameSequence) Validate(frameCount int) error {
	if s.FrameCount != frameCount || s.FrameCount <= 0 || len(s.FrameHashes) != s.FrameCount {
		return fmt.Errorf("perceptual frame count is invalid")
	}
	for _, value := range s.FrameHashes {
		if len(value) != PerceptualHashHexLength {
			return fmt.Errorf("perceptual frame hash length is invalid")
		}
		if _, err := hex.DecodeString(value); err != nil {
			return fmt.Errorf("perceptual frame hash is invalid: %w", err)
		}
	}
	digest := sha256.Sum256([]byte(strings.Join(s.FrameHashes, "\n")))
	if s.SequenceSHA256 != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("perceptual frame sequence identity is invalid")
	}
	return nil
}

func BuildPerceptualFrameComparison(left, right PerceptualFrameSequence) PerceptualFrameComparison {
	comparison := PerceptualFrameComparison{}
	if left.FrameCount <= 0 || left.FrameCount != right.FrameCount || len(left.FrameHashes) != left.FrameCount || len(right.FrameHashes) != right.FrameCount {
		return comparison
	}
	comparison.FrameCount = left.FrameCount
	for index := range left.FrameHashes {
		distance, err := perceptualHammingDistance(left.FrameHashes[index], right.FrameHashes[index])
		if err != nil {
			return PerceptualFrameComparison{}
		}
		if distance == 0 {
			comparison.ExactHashMatchCount++
		}
		comparison.TotalHammingDistance += distance
		if distance > comparison.MaxHammingDistance {
			comparison.MaxHammingDistance = distance
		}
	}
	comparison.MeanHammingMilli = (comparison.TotalHammingDistance*1000 + comparison.FrameCount/2) / comparison.FrameCount
	comparison.Equivalent = comparison.MaxHammingDistance <= PerceptualMaxHammingDistance
	return comparison
}

func perceptualHammingDistance(left, right string) (int, error) {
	if len(left) != PerceptualHashHexLength || len(right) != PerceptualHashHexLength {
		return 0, fmt.Errorf("perceptual hash length is invalid")
	}
	leftBytes, err := hex.DecodeString(left)
	if err != nil {
		return 0, err
	}
	rightBytes, err := hex.DecodeString(right)
	if err != nil {
		return 0, err
	}
	distance := 0
	for index := range leftBytes {
		distance += bits.OnesCount8(leftBytes[index] ^ rightBytes[index])
	}
	return distance, nil
}
