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
            "encoder-time-base-avtb-v1": "f30eb7419ce6cd8de3ee00f37acda9c4b3250dfec936ba87ca24ed7e8b34c518",
            "encoder-time-base-90k-v1": "f30eb7419ce6cd8de3ee00f37acda9c4b3250dfec936ba87ca24ed7e8b34c518",
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
            "encoder-time-base-avtb-v1": "2ce6219bc6fb3573270cde1d1758a940ccac7d827333d2db5b32fcd44c8abad7",
            "encoder-time-base-90k-v1": "153cc1158a382acae03a72f03b4c80cdaa1e3319cb8369c851745fdec5bf312c",
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
            "encoder-time-base-avtb-v1": "4b06a6431d1ac8b56ae53e949ee9f332b0d161752fdfc8928f6a12bb074fa212",
            "encoder-time-base-90k-v1": "4b06a6431d1ac8b56ae53e949ee9f332b0d161752fdfc8928f6a12bb074fa212",
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
            "encoder-time-base-avtb-v1": "c7bbeed422316481950869655b139b242355c67b7c81785eff69871ffb41ef27",
            "encoder-time-base-90k-v1": "140a215a61ff6e296d63d03bed61c66ef0ecd608b8eae9738be064924104a700",
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
            "encoder-time-base-avtb-v1": "c7bbeed422316481950869655b139b242355c67b7c81785eff69871ffb41ef27",
            "encoder-time-base-90k-v1": "140a215a61ff6e296d63d03bed61c66ef0ecd608b8eae9738be064924104a700",
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
            "encoder-time-base-avtb-v1": "c7bbeed422316481950869655b139b242355c67b7c81785eff69871ffb41ef27",
            "encoder-time-base-90k-v1": "140a215a61ff6e296d63d03bed61c66ef0ecd608b8eae9738be064924104a700",
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
