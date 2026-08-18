package service

import "testing"

func TestMediaAnalysisExecutionModes(t *testing.T) {
	for _, mode := range []string{
		MediaAnalysisModeAuto,
		MediaAnalysisModeClientPreferred,
		MediaAnalysisModeServerOnly,
		MediaAnalysisModeOff,
	} {
		if !isValidMediaAnalysisMode(mode) {
			t.Fatalf("expected mode %q to be valid", mode)
		}
	}
	if isValidMediaAnalysisMode("unknown") {
		t.Fatal("unknown mode must be rejected")
	}
}

func TestMediaAnalysisWorkerEligibility(t *testing.T) {
	cases := []struct {
		name string
		input MediaAnalysisWorkerHeartbeat
		want bool
	}{
		{
			name: "安卓充电并使用无线网络",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "wifi", Charging: true, BatteryPercent: 10, Capabilities: []string{"highlight_v1"}},
			want: true,
		},
		{
			name: "安卓电量充足并使用无线网络",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "wifi", BatteryPercent: 60, Capabilities: []string{"highlight_v1"}},
			want: true,
		},
		{
			name: "安卓低电量不参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "wifi", BatteryPercent: 20, Capabilities: []string{"highlight_v1"}},
			want: false,
		},
		{
			name: "安卓移动网络不参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "cellular", Charging: true, BatteryPercent: 100, Capabilities: []string{"highlight_v1"}},
			want: false,
		},
		{
			name: "桌面节点可参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "desktop", Capabilities: []string{"highlight_v1"}},
			want: true,
		},
		{
			name: "缺少能力声明不参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "desktop"},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := workerEligible(tc.input); got != tc.want {
				t.Fatalf("workerEligible() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMediaAnalysisWorkerUtilities(t *testing.T) {
	if got := normalizeWorkerKind("windows"); got != "desktop" {
		t.Fatalf("windows kind = %q", got)
	}
	if got := normalizeWorkerKind("android"); got != "android" {
		t.Fatalf("android kind = %q", got)
	}
	if got := thumbnailExtension("image/jpeg"); got != ".jpg" {
		t.Fatalf("jpeg extension = %q", got)
	}
	if got := thumbnailExtension("image/webp"); got != ".webp" {
		t.Fatalf("webp extension = %q", got)
	}
}
