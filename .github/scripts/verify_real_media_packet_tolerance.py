#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

TOLERANCE_TICKS = 1
DECODED_FRAME_POLICY = "perceptual_frame_sequence_v1"
SCALAR_FIELDS = (
    "first_pts",
    "first_dts",
    "last_pts",
    "last_dts",
    "min_composition_offset_ticks",
    "max_composition_offset_ticks",
)
COUNTER_FIELDS = (
    "packet_count",
    "reordered_packet_count",
    "pts_before_dts_count",
    "pts_after_dts_count",
    "pts_equal_dts_count",
    "adjacent_pts_inversion_count",
    "dts_non_monotonic_count",
    "dts_duplicate_count",
    "max_presentation_reorder_depth",
)


def expand(histogram, value_field):
    values = []
    for bucket in histogram:
        values.extend([bucket[value_field]] * bucket["count"])
    return values


def compare_sequence(left, right, label):
    assert len(left) == len(right), f"{label} sample count differs"
    maximum = 0
    for index, (left_value, right_value) in enumerate(zip(left, right)):
        delta = abs(left_value - right_value)
        assert delta <= TOLERANCE_TICKS, f"{label}[{index}] differs by {delta} ticks"
        maximum = max(maximum, delta)
    return maximum


def compare_packet_order(left, right, label):
    assert left["time_base"] == right["time_base"], f"{label} time base differs"
    for field in COUNTER_FIELDS:
        assert left[field] == right[field], f"{label} {field} differs"
    maximum = 0
    for field in SCALAR_FIELDS:
        delta = abs(left[field] - right[field])
        assert delta <= TOLERANCE_TICKS, f"{label} {field} differs by {delta} ticks"
        maximum = max(maximum, delta)
    maximum = max(
        maximum,
        compare_sequence(
            expand(left["dts_delta_histogram"], "delta_ticks"),
            expand(right["dts_delta_histogram"], "delta_ticks"),
            f"{label} DTS deltas",
        ),
        compare_sequence(
            expand(left["composition_offset_histogram"], "offset_ticks"),
            expand(right["composition_offset_histogram"], "offset_ticks"),
            f"{label} composition offsets",
        ),
    )
    return maximum


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()

    report = json.loads(args.report.read_text())
    evidence = report["evidence"]
    assert evidence["packet_order_comparison_tolerance_ticks"] == TOLERANCE_TICKS
    assert evidence["decoded_frame_comparison_policy"] == DECODED_FRAME_POLICY
    maximum = 0
    comparisons = 0
    for case in evidence["cases"]:
        candidates = case["evidence"]["candidates"]
        assert len(candidates) == 2
        assert len(candidates[0]["runs"]) == len(candidates[1]["runs"]) == evidence["repeat_count"]
        for left_run, right_run in zip(candidates[0]["runs"], candidates[1]["runs"]):
            assert left_run["ordinal"] == right_run["ordinal"]
            for window in ("startup", "continuation"):
                maximum = max(
                    maximum,
                    compare_packet_order(
                        left_run[f"{window}_packet_order"],
                        right_run[f"{window}_packet_order"],
                        f"{case['source']['case_id']} run {left_run['ordinal']} {window}",
                    ),
                )
                comparisons += 1
        comparison = case["evidence"]["comparison"]
        assert comparison["startup_packet_order_equivalent"] is True
        assert comparison["continuation_packet_order_equivalent"] is True
        assert comparison["equivalent"] is True

    print(json.dumps({
        "packet_comparisons": comparisons,
        "tolerance_ticks": TOLERANCE_TICKS,
        "maximum_observed_difference_ticks": maximum,
        "decoded_frame_policy": DECODED_FRAME_POLICY,
    }, sort_keys=True))


if __name__ == "__main__":
    main()
