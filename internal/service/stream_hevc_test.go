package service

import "testing"

func TestClientSupportsHEVCForNativeClients(t *testing.T) {
	s := &StreamService{}
	for _, client := range []string{"Hills/1.0", "hills", "Yamby/1.0", "YAMBY", "Infuse/8", "ExoPlayerLib/2"} {
		if !s.ClientSupportsHEVC(client) {
			t.Errorf("native client %q should support HEVC direct play", client)
		}
	}
	if s.ClientSupportsHEVC("unknown-client") {
		t.Fatal("unknown clients must retain conservative playback selection")
	}
}
