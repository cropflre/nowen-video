from __future__ import annotations

import argparse
import os

from .integrity import validate_session_file
from .report import matrix, post
from .selftest import run as self_test
from .session import finalize, prepare, record


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description="Android V2 RC physical-device P0 session and matrix helper")
    sub = root.add_subparsers(dest="command", required=True)
    item = sub.add_parser("prepare", help="verify/install/launch a candidate and create a device session")
    item.add_argument("--candidate-dir", required=True)
    item.add_argument("--session-dir", required=True)
    item.add_argument("--version", default="0.1.0-rc.1")
    item.add_argument("--expected-commit", required=True)
    item.add_argument("--expected-fingerprint", required=True)
    item.add_argument("--adb", default=os.environ.get("ADB", "adb"))
    item.add_argument("--serial")
    item.add_argument("--install-mode", choices=("none", "replace", "fresh"), default="replace")
    item.add_argument("--launch-wait", type=float, default=3.0)
    item.add_argument("--tester", required=True)
    item.add_argument("--server-version", required=True)
    item.add_argument("--network-profile", choices=("LAN", "EXTERNAL", "MOBILE", "MIXED"), required=True)
    item.add_argument("--scope", choices=("full", "startup"), default="full")
    item = sub.add_parser("record", help="record manual case results")
    item.add_argument("--session-dir", required=True)
    item.add_argument("--result", action="append", help="CASE_ID=PASS|FAIL|BLOCKED|N/A")
    item.add_argument("--note", action="append", help="CASE_ID=public-safe note")
    item = sub.add_parser("finalize", help="compute a fail-closed single-device verdict")
    item.add_argument("--session-dir", required=True)
    item = sub.add_parser("matrix", help="aggregate finalized sessions for exact API 26/33/35 coverage")
    item.add_argument("--session-dir", action="append", required=True)
    item.add_argument("--output-dir", required=True)
    item = sub.add_parser("post", help="explicitly post a finalized redacted report")
    item.add_argument("--report", required=True)
    item.add_argument("--repository", default="cropflre/nowen-video")
    item.add_argument("--issue", type=int, default=55)
    sub.add_parser("self-test", help="run offline candidate, fake-ADB, record, finalize and matrix tests")
    return root


def main() -> int:
    args = parser().parse_args()
    if args.command == "prepare":
        prepare(args)
        return 0
    if args.command == "record":
        validate_session_file(args.session_dir)
        record(args)
        return 0
    if args.command == "finalize":
        validate_session_file(args.session_dir)
        return finalize(args)
    if args.command == "matrix":
        for directory in args.session_dir:
            validate_session_file(directory)
        return matrix(args)
    if args.command == "post":
        post(args)
        return 0
    if args.command == "self-test":
        self_test()
        return 0
    raise SystemExit(f"unsupported command: {args.command}")
