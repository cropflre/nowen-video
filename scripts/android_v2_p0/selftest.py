from __future__ import annotations

import argparse
import json
import pathlib
import tempfile

from .common import CASES_PATH, PACKAGE_ID, LEGACY_PACKAGE_ID, PRODUCT, load_json, sha256_file, verify_candidate, write_json
from .report import matrix
from .session import finalize, now, prepare, record


def _fixture(directory: pathlib.Path, version: str, commit: str, cert: str) -> None:
    directory.mkdir(parents=True)
    artifacts = []
    for kind in ("apk", "aab"):
        path = directory / f"nowen-video-android-v2-{version}.{kind}"
        path.write_bytes(f"fixture-{kind}".encode())
        artifacts.append({"type": kind, "name": path.name, "size_bytes": path.stat().st_size, "sha256": sha256_file(path)})
    manifest = {
        "schema_version": 1, "product": PRODUCT, "channel": "rc",
        "version": {"name": version, "code": 100501},
        "application": {"id": PACKAGE_ID, "min_sdk": 26, "target_sdk": 35},
        "source": {"repository": "cropflre/nowen-video", "commit": commit, "ref": "refs/heads/main"},
        "workflow": {"event": "workflow_dispatch", "run_id": "123", "run_attempt": "1"},
        "signing": {"certificate_sha256": cert}, "artifacts": artifacts,
    }
    verification = {
        "schema_version": 1, "product": PRODUCT, "repository": "cropflre/nowen-video",
        "version": {"name": version, "code": 100501}, "source_commit": commit,
        "certificate_sha256": cert, "checksums_verified": True, "metadata_verified": True,
        "apk_signature_verified": True, "aab_signature_verified": True, "sensitive_values_included": False,
    }
    write_json(directory / "release-manifest.json", manifest)
    write_json(directory / "candidate-verification.json", verification)
    (directory / "SHA256SUMS.txt").write_text("\n".join(f"{item['sha256']}  {item['name']}" for item in artifacts) + "\n", encoding="utf-8")
    (directory / "RELEASE_NOTES.md").write_text(f"# {version}\nCommit: {commit}\nCertificate: {cert}\n", encoding="utf-8")
    (directory / "workflow-run-id.txt").write_text("123\n", encoding="utf-8")


def _fake_adb(path: pathlib.Path, version: str) -> None:
    source = f"""#!/usr/bin/env bash
set -euo pipefail
if [[ "${{1:-}}" == "-s" ]]; then shift 2; fi
joined="$*"
case "$joined" in
  "devices -l") printf 'List of devices attached\\nFAKE123 device product:fake model:Nowen_Test device:fake transport_id:1\\n' ;;
  "shell getprop ro.product.manufacturer") echo Nowen ;;
  "shell getprop ro.product.model") echo 'Test Device' ;;
  "shell getprop ro.product.device") echo fake ;;
  "shell getprop ro.build.version.release") echo 13 ;;
  "shell getprop ro.build.version.sdk") echo 33 ;;
  "shell getprop ro.product.cpu.abi") echo arm64-v8a ;;
  "shell getprop ro.build.fingerprint") echo 'nowen/fake/fake:13/test:user/release-keys' ;;
  "shell getprop persist.sys.locale"|"shell getprop ro.product.locale") echo zh-CN ;;
  "shell wm size") echo 'Physical size: 1080x2400' ;;
  "shell wm density") echo 'Physical density: 420' ;;
  "shell df -k /data") printf 'Filesystem 1K-blocks Used Available Use%% Mounted on\\n/dev/fake 1000000 1000 999000 1%% /data\\n' ;;
  "shell dumpsys battery") echo 'level: 88' ;;
  "shell pm list packages {LEGACY_PACKAGE_ID}") echo 'package:{LEGACY_PACKAGE_ID}' ;;
  "shell pm list packages {PACKAGE_ID}") echo 'package:{PACKAGE_ID}' ;;
  install*|uninstall*) echo Success ;;
  "shell dumpsys package {PACKAGE_ID}") printf 'Package [{PACKAGE_ID}]\\nversionCode=100501 minSdk=26 targetSdk=35\\nversionName={version}\\n' ;;
  "logcat -c"|"shell am force-stop {PACKAGE_ID}") : ;;
  "shell monkey"*) echo 'Events injected: 1' ;;
  "shell pidof {PACKAGE_ID}") echo 1234 ;;
  "shell dumpsys activity activities") echo 'topResumedActivity=ActivityRecord{{fake {PACKAGE_ID}/.MainActivity}}' ;;
  "shell dumpsys window windows") echo 'mCurrentFocus=Window{{fake {PACKAGE_ID}/.MainActivity}}' ;;
  "exec-out screencap -p") printf FAKEPNG ;;
  "shell uiautomator dump /sdcard/nowen-video-p0-window.xml") echo 'UI hierarchy dumped' ;;
  "exec-out cat /sdcard/nowen-video-p0-window.xml") printf '<hierarchy></hierarchy>' ;;
  "logcat -d -v threadtime"|"logcat -d --pid 1234 -v threadtime") echo 'I/Nowen: app started successfully' ;;
  *) echo "unsupported fake adb: $joined" >&2; exit 2 ;;
esac
"""
    path.write_text(source, encoding="utf-8")
    path.chmod(0o755)


def run() -> None:
    with tempfile.TemporaryDirectory(prefix="android-v2-p0-") as raw:
        root = pathlib.Path(raw)
        candidate, session = root / "candidate", root / "session"
        version, commit, cert = "0.1.0-rc.1", "c" * 40, "ab" * 32
        _fixture(candidate, version, commit, cert)
        adb = root / "adb"
        _fake_adb(adb, version)
        prepare(argparse.Namespace(
            candidate_dir=str(candidate), session_dir=str(session), version=version,
            expected_commit=commit, expected_fingerprint=cert, adb=str(adb), serial=None,
            install_mode="replace", launch_wait=0.0, tester="CI self-test",
            server_version="fixture-server", network_profile="LAN", scope="startup",
        ))
        payload = load_json(session / "p0-session.json")
        assert isinstance(payload, dict)
        results = [f"{item['id']}=PASS" for item in payload["manual_cases"] if item["status"] == "PENDING"]
        record(argparse.Namespace(session_dir=str(session), result=results, note=None))
        if finalize(argparse.Namespace(session_dir=str(session))) != 0:
            raise SystemExit("self-test expected a PASS startup session")

        base = load_json(session / "p0-session.json")
        catalog = load_json(CASES_PATH)
        assert isinstance(base, dict) and isinstance(catalog, list)
        session26, session35 = root / "session-api26-full", root / "session-api35-startup"
        for target, api, scope in ((session26, 26, "full"), (session35, 35, "startup")):
            target.mkdir()
            clone = json.loads(json.dumps(base))
            clone["session_id"], clone["scope"], clone["device"]["api_level"] = f"self-test-{api}-{scope}", scope, api
            clone["required_case_ids"] = [item["id"] for item in catalog if scope == "full" or bool(item.get("startup"))]
            required = set(clone["required_case_ids"])
            for case in clone["manual_cases"]:
                case["status"] = "PASS" if case["id"] in required else "N/A"
                case["note"] = "" if case["id"] in required else "not included in startup-only scope"
            clone["final"] = {"verdict": "PASS", "finalized_at_utc": now(), "blocking_items": [], "meaning": "self-test"}
            write_json(target / "p0-session.json", clone)
        if matrix(argparse.Namespace(session_dir=[str(session26), str(session), str(session35)], output_dir=str(root / "matrix"))) != 0:
            raise SystemExit("self-test expected API 26/33/35 matrix PASS")

        broken = load_json(candidate / "release-manifest.json")
        assert isinstance(broken, dict)
        broken["source"]["commit"] = "d" * 40
        write_json(candidate / "release-manifest.json", broken)
        try:
            verify_candidate(candidate, version, commit, cert)
        except SystemExit:
            pass
        else:
            raise SystemExit("self-test expected wrong candidate commit to be rejected")
    print("Android V2 device P0 helper self-test passed")
