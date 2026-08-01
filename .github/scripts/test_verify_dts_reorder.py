#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path

MODULE_PATH = Path(__file__).with_name("verify_dts_reorder.py")
SPEC = importlib.util.spec_from_file_location("verify_dts_reorder", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class AnalyzePacketsTest(unittest.TestCase):
    def test_accepts_deterministic_b_frame_reorder(self) -> None:
        packets = [
            MODULE.Packet(pts=0, dts=-2, duration=1, flags="K_"),
            MODULE.Packet(pts=3, dts=-1, duration=1, flags="__"),
            MODULE.Packet(pts=1, dts=0, duration=1, flags="__"),
            MODULE.Packet(pts=2, dts=1, duration=1, flags="__"),
            MODULE.Packet(pts=6, dts=2, duration=1, flags="__"),
            MODULE.Packet(pts=4, dts=3, duration=1, flags="__"),
            MODULE.Packet(pts=5, dts=4, duration=1, flags="__"),
            MODULE.Packet(pts=7, dts=5, duration=1, flags="__"),
        ]
        evidence = MODULE.analyze_packets(packets, "1/1", 3)
        self.assertEqual(evidence["packet_count"], 8)
        self.assertGreater(evidence["presentation_backstep_count"], 0)
        self.assertGreater(evidence["positive_composition_offset_count"], 0)
        self.assertEqual(evidence["non_increasing_dts_count"], 0)

    def test_rejects_decode_order_without_b_frames(self) -> None:
        packets = [
            MODULE.Packet(pts=index, dts=index, duration=1, flags="K_" if index == 0 else "__")
            for index in range(8)
        ]
        with self.assertRaisesRegex(ValueError, "B-frame reorder is unproven"):
            MODULE.analyze_packets(packets, "1/90000", 3)

    def test_rejects_non_increasing_dts(self) -> None:
        packets = [
            MODULE.Packet(pts=0, dts=-2, duration=1, flags="K_"),
            MODULE.Packet(pts=3, dts=-1, duration=1, flags="__"),
            MODULE.Packet(pts=1, dts=0, duration=1, flags="__"),
            MODULE.Packet(pts=2, dts=0, duration=1, flags="__"),
            MODULE.Packet(pts=6, dts=2, duration=1, flags="__"),
            MODULE.Packet(pts=4, dts=3, duration=1, flags="__"),
            MODULE.Packet(pts=5, dts=4, duration=1, flags="__"),
            MODULE.Packet(pts=7, dts=5, duration=1, flags="__"),
        ]
        with self.assertRaisesRegex(ValueError, "non-increasing DTS"):
            MODULE.analyze_packets(packets, "1/1", 3)

    def test_rejects_negative_composition_offsets(self) -> None:
        packets = [
            MODULE.Packet(pts=0, dts=-2, duration=1, flags="K_"),
            MODULE.Packet(pts=3, dts=-1, duration=1, flags="__"),
            MODULE.Packet(pts=1, dts=0, duration=1, flags="__"),
            MODULE.Packet(pts=2, dts=1, duration=1, flags="__"),
            MODULE.Packet(pts=6, dts=2, duration=1, flags="__"),
            MODULE.Packet(pts=4, dts=3, duration=1, flags="__"),
            MODULE.Packet(pts=5, dts=6, duration=1, flags="__"),
            MODULE.Packet(pts=7, dts=7, duration=1, flags="__"),
        ]
        with self.assertRaisesRegex(ValueError, "negative PTS-DTS"):
            MODULE.analyze_packets(packets, "1/1", 3)


class UtilityTest(unittest.TestCase):
    def test_histogram_is_canonical(self) -> None:
        self.assertEqual(
            MODULE.histogram([2, 1, 2]),
            [
                {"value_micros": 1, "count": 1},
                {"value_micros": 2, "count": 2},
            ],
        )

    def test_time_base_rounding(self) -> None:
        time_base = MODULE.parse_time_base("1/90000")
        self.assertEqual(MODULE.ticks_to_micros(3, time_base), 33)
        self.assertEqual(MODULE.ticks_to_micros(-3, time_base), -33)


if __name__ == "__main__":
    unittest.main()
