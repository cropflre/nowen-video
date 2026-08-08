package service

import (
	"encoding/json"
	"testing"
)

func TestAudioTrackInfoJSONContract(t *testing.T) {
	track := AudioTrackInfo{
		Index:    4,
		AudioIdx: 1,
		Codec:    "dts",
		Language: "jpn",
		Title:    "Japanese",
		Channels: 6,
		Default:  true,
	}
	encoded, err := json.Marshal(track)
	if err != nil {
		t.Fatal(err)
	}
	var decoded AudioTrackInfo
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != track {
		t.Fatalf("audio track contract changed: got=%+v want=%+v", decoded, track)
	}
}
