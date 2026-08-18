package service

import (
	"encoding/json"
	"testing"
	"time"
)

func mustMediaComputeJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test input: %v", err)
	}
	return data
}

func TestMediaComputeGenericClaimEnvelope(t *testing.T) {
	analysis := &MediaAnalysisService{}
	input := mustMediaComputeJSON(t, map[string]any{"media_id": "media-1", "points": 24})
	claim, err := analysis.buildMediaComputeClaim(
		&mediaAnalysisRemoteTask{
			TaskID: "task-1", ClaimToken: "claim-1", LeaseUntil: time.Now().Add(time.Minute),
		},
		mediaComputeTaskDescriptor{
			JobType: "waveform_v1", RequiredCapability: "waveform_v1", Input: input,
		},
	)
	if err != nil {
		t.Fatalf("build generic claim: %v", err)
	}
	if claim.ProtocolVersion != MediaComputeProtocolVersion {
		t.Fatalf("protocol version = %d", claim.ProtocolVersion)
	}
	if claim.JobType != "waveform_v1" || claim.RequiredCapability != "waveform_v1" {
		t.Fatalf("unexpected envelope: job=%q capability=%q", claim.JobType, claim.RequiredCapability)
	}
	if string(claim.Input) != string(input) {
		t.Fatalf("input = %s, want %s", claim.Input, input)
	}
	if claim.MediaID != "" || claim.StreamURL != "" || len(claim.SampleTimes) != 0 {
		t.Fatal("generic jobs must not leak highlight-only compatibility fields")
	}
}

func TestMediaComputeClientProtocolVersion(t *testing.T) {
	cases := map[string]int{
		"desktop-v2/dev": MediaComputeProtocolVersion,
		"android-v2/16":  MediaComputeProtocolVersion,
		"v2/custom":      MediaComputeProtocolVersion,
		"desktop-v1/dev": 1,
		"android-v1/15":  1,
		"":               1,
	}
	for input, want := range cases {
		if got := mediaComputeClientProtocolVersion(input); got != want {
			t.Fatalf("mediaComputeClientProtocolVersion(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestMediaComputeNodeCapabilitiesAreJobIndependent(t *testing.T) {
	heartbeat := MediaAnalysisWorkerHeartbeat{
		Kind: "desktop", Capabilities: []string{" highlight_v1 ", "waveform_v1"},
	}
	if !mediaComputeNodeAvailable(heartbeat) {
		t.Fatal("desktop with any declared capability should be available")
	}
	if !mediaComputeNodeSupportsCapability(heartbeat, "HIGHLIGHT_V1") {
		t.Fatal("capability matching should be case-insensitive and trim whitespace")
	}
	if !mediaComputeNodeCanRun(heartbeat, "waveform_v1") {
		t.Fatal("node should run an independently declared future capability")
	}
	if mediaComputeNodeCanRun(heartbeat, "subtitle_v1") {
		t.Fatal("undeclared capability must not match")
	}
}

func TestMediaComputeAndroidAvailabilityRemainsPowerAndWifiAware(t *testing.T) {
	base := MediaAnalysisWorkerHeartbeat{
		Kind: "android", Network: "wifi", BatteryPercent: 60, Capabilities: []string{"waveform_v1"},
	}
	if !mediaComputeNodeCanRun(base, "waveform_v1") {
		t.Fatal("eligible Android should run its declared capability")
	}
	low := base
	low.BatteryPercent = 20
	if mediaComputeNodeCanRun(low, "waveform_v1") {
		t.Fatal("low-battery Android must remain unavailable")
	}
	cellular := base
	cellular.Network = "cellular"
	cellular.Charging = true
	if mediaComputeNodeCanRun(cellular, "waveform_v1") {
		t.Fatal("Android on cellular must remain unavailable")
	}
}

func TestPreferredDesktopIsScopedToRequiredCapability(t *testing.T) {
	now := time.Now()
	state := &mediaAnalysisWorkerState{workers: map[string]MediaAnalysisWorkerView{
		"desktop-1": {
			MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
				WorkerID: "desktop-1", Kind: "desktop", Capabilities: []string{"thumbnail_v1"},
			},
			LastSeen: now, State: "idle",
		},
	}}
	if !hasPreferredDesktopForCapabilityLocked(state, now, "android-1", "thumbnail_v1") {
		t.Fatal("desktop should be preferred for the capability it can execute")
	}
	if hasPreferredDesktopForCapabilityLocked(state, now, "android-1", "waveform_v1") {
		t.Fatal("desktop must not block Android for a capability it cannot execute")
	}
}

func TestRegisterComputeTaskCreatesReusableDescriptor(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)

	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "task-wave", MediaID: "media-1", Fingerprint: "fp-1",
		JobType: "waveform_v1", RequiredCapability: "waveform_v1",
		Input: mustMediaComputeJSON(t, map[string]any{"media_id": "media-1", "bins": 128}),
	}); err != nil {
		t.Fatalf("register compute task: %v", err)
	}
	descriptor := mediaComputeDescriptor(analysis, "task-wave")
	if descriptor.JobType != "waveform_v1" || descriptor.RequiredCapability != "waveform_v1" {
		t.Fatalf("descriptor = %#v", descriptor)
	}
	state := mediaAnalysisState(analysis)
	state.mu.Lock()
	remote := state.remoteTasks["task-wave"]
	state.mu.Unlock()
	if remote == nil || remote.MediaID != "media-1" || remote.Fingerprint != "fp-1" {
		t.Fatalf("remote task = %#v", remote)
	}

	analysis.UnregisterComputeTask("task-wave")
	state.mu.Lock()
	_, stillRemote := state.remoteTasks["task-wave"]
	state.mu.Unlock()
	if stillRemote {
		t.Fatal("unregister must remove remote task")
	}
	descriptors := mediaComputeDescriptors(analysis)
	descriptors.mu.Lock()
	_, stillDescribed := descriptors.tasks["task-wave"]
	descriptors.mu.Unlock()
	if stillDescribed {
		t.Fatal("unregister must remove generic descriptor")
	}
}

func TestRegisterComputeTaskRejectsInvalidJSON(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)
	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "bad", JobType: "waveform_v1", RequiredCapability: "waveform_v1",
		Input: json.RawMessage("not-json"),
	}); err == nil {
		t.Fatal("invalid generic job input must be rejected")
	}
}

func TestMediaComputeNodeViewReportsRegisteredJob(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)
	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "task-wave", JobType: "waveform_v1", RequiredCapability: "waveform_v1",
		Input: mustMediaComputeJSON(t, map[string]any{"bins": 64}),
	}); err != nil {
		t.Fatalf("register compute task: %v", err)
	}
	now := time.Now()
	view := mediaComputeNodeView(analysis, MediaAnalysisWorkerView{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-1", Kind: "desktop", Version: "desktop-v2/dev",
			Capabilities: []string{"waveform_v1"},
		},
		LastSeen: now, State: "busy", TaskID: "task-wave",
	})
	if view.ClientProtocolVersion != MediaComputeProtocolVersion {
		t.Fatalf("client protocol = %d", view.ClientProtocolVersion)
	}
	if view.CurrentJobType != "waveform_v1" {
		t.Fatalf("current job type = %q", view.CurrentJobType)
	}
	if !view.LastSeen.Equal(now) {
		t.Fatal("node last_seen must be preserved")
	}
}
