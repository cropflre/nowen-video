package longdrift

import (
	"testing"

	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

func TestScalingRegistryHasBoundedThirtyFourHourMatrix(t *testing.T) {
	shards := AvailableScalingShards()
	if len(shards) != 9 {
		t.Fatalf("unexpected scaling shard count: %d", len(shards))
	}
	var encodedMinutes int64
	for _, shard := range shards {
		tier, ok := LookupScalingTier(shard.TierID)
		if !ok {
			t.Fatalf("missing tier %s", shard.TierID)
		}
		if _, ok := LookupProfile(shard.ProfileID); !ok {
			t.Fatalf("missing profile %s", shard.ProfileID)
		}
		if _, ok := ScalingCandidateTimeBase(shard.CandidateID); !ok {
			t.Fatalf("missing candidate %s", shard.CandidateID)
		}
		encodedMinutes += tier.DurationMicros / 60_000_000
	}
	if encodedMinutes != 2_040 {
		t.Fatalf("unexpected encoded minutes: %d", encodedMinutes)
	}
}

func TestScalingDepthTierComparesBothCandidates(t *testing.T) {
	tier, ok := LookupScalingTier(ScalingTierDepth6H)
	if !ok {
		t.Fatal("six-hour tier is missing")
	}
	if len(tier.ProfileIDs) != 2 || len(tier.CandidateIDs) != 2 {
		t.Fatalf("unexpected six-hour tier: %+v", tier)
	}
	if tier.CandidateIDs[0] != transcodetimebase.CandidateAVTB || tier.CandidateIDs[1] != transcodetimebase.Candidate90K {
		t.Fatalf("unexpected candidate order: %v", tier.CandidateIDs)
	}
	if len(CheckpointTargetsForPolicy(tier.Policy())) != 7 {
		t.Fatalf("unexpected six-hour checkpoint geometry")
	}
}

func TestScalingBreadthTierCoversAllProfiles(t *testing.T) {
	tier, ok := LookupScalingTier(ScalingTierBreadth2H)
	if !ok {
		t.Fatal("two-hour tier is missing")
	}
	if len(tier.ProfileIDs) != len(AvailableProfiles()) {
		t.Fatalf("two-hour tier covers %d profiles, want %d", len(tier.ProfileIDs), len(AvailableProfiles()))
	}
	if len(tier.CandidateIDs) != 1 || tier.CandidateIDs[0] != transcodetimebase.CandidateAVTB {
		t.Fatalf("unexpected breadth candidate policy: %v", tier.CandidateIDs)
	}
}
