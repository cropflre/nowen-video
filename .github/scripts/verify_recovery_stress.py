#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path

SCENARIO_REPORT_SCHEMA = "ffmpeg-recovery-resource-scenario-v4"
AGGREGATE_REPORT_SCHEMA = "ffmpeg-recovery-resource-aggregate-v4"
SCENARIO_SCHEMA = "transcode-recovery-resource-scenario-evidence-v4"
AGGREGATE_SCHEMA = "transcode-recovery-resource-aggregate-evidence-v4"
TIMESTAMP_VERSION = "hls-timestamp-normalization-v1"
SOURCE_PROFILE_CASE = "real-mp4-h264-aac-cfr-30-aac-44100-v1"
FATAL_OUTPUT_ENOSPC = "write_enospc"

SCENARIOS = [
    {
        "id": "cancel-active-segment-write-v1",
        "purpose": "cancel a production-shaped HLS process after completed segments exist and prove the partial workspace loses readability",
        "fault_kind": "context_cancel",
        "logical_duration_micros": 20 * 60 * 1_000_000,
        "trigger_micros": 5 * 60 * 1_000_000,
        "expected_process_count": 1,
        "expected_final_job_status": "cancelled",
        "expected_final_artifact_status": "cancelled",
        "requires_replacement_attempt": False,
        "limits": {"cpu_count": 0, "address_space_bytes": 0, "enospc_after_bytes": 0},
    },
    {
        "id": "sigkill-lease-requeue-restart-v1",
        "purpose": "kill the owning FFmpeg process, expire and requeue its Lease, then complete a replacement Attempt",
        "fault_kind": "sigkill",
        "logical_duration_micros": 20 * 60 * 1_000_000,
        "trigger_micros": 5 * 60 * 1_000_000,
        "expected_process_count": 2,
        "expected_final_job_status": "completed",
        "expected_final_artifact_status": "published",
        "requires_replacement_attempt": True,
        "limits": {"cpu_count": 0, "address_space_bytes": 0, "enospc_after_bytes": 0},
    },
    {
        "id": "enospc-segment-write-v1",
        "purpose": "inject ENOSPC into HLS segment writes and prove the failed Artifact remains unpublished and cleanup eligible",
        "fault_kind": "enospc",
        "logical_duration_micros": 10 * 60 * 1_000_000,
        "trigger_micros": 0,
        "expected_process_count": 1,
        "expected_final_job_status": "failed",
        "expected_final_artifact_status": "failed",
        "requires_replacement_attempt": False,
        "limits": {"cpu_count": 0, "address_space_bytes": 0, "enospc_after_bytes": 1_000_000},
    },
    {
        "id": "bounded-one-core-512m-v1",
        "purpose": "complete production-shaped HLS under one allowed CPU and a 512 MiB address-space ceiling",
        "fault_kind": "resource_limits",
        "logical_duration_micros": 30 * 60 * 1_000_000,
        "trigger_micros": 0,
        "expected_process_count": 1,
        "expected_final_job_status": "completed",
        "expected_final_artifact_status": "published",
        "requires_replacement_attempt": False,
        "limits": {"cpu_count": 1, "address_space_bytes": 512 * 1024 * 1024, "enospc_after_bytes": 0},
    },
    {
        "id": "stale-lease-finalize-fence-v1",
        "purpose": "let an old successful worker finalize after Lease replacement and prove both Prepare and Commit are fenced",
        "fault_kind": "stale_lease_finalize",
        "logical_duration_micros": 10 * 60 * 1_000_000,
        "trigger_micros": 0,
        "expected_process_count": 2,
        "expected_final_job_status": "completed",
        "expected_final_artifact_status": "published",
        "requires_replacement_attempt": True,
        "limits": {"cpu_count": 0, "address_space_bytes": 0, "enospc_after_bytes": 0},
    },
]
SCENARIO_BY_ID = {scenario["id"]: scenario for scenario in SCENARIOS}


def canonical_hash(value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def validate_sha256(value, label):
    assert isinstance(value, str) and len(value) == 64, f"{label} is not SHA-256"
    int(value, 16)


def validate_source_contract(spec, manifest, evidence):
    assert canonical_hash(spec) == evidence["spec_hash"] == manifest["spec_hash"]
    assert canonical_hash(manifest) == evidence["manifest_hash"]
    assert evidence["spec_version"] == manifest["spec_version"]
    assert evidence["manifest_version"] == manifest["schema_version"]
    assert evidence["source_generator_version"] == manifest["generator_version"]
    assert evidence["source_ffmpeg_version"] == manifest["ffmpeg_version"]
    assert evidence["source_ffprobe_version"] == manifest["ffprobe_version"]

    assets = {asset["case_id"]: asset for asset in manifest["assets"]}
    source = evidence["source"]
    assert source["case_id"] == SOURCE_PROFILE_CASE
    asset = assets[SOURCE_PROFILE_CASE]
    assert source["relative_path"] == asset["relative_path"]
    assert source["sha256"] == asset["sha256"]
    assert source["size_bytes"] == asset["size_bytes"]
    assert source["asset_evidence_hash"] == canonical_hash(asset)
    validate_sha256(source["sha256"], "source SHA-256")
    validate_sha256(source["asset_evidence_hash"], "source evidence SHA-256")


def validate_process(process, ordinal):
    assert process["attempt_ordinal"] == ordinal
    validate_sha256(process["command_hash"], "command hash")
    validate_sha256(process["stderr_sha256"], "stderr hash")
    assert process["maximum_progress_micros"] >= 0
    assert process["trigger_observed_micros"] >= 0
    assert process["segment_count"] >= 0
    assert process["max_rss_bytes"] >= 0
    assert process["elapsed_millis"] >= 0
    assert process["cpu_count_limit"] >= 0
    assert process["memory_limit_bytes"] >= 0
    assert isinstance(process["resource_controller"], str)
    assert isinstance(process["fault_backend"], str)
    assert isinstance(process["stderr_markers"], list)
    assert isinstance(process["fatal_output_detected"], bool)
    assert isinstance(process["fatal_output_code"], str)
    assert process["fatal_output_detected"] == bool(process["fatal_output_code"])


def assert_clean_process(process):
    assert process["fatal_output_detected"] is False
    assert process["fatal_output_code"] == ""


def validate_common(evidence, spec, manifest):
    assert evidence["schema_version"] == SCENARIO_SCHEMA
    validate_source_contract(spec, manifest, evidence)
    assert evidence["certification_ffmpeg_version"]
    assert evidence["certification_ffprobe_version"]
    assert evidence["timestamp_plan_version"] == TIMESTAMP_VERSION
    validate_sha256(evidence["timestamp_plan_hash"], "timestamp plan hash")
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True
    assert evidence["passed"] is True

    scenario = evidence["scenario"]
    expected = SCENARIO_BY_ID[scenario["id"]]
    assert scenario == expected
    assert len(evidence["processes"]) == expected["expected_process_count"]
    for ordinal, process in enumerate(evidence["processes"], start=1):
        validate_process(process, ordinal)

    transitions = evidence["transitions"]
    assert len(transitions) >= 3
    for sequence, transition in enumerate(transitions, start=1):
        assert transition["sequence"] == sequence
        assert transition["job_status"]
        assert transition["desired_state"]
        assert transition["reason"]

    artifact = evidence["artifact"]
    assert artifact["final_job_status"] == expected["expected_final_job_status"]
    assert artifact["final_artifact_status"] == expected["expected_final_artifact_status"]
    validate_sha256(evidence["fence"]["first_token_hash"], "first Lease token hash")
    if expected["requires_replacement_attempt"]:
        validate_sha256(evidence["fence"]["second_token_hash"], "second Lease token hash")
        assert evidence["fence"]["first_token_hash"] != evidence["fence"]["second_token_hash"]


def validate_scenario_outcome(evidence):
    scenario_id = evidence["scenario"]["id"]
    scenario = evidence["scenario"]
    processes = evidence["processes"]
    first = processes[0]
    fence = evidence["fence"]
    artifact = evidence["artifact"]

    if scenario_id == "cancel-active-segment-write-v1":
        assert_clean_process(first)
        assert first["cancelled"] is True
        assert first["trigger_observed_micros"] >= scenario["trigger_micros"]
        assert first["segment_count"] > 0
        assert artifact["readable_artifact_id"] == ""
        assert artifact["partial_workspace_quarantined"] is True
        assert artifact["cleanup_eligible"] is True
    elif scenario_id == "sigkill-lease-requeue-restart-v1":
        assert_clean_process(first)
        assert_clean_process(processes[1])
        assert first["signal"] == "SIGKILL"
        assert first["exit_code"] != 0
        assert processes[1]["exit_code"] == 0
        assert fence["lease_expired_requeued"] is True
        assert fence["old_prepare_rejected"] is True
        assert fence["old_commit_rejected"] is True
        assert fence["replacement_lease_different"] is True
        assert fence["replacement_publish_committed"] is True
        assert artifact["readable_artifact_id"]
        assert artifact["partial_workspace_quarantined"] is True
    elif scenario_id == "enospc-segment-write-v1":
        assert first["fatal_output_detected"] is True
        assert first["fatal_output_code"] == FATAL_OUTPUT_ENOSPC
        assert first["fault_backend"] == "dev-full-bind"
        assert "ENOSPC" in first["stderr_markers"]
        assert evidence["error_code"] == FATAL_OUTPUT_ENOSPC
        assert artifact["readable_artifact_id"] == ""
        assert artifact["cleanup_eligible"] is True
    elif scenario_id == "bounded-one-core-512m-v1":
        assert_clean_process(first)
        assert first["exit_code"] == 0
        assert first["resource_controller"] == "cgroup-v2"
        assert first["cpu_count_limit"] == scenario["limits"]["cpu_count"]
        assert first["memory_limit_bytes"] == scenario["limits"]["address_space_bytes"]
        assert 0 < first["max_rss_bytes"] <= scenario["limits"]["address_space_bytes"]
        assert fence["replacement_publish_committed"] is True
        assert artifact["readable_artifact_id"]
    elif scenario_id == "stale-lease-finalize-fence-v1":
        assert_clean_process(first)
        assert_clean_process(processes[1])
        assert first["exit_code"] == 0
        assert processes[1]["exit_code"] == 0
        assert fence["lease_expired_requeued"] is True
        assert fence["old_prepare_rejected"] is True
        assert fence["old_commit_rejected"] is True
        assert fence["replacement_lease_different"] is True
        assert fence["replacement_publish_committed"] is True
        assert artifact["readable_artifact_id"]
        assert artifact["partial_workspace_quarantined"] is True
    else:
        raise AssertionError(f"unexpected scenario {scenario_id}")


def validate_scenario_report(report):
    assert report["schema_version"] == SCENARIO_REPORT_SCHEMA
    evidence = report["evidence"]
    validate_common(evidence, report["spec"], report["manifest"])
    validate_scenario_outcome(evidence)
    assert report["contract_version"] == SCENARIO_SCHEMA
    assert report["contract_hash"] == canonical_hash(evidence)
    return {
        "kind": "scenario",
        "scenario": evidence["scenario"]["id"],
        "processes": len(evidence["processes"]),
        "segments": sum(process["segment_count"] for process in evidence["processes"]),
        "maximum_rss_bytes": max(process["max_rss_bytes"] for process in evidence["processes"]),
        "contract_hash": report["contract_hash"],
    }


def validate_aggregate_report(report):
    assert report["schema_version"] == AGGREGATE_REPORT_SCHEMA
    evidence = report["evidence"]
    assert evidence["schema_version"] == AGGREGATE_SCHEMA
    validate_source_contract(report["spec"], report["manifest"], evidence)
    assert evidence["timestamp_plan_version"] == TIMESTAMP_VERSION
    validate_sha256(evidence["timestamp_plan_hash"], "aggregate timestamp plan hash")
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True
    assert evidence["all_passed"] is True

    bindings = evidence["scenarios"]
    assert len(bindings) == len(SCENARIOS)
    total_processes = 0
    total_segments = 0
    maximum_rss = 0
    for expected, binding in zip(SCENARIOS, bindings):
        assert binding["scenario_id"] == expected["id"]
        scenario_evidence = binding["evidence"]
        validate_common(scenario_evidence, report["spec"], report["manifest"])
        validate_scenario_outcome(scenario_evidence)
        assert scenario_evidence["scenario"] == expected
        assert scenario_evidence["source_generator_version"] == evidence["source_generator_version"]
        assert scenario_evidence["source_ffmpeg_version"] == evidence["source_ffmpeg_version"]
        assert scenario_evidence["source_ffprobe_version"] == evidence["source_ffprobe_version"]
        assert scenario_evidence["certification_ffmpeg_version"] == evidence["certification_ffmpeg_version"]
        assert scenario_evidence["certification_ffprobe_version"] == evidence["certification_ffprobe_version"]
        assert scenario_evidence["timestamp_plan_version"] == evidence["timestamp_plan_version"]
        assert scenario_evidence["timestamp_plan_hash"] == evidence["timestamp_plan_hash"]
        assert scenario_evidence["source"] == evidence["source"]
        assert binding["contract_version"] == SCENARIO_SCHEMA
        assert binding["contract_hash"] == canonical_hash(scenario_evidence)
        total_processes += len(scenario_evidence["processes"])
        for process in scenario_evidence["processes"]:
            total_segments += process["segment_count"]
            maximum_rss = max(maximum_rss, process["max_rss_bytes"])

    assert evidence["total_processes"] == total_processes
    assert evidence["total_segments_observed"] == total_segments
    assert evidence["maximum_rss_bytes"] == maximum_rss
    assert report["contract_version"] == AGGREGATE_SCHEMA
    assert report["contract_hash"] == canonical_hash(evidence)
    return {
        "kind": "aggregate",
        "scenarios": len(bindings),
        "processes": total_processes,
        "segments": total_segments,
        "maximum_rss_bytes": maximum_rss,
        "contract_hash": report["contract_hash"],
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    report = json.loads(args.report.read_text())
    if report.get("schema_version") == SCENARIO_REPORT_SCHEMA:
        summary = validate_scenario_report(report)
    elif report.get("schema_version") == AGGREGATE_REPORT_SCHEMA:
        summary = validate_aggregate_report(report)
    else:
        raise AssertionError(f"unsupported report schema {report.get('schema_version')!r}")
    print(json.dumps(summary, sort_keys=True))


if __name__ == "__main__":
    main()
