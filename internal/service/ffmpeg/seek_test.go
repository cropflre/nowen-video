package ffmpeg

import (
	"reflect"
	"testing"
)

func TestWithInputSeekMicrosPreservesLegacyCentiseconds(t *testing.T) {
	got := WithInputSeekMicros([]string{"-y", "-ss", "30.00", "-i", "source.mp4"}, 30_000_000)
	want := []string{"-y", "-ss", "30.00", "-i", "source.mp4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithInputSeekMicros() = %#v, want %#v", got, want)
	}
}

func TestWithInputSeekMicrosRetainsFramePrecision(t *testing.T) {
	got := WithInputSeekMicros([]string{"-y", "-ss", "30.03", "-i", "source.mp4"}, 30_033_333)
	want := []string{"-y", "-ss", "30.033333", "-i", "source.mp4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithInputSeekMicros() = %#v, want %#v", got, want)
	}
}

func TestWithInputSeekMicrosInsertsBeforeInput(t *testing.T) {
	got := WithInputSeekMicros([]string{"-y", "-copyts", "-i", "source.mp4"}, 29_978_667)
	want := []string{"-y", "-ss", "29.978667", "-copyts", "-i", "source.mp4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithInputSeekMicros() = %#v, want %#v", got, want)
	}
}

func TestWithInputSeekMicrosDoesNotMutateCallerSlice(t *testing.T) {
	original := []string{"-y", "-ss", "30.03", "-i", "source.mp4"}
	_ = WithInputSeekMicros(original, 30_033_333)
	if original[2] != "30.03" {
		t.Fatalf("caller slice was mutated: %#v", original)
	}
}
