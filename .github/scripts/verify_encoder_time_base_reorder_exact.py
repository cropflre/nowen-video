#!/usr/bin/env python3
import hashlib
import json
import sys
from pathlib import Path

EXPECTED_FFMPEG = "ffmpeg version 6.1.1-3ubuntu5 Copyright (c) 2000-2023 the FFmpeg developers"
EXPECTED_FFPROBE = "ffprobe version 6.1.1-3ubuntu5 Copyright (c) 2007-2023 the FFmpeg developers"

EXPECTED = {
    "reorder-cfr-24-b2-origin-zero-v1": {
        "metrics": {
            "frames": [720, 240],
            "dominant_us": [41667, 41667],
            "video_boundary_delta_us": -41667,
            "audio_boundary_delta_us": -79000,
            "startup_end_skew_us": 16000,
            "continuation_start_skew_us": -21333,
            "boundary_delta_skew_us": -37333,
            "reordered": [255, 85],
            "depth": [2, 2],
            "max_cts_us": [125000, 125000],
            "exact_pixels": [True, True],
            "perceptual_max": [0, 0],
        },
        "semantic_sha256": {
            "encoder-time-base-avtb-v1": "d6cd5ccf2346bb3210f3104bc09f400f01c90c03650a477cd381815c18ed91f5",
            "encoder-time-base-90k-v1": "d6cd5ccf2346bb3210f3104bc09f400f01c90c03650a477cd381815c18ed91f5",
        },
    },
    "reorder-cfr-30000-1001-b3-origin-zero-v1": {
        "metrics": {
            "frames": [900, 299],
            "dominant_us": [33367, 33367],
            "video_boundary_delta_us": -33367,
            "audio_boundary_delta_us": -70700,
            "startup_end_skew_us": -14000,
            "continuation_start_skew_us": -51333,
            "boundary_delta_skew_us": -37333,
            "reordered": [240, 80],
            "depth": [3, 3],
            "max_cts_us": [133467, 133467],
            "exact_pixels": [False, True],
            "perceptual_max": [0, 0],
        },
        "semantic_sha256": {
            "encoder-time-base-avtb-v1": "b361f212431e1aa7a2e044dbfb339f30d1c732d663ee17ca6d68efb4f01cc462",
            "encoder-time-base-90k-v1": "5ada6289ae2e040ce92eadbb214717e8a72ef49ffa323b5f7e7e06c74197723e",
        },
    },
    "reorder-vfr-24-30-b3-origin-zero-v1": {
        "metrics": {
            "frames": [780, 300],
            "dominant_us": [41667, 33333],
            "video_boundary_delta_us": -41667,
            "audio_boundary_delta_us": -79000,
            "startup_end_skew_us": 16000,
            "continuation_start_skew_us": -21333,
            "boundary_delta_skew_us": -37333,
            "reordered": [210, 80],
            "depth": [3, 3],
            "max_cts_us": [166667, 133333],
            "exact_pixels": [True, True],
            "perceptual_max": [0, 0],
        },
        "semantic_sha256": {
            "encoder-time-base-avtb-v1": "b521aef133e41b3f7aabe9edefb6188ccbdee00133e0edbd5b6051584c012936",
            "encoder-time-base-90k-v1": "b521aef133e41b3f7aabe9edefb6188ccbdee00133e0edbd5b6051584c012936",
        },
    },
    "reorder-cfr-30-b3-origin-positive-5s-v1": {
        "metrics": {
            "frames": [900, 300],
            "dominant_us": [33333, 33333],
            "video_boundary_delta_us": -33333,
            "audio_boundary_delta_us": -70667,
            "startup_end_skew_us": 16000,
            "continuation_start_skew_us": -21333,
            "boundary_delta_skew_us": -37334,
            "reordered": [240, 80],
            "depth": [3, 3],
            "max_cts_us": [133333, 133333],
            "exact_pixels": [False, False],
            "perceptual_max": [0, 0],
        },
        "semantic_sha256": {
            "encoder-time-base-avtb-v1": "f74aafdce45035d481c3429dc03134abdd576db53f2688cdd7f74db886ce4a3e",
            "encoder-time-base-90k-v1": "9b1db75ed5482506e035e16bd8381f7c4e6792dfdd5694638280a13eb9488401",
        },
    },
    "reorder-cfr-30-b3-origin-negative-2s-v1": {
        "metrics": {
            "frames": [900, 300],
            "dominant_us": [33333, 33333],
            "video_boundary_delta_us": -33333,
            "audio_boundary_delta_us": -70667,
            "startup_end_skew_us": 16000,
            "continuation_start_skew_us": -21333,
            "boundary_delta_skew_us": -37334,
            "reordered": [240, 80],
            "depth": [3, 3],
            "max_cts_us": [133333, 133333],
            "exact_pixels": [False, False],
            "perceptual_max": [0, 0],
        },
        "semantic_sha256": {
            "encoder-time-base-avtb-v1": "f74aafdce45035d481c3429dc03134abdd576db53f2688cdd7f74db886ce4a3e",
            "encoder-time-base-90k-v1": "9b1db75ed5482506e035e16bd8381f7c4e6792dfdd5694638280a13eb9488401",
        },
    },
    "reorder-cfr-30-b3-long-gop-origin-zero-v1": {
        "metrics": {
            "frames": [900, 300],
            "dominant_us": [33333, 33333],
            "video_boundary_delta_us": -33333,
            "audio_boundary_delta_us": -70667,
            "startup_end_skew_us": 16000,
            "continuation_start_skew_us": -21333,
            "boundary_delta_skew_us": -37334,
            "reordered": [240, 80],
            "depth": [3, 3],
            "max_cts_us": [133333, 133333],
            "exact_pixels": [False, False],
            "perceptual_max": [0, 0],
        },
        "semantic_sha256": {
            "encoder-time-base-avtb-v1": "f74aafdce45035d481c3429dc03134abdd576db53f2688cdd7f74db886ce4a3e",
            "encoder-time-base-90k-v1": "9b1db75ed5482506e035e16bd8381f7c4e6792dfdd5694638280a13eb9488401",
        },
    },
}


def fail(message: str) -> None:
    raise AssertionError(message)


def without(mapping: dict, excluded: set[str]) -> dict:
    return {key: value for key, value in mapping.items() if key not in excluded}


def semantic_run(run: dict) -> dict:
    base = run["base"]
    boundary = base["boundary"]
    av_sync = base["av_sync"]
    return {
        "startup_timeline": without(base["startup_timeline"], {"kind"}),
        "continuation_timeline": without(base["continuation_timeline"], {"kind"}),
        "startup_mapping": base["startup_mapping"],
        "continuation_mapping": base["continuation_mapping"],
        "startup_fingerprint": base["startup_fingerprint"],
        "continuation_fingerprint": base["continuation_fingerprint"],
        "boundary": {
            "expected_boundary_micros": boundary["expected_boundary_micros"],
            "video": boundary["video"],
            "audio": boundary["audio"],
            "seamless_allowed": boundary["seamless_allowed"],
            "discontinuity_required": boundary["discontinuity_required"],
        },
        "av_sync": without(
            av_sync,
            {
                "schema_version",
                "case_id",
                "fixture_id",
                "boundary_evidence_version",
                "boundary_evidence_hash",
            },
        ),
        "startup_packet_order": without(run["startup_packet_order"], {"kind"}),
        "continuation_packet_order": without(run["continuation_packet_order"], {"kind"}),
        "startup_perceptual_sequence": run["startup_perceptual_sequence"],
        "continuation_perceptual_sequence": run["continuation_perceptual_sequence"],
    }


def semantic_sha256(run: dict) -> str:
    canonical = json.dumps(
        semantic_run(run),
        ensure_ascii=False,
        separators=(",", ":"),
        sort_keys=True,
    ).encode()
    return hashlib.sha256(canonical).hexdigest()


def observed_metrics(case: dict) -> dict:
    summary = case["candidates"][0]["summary"]
    base = summary["base"]
    comparison = case["comparison"]
    return {
        "frames": [
            base["startup_frame_count"]["min"],
            base["continuation_frame_count"]["min"],
        ],
        "dominant_us": [
            base["startup_dominant_delta_micros"]["min"],
            base["continuation_dominant_delta_micros"]["min"],
        ],
        "video_boundary_delta_us": base["video_boundary_delta_micros"]["min"],
        "audio_boundary_delta_us": base["audio_boundary_delta_micros"]["min"],
        "startup_end_skew_us": base["startup_end_skew_micros"]["min"],
        "continuation_start_skew_us": base["continuation_start_skew_micros"]["min"],
        "boundary_delta_skew_us": base["boundary_delta_skew_micros"]["min"],
        "reordered": [
            summary["startup_reordered_packet_count"]["min"],
            summary["continuation_reordered_packet_count"]["min"],
        ],
        "depth": [
            summary["startup_max_reorder_depth"]["min"],
            summary["continuation_max_reorder_depth"]["min"],
        ],
        "max_cts_us": [
            summary["startup_max_composition_offset_micros"]["min"],
            summary["continuation_max_composition_offset_micros"]["min"],
        ],
        "exact_pixels": [
            comparison["base"]["startup_sequence_equivalent"],
            comparison["base"]["continuation_sequence_equivalent"],
        ],
        "perceptual_max": [
            comparison["startup_perceptual_comparison"]["max_hamming_distance"],
            comparison["continuation_perceptual_comparison"]["max_hamming_distance"],
        ],
    }


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: verify_encoder_time_base_reorder_exact.py REPORT.json")
    report = json.loads(Path(sys.argv[1]).read_text())
    evidence = report["evidence"]
    if evidence["ffmpeg_version"] != EXPECTED_FFMPEG or evidence["ffprobe_version"] != EXPECTED_FFPROBE:
        fail("reference FFmpeg/FFprobe toolchain drifted")
    cases = evidence["cases"]
    if [case["case"]["base"]["id"] for case in cases] != list(EXPECTED):
        fail("exact reorder case registry drifted")
    for case in cases:
        case_id = case["case"]["base"]["id"]
        expected = EXPECTED[case_id]
        metrics = observed_metrics(case)
        if metrics != expected["metrics"]:
            fail(f"exact reorder metrics drifted for {case_id}: {metrics!r}")
        for candidate in case["candidates"]:
            candidate_id = candidate["spec"]["id"]
            expected_digest = expected["semantic_sha256"][candidate_id]
            digests = [semantic_sha256(run) for run in candidate["runs"]]
            if digests != [expected_digest, expected_digest, expected_digest]:
                fail(
                    f"exact semantic evidence drifted for {case_id}/{candidate_id}: {digests!r}"
                )
    print(json.dumps({case_id: value["metrics"] for case_id, value in EXPECTED.items()}, sort_keys=True))


if __name__ == "__main__":
    main()
