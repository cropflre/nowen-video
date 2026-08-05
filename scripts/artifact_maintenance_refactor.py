#!/usr/bin/env python3
from __future__ import annotations

import argparse
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]

SYMBOLS = (
    "TranscodeService",
    "ArtifactMaintenanceService",
    "TranscodeJob",
    "transcodePriorityQueue",
    "recoverPendingTasks",
    "leaseRecoveryLoop",
    "StartTranscode",
    "NewTranscodeService",
    "NewArtifactMaintenanceService",
    "ExecutionRuntime",
    "GetHWAccelInfo",
)


def go_files() -> list[pathlib.Path]:
    return [
        path
        for path in ROOT.rglob("*.go")
        if "vendor" not in path.parts and ".git" not in path.parts
    ]


def audit() -> None:
    files = sorted(go_files())
    print("=== artifact maintenance refactor audit ===")
    for symbol in SYMBOLS:
        print(f"\n--- {symbol} ---")
        matches = 0
        for path in files:
            for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
                if symbol in line:
                    print(f"{path.relative_to(ROOT)}:{lineno}:{line}")
                    matches += 1
        if matches == 0:
            print("<no matches>")
    print("\n--- transcode service files ---")
    for path in sorted((ROOT / "internal/service").glob("transcode*.go")):
        text = path.read_text(encoding="utf-8")
        receivers = sorted(set(re.findall(r"func \(s \*(\w+)\)", text)))
        symbols = [name for name in SYMBOLS if name in text]
        print(f"{path.relative_to(ROOT)} size={path.stat().st_size} receivers={receivers} symbols={symbols}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--audit", action="store_true")
    args = parser.parse_args()
    if args.audit:
        audit()
        return 0
    print("refactor mode is not implemented yet", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
