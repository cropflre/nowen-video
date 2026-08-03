package recoverystress

func AvailableScenarios() []ScenarioSpec {
	return []ScenarioSpec{
		{
			ID:                          ScenarioCancelActiveWrite,
			Purpose:                     "cancel a production-shaped HLS process after completed segments exist and prove the partial workspace loses readability",
			FaultKind:                   "context_cancel",
			LogicalDurationMicros:       20 * 60 * 1_000_000,
			TriggerMicros:               5 * 60 * 1_000_000,
			ExpectedProcessCount:        1,
			ExpectedFinalJobStatus:      "cancelled",
			ExpectedFinalArtifactStatus: "cancelled",
		},
		{
			ID:                          ScenarioSIGKILLRecovery,
			Purpose:                     "kill the owning FFmpeg process, expire and requeue its Lease, then complete a replacement Attempt",
			FaultKind:                   "sigkill",
			LogicalDurationMicros:       20 * 60 * 1_000_000,
			TriggerMicros:               5 * 60 * 1_000_000,
			ExpectedProcessCount:        2,
			ExpectedFinalJobStatus:      "completed",
			ExpectedFinalArtifactStatus: "published",
			RequiresReplacementAttempt:  true,
		},
		{
			ID:                          ScenarioENOSPCWrite,
			Purpose:                     "inject ENOSPC into HLS segment writes and prove the failed Artifact remains unpublished and cleanup eligible",
			FaultKind:                   "enospc",
			LogicalDurationMicros:       10 * 60 * 1_000_000,
			ExpectedProcessCount:        1,
			ExpectedFinalJobStatus:      "failed",
			ExpectedFinalArtifactStatus: "failed",
			Limits:                      ResourceLimits{ENOSPCAfterBytes: 1_000_000},
		},
		{
			ID:                          ScenarioBoundedResources,
			Purpose:                     "complete production-shaped HLS under one allowed CPU and a 512 MiB address-space ceiling",
			FaultKind:                   "resource_limits",
			LogicalDurationMicros:       30 * 60 * 1_000_000,
			ExpectedProcessCount:        1,
			ExpectedFinalJobStatus:      "completed",
			ExpectedFinalArtifactStatus: "published",
			Limits: ResourceLimits{
				CPUCount:          1,
				AddressSpaceBytes: 512 * 1024 * 1024,
				MemoryMaxBytes:    512 * 1024 * 1024,
			},
		},
		{
			ID:                          ScenarioStaleLeaseFence,
			Purpose:                     "let an old successful worker finalize after Lease replacement and prove both Prepare and Commit are fenced",
			FaultKind:                   "stale_lease_finalize",
			LogicalDurationMicros:       10 * 60 * 1_000_000,
			ExpectedProcessCount:        2,
			ExpectedFinalJobStatus:      "completed",
			ExpectedFinalArtifactStatus: "published",
			RequiresReplacementAttempt:  true,
		},
	}
}

func LookupScenario(id string) (ScenarioSpec, bool) {
	for _, scenario := range AvailableScenarios() {
		if scenario.ID == id {
			return scenario, true
		}
	}
	return ScenarioSpec{}, false
}
