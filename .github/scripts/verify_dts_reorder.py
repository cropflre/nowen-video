#!/usr/bin/env python3
"""Certify explicit encoder time-base candidates with B-frame DTS reordering.

The report is evidence only. It never authorizes seamless handoff and keeps
EXT-X-DISCONTINUITY mandatory until real media, hardware and client gates pass.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import tempfile
from collections import Counter
from dataclasses import asdict, dataclass
from fractions import Fraction
from pathlib import Path
from typing import Any, Iterable, Sequence

SCHEMA_VERSION = "encoder-time-base-dts-reorder-evidence-v1"
REPEAT_COUNT = 3
BOUNDARY_MICROS = 8_000_000
DURATION_MICROS = 12_000_000
AUDIO_SAMPLE_RATE = 48_000


@dataclass(frozen=True)
class CaseSpec:
    id: str
    description: str
    rate_numerator: int
    rate_denominator: int
    gop_size: int
    max_b_frames: int = 3

    @property
    def rate_expression(self) -> str:
        if self.rate_denominator == 1:
            return str(self.rate_numerator)
        return f"{self.rate_numerator}/{self.rate_denominator}"


@dataclass(frozen=True)
class CandidateSpec:
    id: str
    description: str
    encoder_time_base: str


@dataclass(frozen=True)
class Packet:
    pts: int
    dts: int
    duration: int
    flags: str


CASES: tuple[CaseSpec, ...] = (
    CaseSpec(
        "dts-cfr-24000-1001-b3-v1",
        "24000/1001 fps CFR with deterministic three-frame B reorder",
        24_000,
        1_001,
        48,
    ),
    CaseSpec(
        "dts-cfr-30000-1001-b3-v1",
        "30000/1001 fps CFR with deterministic three-frame B reorder",
        30_000,
        1_001,
        60,
    ),
    CaseSpec(
        "dts-cfr-60-b3-v1",
        "60 fps CFR with deterministic three-frame B reorder",
        60,
        1,
        120,
    ),
)

CANDIDATES: tuple[CandidateSpec, ...] = (
    CandidateSpec(
        "encoder-time-base-avtb-v1",
        "Explicit AVTB encoder time base",
        "1/1000000",
    ),
    CandidateSpec(
        "encoder-time-base-90k-v1",
        "Explicit MPEG-TS 90 kHz encoder time base",
        "1/90000",
    ),
)


def run_checked(command: Sequence[str]) -> str:
    process = subprocess.run(
        list(command),
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    if process.returncode != 0:
        rendered = " ".join(command)
        raise RuntimeError(f"command failed ({process.returncode}): {rendered}\n{process.stdout}")
    return process.stdout


def tool_version(path: str) -> str:
    output = run_checked((path, "-version"))
    first = output.splitlines()[0].strip() if output else ""
    if not first:
        raise RuntimeError(f"unable to identify tool version: {path}")
    return first


def flexible_int(value: Any, field: str) -> int:
    if value is None or value == "N/A" or value == "":
        raise ValueError(f"packet {field} is unavailable")
    try:
        return int(value)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"packet {field} is invalid: {value!r}") from exc


def optional_int(value: Any) -> int:
    if value is None or value == "N/A" or value == "":
        return 0
    try:
        return int(value)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"packet integer is invalid: {value!r}") from exc


def parse_time_base(value: str) -> Fraction:
    try:
        numerator, denominator = value.split("/", 1)
        result = Fraction(int(numerator), int(denominator))
    except (ValueError, ZeroDivisionError) as exc:
        raise ValueError(f"invalid time base {value!r}") from exc
    if result <= 0:
        raise ValueError(f"time base must be positive: {value!r}")
    return result


def ticks_to_micros(value: int, time_base: Fraction) -> int:
    scaled = Fraction(value) * time_base * 1_000_000
    if scaled >= 0:
        return int(scaled + Fraction(1, 2))
    return int(scaled - Fraction(1, 2))


def sha256_json(value: Any) -> str:
    payload = json.dumps(value, separators=(",", ":"), sort_keys=True).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def histogram(values: Iterable[int]) -> list[dict[str, int]]:
    counts = Counter(values)
    return [
        {"value_micros": value, "count": counts[value]}
        for value in sorted(counts)
    ]


def duplicate_count(values: Sequence[int]) -> int:
    return len(values) - len(set(values))


def non_increasing_count(values: Sequence[int]) -> int:
    return sum(1 for left, right in zip(values, values[1:]) if right <= left)


def presentation_backstep_count(values: Sequence[int]) -> int:
    return sum(1 for left, right in zip(values, values[1:]) if right < left)


def delta_values(values: Sequence[int]) -> list[int]:
    return [right - left for left, right in zip(values, values[1:])]


def analyze_packets(packets: Sequence[Packet], time_base_text: str, max_b_frames: int) -> dict[str, Any]:
    if len(packets) < 8:
        raise ValueError(f"video packet set is too small: {len(packets)}")
    if max_b_frames <= 0:
        raise ValueError("max_b_frames must be positive")

    time_base = parse_time_base(time_base_text)
    pts_ticks = [packet.pts for packet in packets]
    dts_ticks = [packet.dts for packet in packets]
    duration_ticks = [packet.duration for packet in packets]
    pts_micros = [ticks_to_micros(value, time_base) for value in pts_ticks]
    dts_micros = [ticks_to_micros(value, time_base) for value in dts_ticks]
    duration_micros = [ticks_to_micros(value, time_base) for value in duration_ticks if value > 0]

    pts_duplicates = duplicate_count(pts_ticks)
    dts_duplicates = duplicate_count(dts_ticks)
    dts_non_increasing = non_increasing_count(dts_ticks)
    pts_backsteps = presentation_backstep_count(pts_ticks)
    composition_offsets = [pts - dts for pts, dts in zip(pts_micros, dts_micros)]
    negative_composition_offsets = sum(1 for value in composition_offsets if value < 0)
    positive_composition_offsets = sum(1 for value in composition_offsets if value > 0)

    presentation_order = sorted(range(len(packets)), key=lambda index: (pts_ticks[index], index))
    presentation_rank = [0] * len(packets)
    for rank, decode_index in enumerate(presentation_order):
        presentation_rank[decode_index] = rank
    decode_to_presentation_displacement = [
        rank - decode_index for decode_index, rank in enumerate(presentation_rank)
    ]
    maximum_forward_reorder = max(decode_to_presentation_displacement)
    maximum_backward_reorder = abs(min(decode_to_presentation_displacement))

    sorted_pts_micros = sorted(pts_micros)
    normalized_presentation = [value - sorted_pts_micros[0] for value in sorted_pts_micros]
    normalized_decode_dts = [value - dts_micros[0] for value in dts_micros]
    presentation_deltas = delta_values(sorted_pts_micros)
    decode_dts_deltas = delta_values(dts_micros)

    evidence = {
        "time_base": time_base_text,
        "packet_count": len(packets),
        "key_packet_count": sum(1 for packet in packets if "K" in packet.flags),
        "first_pts_ticks": pts_ticks[0],
        "first_dts_ticks": dts_ticks[0],
        "last_pts_ticks": pts_ticks[-1],
        "last_dts_ticks": dts_ticks[-1],
        "first_pts_micros": pts_micros[0],
        "first_dts_micros": dts_micros[0],
        "last_presentation_pts_micros": sorted_pts_micros[-1],
        "last_decode_dts_micros": dts_micros[-1],
        "duplicate_pts_count": pts_duplicates,
        "duplicate_dts_count": dts_duplicates,
        "non_increasing_dts_count": dts_non_increasing,
        "presentation_backstep_count": pts_backsteps,
        "positive_composition_offset_count": positive_composition_offsets,
        "negative_composition_offset_count": negative_composition_offsets,
        "minimum_composition_offset_micros": min(composition_offsets),
        "maximum_composition_offset_micros": max(composition_offsets),
        "maximum_forward_reorder_packets": maximum_forward_reorder,
        "maximum_backward_reorder_packets": maximum_backward_reorder,
        "presentation_delta_histogram": histogram(presentation_deltas),
        "decode_dts_delta_histogram": histogram(decode_dts_deltas),
        "composition_offset_histogram": histogram(composition_offsets),
        "duration_histogram": histogram(duration_micros),
        "presentation_sequence_sha256": sha256_json(normalized_presentation),
        "decode_dts_sequence_sha256": sha256_json(normalized_decode_dts),
        "decode_to_presentation_rank_sha256": sha256_json(presentation_rank),
        "composition_offset_sequence_sha256": sha256_json(composition_offsets),
    }

    errors: list[str] = []
    if pts_duplicates:
        errors.append(f"duplicate PTS packets: {pts_duplicates}")
    if dts_duplicates:
        errors.append(f"duplicate DTS packets: {dts_duplicates}")
    if dts_non_increasing:
        errors.append(f"non-increasing DTS transitions: {dts_non_increasing}")
    if pts_backsteps == 0:
        errors.append("no presentation-order backstep was observed; B-frame reorder is unproven")
    if positive_composition_offsets == 0:
        errors.append("no positive PTS-DTS composition offset was observed")
    if negative_composition_offsets:
        errors.append(f"negative PTS-DTS composition offsets: {negative_composition_offsets}")
    if maximum_forward_reorder <= 0 or maximum_backward_reorder <= 0:
        errors.append("decode/presentation order displacement is absent")
    if maximum_forward_reorder > max_b_frames + 1 or maximum_backward_reorder > max_b_frames + 1:
        errors.append(
            "decode/presentation displacement exceeds configured B-frame bound: "
            f"forward={maximum_forward_reorder} backward={maximum_backward_reorder} max_b_frames={max_b_frames}"
        )
    if any(delta <= 0 for delta in presentation_deltas):
        errors.append("presentation timeline is not strictly increasing after PTS sort")
    if any(delta <= 0 for delta in decode_dts_deltas):
        errors.append("decode timeline is not strictly increasing")
    if errors:
        raise ValueError("; ".join(errors))
    return evidence


def filter_graph(case: CaseSpec, window: str) -> str:
    if window == "startup":
        start_seconds = 0
        end_seconds = BOUNDARY_MICROS / 1_000_000
        offset_seconds = 0
    elif window == "continuation":
        start_seconds = BOUNDARY_MICROS / 1_000_000
        end_seconds = DURATION_MICROS / 1_000_000
        offset_seconds = start_seconds
    else:
        raise ValueError(f"unsupported window {window!r}")

    return (
        f"testsrc2=size=320x180:rate={case.rate_expression}:duration={DURATION_MICROS / 1_000_000:g},"
        f"trim=start={start_seconds:g}:end={end_seconds:g},settb=AVTB,"
        f"setpts=PTS-STARTPTS+{offset_seconds:g}/TB[out0];"
        f"sine=frequency=1000:sample_rate={AUDIO_SAMPLE_RATE}:duration={DURATION_MICROS / 1_000_000:g},"
        f"atrim=start={start_seconds:g}:end={end_seconds:g},asettb=1/{AUDIO_SAMPLE_RATE},"
        f"asetpts=PTS-STARTPTS+{offset_seconds:g}/TB[out1]"
    )


def build_ffmpeg_command(
    ffmpeg: str,
    case: CaseSpec,
    candidate: CandidateSpec,
    window: str,
    output_path: Path,
) -> list[str]:
    x264_params = (
        f"bframes={case.max_b_frames}:b-adapt=0:scenecut=0:open-gop=0:"
        f"keyint={case.gop_size}:min-keyint={case.gop_size}"
    )
    return [
        ffmpeg,
        "-hide_banner",
        "-loglevel",
        "error",
        "-y",
        "-copyts",
        "-f",
        "lavfi",
        "-i",
        filter_graph(case, window),
        "-map",
        "0:v:0",
        "-map",
        "0:a:0",
        "-threads",
        "1",
        "-c:v",
        "libx264",
        "-preset",
        "medium",
        "-crf",
        "23",
        "-pix_fmt",
        "yuv420p",
        "-g",
        str(case.gop_size),
        "-keyint_min",
        str(case.gop_size),
        "-sc_threshold",
        "0",
        "-bf",
        str(case.max_b_frames),
        "-b_strategy",
        "0",
        "-x264-params",
        x264_params,
        "-fps_mode",
        "passthrough",
        "-enc_time_base:v:0",
        candidate.encoder_time_base,
        "-c:a",
        "aac",
        "-b:a",
        "128k",
        "-ar",
        str(AUDIO_SAMPLE_RATE),
        "-ac",
        "2",
        "-avoid_negative_ts",
        "disabled",
        "-muxdelay",
        "0",
        "-muxpreload",
        "0",
        "-f",
        "mpegts",
        str(output_path),
    ]


def probe_packets(ffprobe: str, media_path: Path) -> tuple[str, list[Packet]]:
    output = run_checked(
        (
            ffprobe,
            "-v",
            "error",
            "-print_format",
            "json",
            "-show_streams",
            "-show_packets",
            "-show_entries",
            "stream=index,codec_type,time_base:packet=stream_index,pts,dts,duration,flags",
            str(media_path),
        )
    )
    document = json.loads(output)
    streams = document.get("streams") or []
    video_stream = next((stream for stream in streams if stream.get("codec_type") == "video"), None)
    if video_stream is None:
        raise ValueError(f"no video stream in {media_path}")
    stream_index = flexible_int(video_stream.get("index"), "stream index")
    time_base = str(video_stream.get("time_base") or "")
    parse_time_base(time_base)

    packets: list[Packet] = []
    for raw in document.get("packets") or []:
        if flexible_int(raw.get("stream_index"), "stream_index") != stream_index:
            continue
        packets.append(
            Packet(
                pts=flexible_int(raw.get("pts"), "pts"),
                dts=flexible_int(raw.get("dts"), "dts"),
                duration=optional_int(raw.get("duration")),
                flags=str(raw.get("flags") or ""),
            )
        )
    return time_base, packets


def stable_projection(window: dict[str, Any]) -> dict[str, Any]:
    excluded = {
        "first_pts_ticks",
        "first_dts_ticks",
        "last_pts_ticks",
        "last_dts_ticks",
        "first_pts_micros",
        "first_dts_micros",
        "last_presentation_pts_micros",
        "last_decode_dts_micros",
    }
    return {key: value for key, value in window.items() if key not in excluded}


def validate_repeats(runs: Sequence[dict[str, Any]], candidate_id: str, case_id: str) -> dict[str, Any]:
    if len(runs) != REPEAT_COUNT:
        raise ValueError(f"{case_id}/{candidate_id} has {len(runs)} repeats, want {REPEAT_COUNT}")
    startup_reference = stable_projection(runs[0]["startup"])
    continuation_reference = stable_projection(runs[0]["continuation"])
    for run in runs[1:]:
        if stable_projection(run["startup"]) != startup_reference:
            raise ValueError(f"startup DTS evidence is not repeat-stable for {case_id}/{candidate_id}")
        if stable_projection(run["continuation"]) != continuation_reference:
            raise ValueError(f"continuation DTS evidence is not repeat-stable for {case_id}/{candidate_id}")
    boundary_offsets = [run["boundary_start_delta_micros"] for run in runs]
    if max(boundary_offsets) - min(boundary_offsets) > 1:
        raise ValueError(f"boundary start delta variance exceeds one microsecond for {case_id}/{candidate_id}")
    return {
        "repeat_count": len(runs),
        "startup_stable": True,
        "continuation_stable": True,
        "boundary_start_delta_micros_min": min(boundary_offsets),
        "boundary_start_delta_micros_max": max(boundary_offsets),
        "boundary_start_delta_micros_span": max(boundary_offsets) - min(boundary_offsets),
        "stable": True,
    }


def candidate_projection(candidate: dict[str, Any]) -> dict[str, Any]:
    first = candidate["runs"][0]
    return {
        "startup": stable_projection(first["startup"]),
        "continuation": stable_projection(first["continuation"]),
        "boundary_start_delta_micros": first["boundary_start_delta_micros"],
    }


def run_matrix(ffmpeg: str, ffprobe: str, work_dir: Path) -> dict[str, Any]:
    cases: list[dict[str, Any]] = []
    for case in CASES:
        candidate_reports: list[dict[str, Any]] = []
        for candidate in CANDIDATES:
            runs: list[dict[str, Any]] = []
            for ordinal in range(1, REPEAT_COUNT + 1):
                run_dir = work_dir / case.id / candidate.id / f"run-{ordinal:02d}"
                run_dir.mkdir(parents=True, exist_ok=True)
                windows: dict[str, dict[str, Any]] = {}
                command_hashes: dict[str, str] = {}
                for window in ("startup", "continuation"):
                    output_path = run_dir / f"{window}.ts"
                    command = build_ffmpeg_command(ffmpeg, case, candidate, window, output_path)
                    run_checked(command)
                    time_base, packets = probe_packets(ffprobe, output_path)
                    windows[window] = analyze_packets(packets, time_base, case.max_b_frames)
                    normalized_command = ["<work-dir>" if value.startswith(str(work_dir)) else value for value in command]
                    command_hashes[window] = sha256_json(normalized_command)

                boundary_start_delta = (
                    windows["continuation"]["first_pts_micros"]
                    - windows["startup"]["first_pts_micros"]
                    - BOUNDARY_MICROS
                )
                frame_tolerance = round(1_000_000 * case.rate_denominator / case.rate_numerator)
                if abs(boundary_start_delta) > frame_tolerance:
                    raise ValueError(
                        f"boundary start delta exceeds one frame for {case.id}/{candidate.id}/run-{ordinal:02d}: "
                        f"delta={boundary_start_delta} tolerance={frame_tolerance}"
                    )
                runs.append(
                    {
                        "ordinal": ordinal,
                        "startup_command_sha256": command_hashes["startup"],
                        "continuation_command_sha256": command_hashes["continuation"],
                        "startup": windows["startup"],
                        "continuation": windows["continuation"],
                        "boundary_start_delta_micros": boundary_start_delta,
                    }
                )
            candidate_report = {
                "candidate": asdict(candidate),
                "runs": runs,
                "summary": validate_repeats(runs, candidate.id, case.id),
            }
            candidate_reports.append(candidate_report)

        left_projection = candidate_projection(candidate_reports[0])
        right_projection = candidate_projection(candidate_reports[1])
        equivalent = left_projection == right_projection
        if not equivalent:
            raise ValueError(f"encoder time-base candidates diverged under DTS reorder for {case.id}")
        cases.append(
            {
                "case": asdict(case),
                "candidates": candidate_reports,
                "comparison": {
                    "candidate_a_id": CANDIDATES[0].id,
                    "candidate_b_id": CANDIDATES[1].id,
                    "presentation_sequence_equivalent": True,
                    "decode_order_equivalent": True,
                    "composition_offsets_equivalent": True,
                    "boundary_start_equivalent": True,
                    "equivalent": True,
                },
            }
        )

    report = {
        "schema_version": SCHEMA_VERSION,
        "ffmpeg_version": tool_version(ffmpeg),
        "ffprobe_version": tool_version(ffprobe),
        "repeat_count": REPEAT_COUNT,
        "boundary_micros": BOUNDARY_MICROS,
        "duration_micros": DURATION_MICROS,
        "cases": cases,
        "all_cases_stable": True,
        "all_candidates_equivalent": True,
        "seamless_allowed": False,
        "discontinuity_required": True,
    }
    report["evidence_sha256"] = sha256_json(report)
    return report


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ffmpeg", default="ffmpeg")
    parser.add_argument("--ffprobe", default="ffprobe")
    parser.add_argument("--output", default="dts-reorder-evidence-v1.json")
    parser.add_argument("--work-dir")
    parser.add_argument("--keep-work-dir", action="store_true")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    ffmpeg = shutil.which(args.ffmpeg) if not Path(args.ffmpeg).is_file() else args.ffmpeg
    ffprobe = shutil.which(args.ffprobe) if not Path(args.ffprobe).is_file() else args.ffprobe
    if not ffmpeg or not ffprobe:
        raise SystemExit("ffmpeg and ffprobe are required")

    temporary: tempfile.TemporaryDirectory[str] | None = None
    if args.work_dir:
        work_dir = Path(args.work_dir).resolve()
        work_dir.mkdir(parents=True, exist_ok=True)
    elif args.keep_work_dir:
        work_dir = Path(tempfile.mkdtemp(prefix="nowen-dts-reorder-"))
        print(json.dumps({"kept_work_dir": str(work_dir)}, sort_keys=True))
    else:
        temporary = tempfile.TemporaryDirectory(prefix="nowen-dts-reorder-")
        work_dir = Path(temporary.name)

    try:
        report = run_matrix(ffmpeg, ffprobe, work_dir)
        output_path = Path(args.output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        print(
            json.dumps(
                {
                    "schema_version": report["schema_version"],
                    "case_count": len(report["cases"]),
                    "execution_count": len(report["cases"]) * len(CANDIDATES) * REPEAT_COUNT * 2,
                    "all_cases_stable": report["all_cases_stable"],
                    "all_candidates_equivalent": report["all_candidates_equivalent"],
                    "discontinuity_required": report["discontinuity_required"],
                    "output": str(output_path),
                },
                sort_keys=True,
            )
        )
    finally:
        if temporary is not None:
            temporary.cleanup()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
