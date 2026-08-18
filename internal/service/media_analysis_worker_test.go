package service

import (
	"testing"
	"time"
)

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
		name  string
		input MediaAnalysisWorkerHeartbeat
		want  bool
	}{
		{
			name:  "安卓充电并使用无线网络",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "wifi", Charging: true, BatteryPercent: 10, Capabilities: []string{"highlight_v1"}},
			want:  true,
		},
		{
			name:  "安卓电量充足并使用无线网络",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "wifi", BatteryPercent: 60, Capabilities: []string{"highlight_v1"}},
			want:  true,
		},
		{
			name:  "安卓低电量不参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "wifi", BatteryPercent: 20, Capabilities: []string{"highlight_v1"}},
			want:  false,
		},
		{
			name:  "安卓移动网络不参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "android", Network: "cellular", Charging: true, BatteryPercent: 100, Capabilities: []string{"highlight_v1"}},
			want:  false,
		},
		{
			name:  "桌面节点可参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "desktop", Capabilities: []string{"highlight_v1"}},
			want:  true,
		},
		{
			name:  "缺少能力声明不参与",
			input: MediaAnalysisWorkerHeartbeat{Kind: "desktop"},
			want:  false,
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

func TestPreferredDesktopWorkerSelection(t *testing.T) {
	now := time.Now()
	eligibleDesktop := MediaAnalysisWorkerView{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID:     "desktop-1",
			Kind:         "desktop",
			Capabilities: []string{"highlight_v1"},
		},
		LastSeen: now,
		State:    "idle",
	}

	state := &mediaAnalysisWorkerState{workers: map[string]MediaAnalysisWorkerView{"desktop-1": eligibleDesktop}}
	if !hasPreferredDesktopWorkerLocked(state, now, "android-1") {
		t.Fatal("刚在线且空闲的桌面节点应优先于 Android")
	}

	busy := eligibleDesktop
	busy.State = "busy"
	state.workers["desktop-1"] = busy
	if hasPreferredDesktopWorkerLocked(state, now, "android-1") {
		t.Fatal("正在计算的桌面节点不应阻塞 Android")
	}

	stale := eligibleDesktop
	stale.LastSeen = now.Add(-remoteDesktopPreferenceTTL - time.Second)
	state.workers["desktop-1"] = stale
	if hasPreferredDesktopWorkerLocked(state, now, "android-1") {
		t.Fatal("过期桌面心跳不应阻塞 Android")
	}

	unavailable := eligibleDesktop
	unavailable.State = "unavailable"
	unavailable.Capabilities = nil
	state.workers["desktop-1"] = unavailable
	if hasPreferredDesktopWorkerLocked(state, now, "android-1") {
		t.Fatal("不可用桌面节点不应阻塞 Android")
	}
}

func TestExtendRemoteLeaseRefreshesWorkerHeartbeat(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)

	oldSeen := time.Now().Add(-2 * time.Minute)
	oldLease := time.Now().Add(30 * time.Second)
	state := mediaAnalysisState(analysis)
	state.mu.Lock()
	state.remoteTasks["task-1"] = &mediaAnalysisRemoteTask{
		TaskID:     "task-1",
		ClaimedBy:  "desktop-1",
		ClaimToken: "claim-1",
		LeaseUntil: oldLease,
	}
	state.workers["desktop-1"] = MediaAnalysisWorkerView{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID:     "desktop-1",
			Kind:         "desktop",
			Capabilities: []string{"highlight_v1"},
		},
		LastSeen: oldSeen,
		State:    "busy",
		TaskID:   "task-1",
	}
	state.mu.Unlock()

	analysis.extendRemoteLease("task-1", "claim-1")

	state.mu.Lock()
	defer state.mu.Unlock()
	remote := state.remoteTasks["task-1"]
	worker := state.workers["desktop-1"]
	if !remote.LeaseUntil.After(oldLease) {
		t.Fatalf("进度上报后租约应延长，原=%v 新=%v", oldLease, remote.LeaseUntil)
	}
	if !worker.LastSeen.After(oldSeen) {
		t.Fatalf("进度上报后节点在线时间应刷新，原=%v 新=%v", oldSeen, worker.LastSeen)
	}
	if worker.State != "busy" || worker.TaskID != "task-1" {
		t.Fatalf("续租后节点应保持计算中，state=%q task=%q", worker.State, worker.TaskID)
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
