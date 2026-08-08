package storageestimate

import "testing"

func TestEstimateKnownDurationUsesCatalogEnvelope(t *testing.T) {
	result, err := Estimate(Input{
		VideoBitrate: "3000k",
		AudioBitrate: "128k",
		DurationMS:   2 * 60 * 60 * 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BitrateBPS != 3_128_000 {
		t.Fatalf("unexpected bitrate: %d", result.BitrateBPS)
	}
	if result.EstimatedBytes <= result.PayloadBytes {
		t.Fatalf("safety envelope missing: %+v", result)
	}
	if result.EstimatedBytes < 3_700_000_000 || result.EstimatedBytes > 3_900_000_000 {
		t.Fatalf("unexpected two-hour estimate: %d", result.EstimatedBytes)
	}
}

func TestEstimateShortArtifactKeepsMinimumReservation(t *testing.T) {
	result, err := Estimate(Input{
		VideoBitrate: "800k",
		AudioBitrate: "96k",
		DurationMS:   30_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.EstimatedBytes != minimumReservationBytes {
		t.Fatalf("short artifact did not receive minimum reservation: %d", result.EstimatedBytes)
	}
}

func TestEstimateUnknownDurationUsesSourceSize(t *testing.T) {
	result, err := Estimate(Input{SourceBytes: 10 * 1024 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	if result.Fallback != "source_size" {
		t.Fatalf("unexpected fallback: %+v", result)
	}
	if result.EstimatedBytes <= 10*1024*1024*1024 {
		t.Fatalf("source safety factor missing: %d", result.EstimatedBytes)
	}
}

func TestEstimateUnknownDurationUsesFailClosedFloor(t *testing.T) {
	result, err := Estimate(Input{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Fallback != "unknown_duration_floor" || result.EstimatedBytes != unknownDurationFloor {
		t.Fatalf("unexpected unknown duration estimate: %+v", result)
	}
}

func TestParseBitrate(t *testing.T) {
	cases := map[string]int64{
		"800k":     800_000,
		"3.5Mbps":  3_500_000,
		"128 kbps": 128_000,
		"6000000":  6_000_000,
	}
	for input, expected := range cases {
		actual, err := ParseBitrate(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		if actual != expected {
			t.Fatalf("parse %q: expected %d got %d", input, expected, actual)
		}
	}
}
