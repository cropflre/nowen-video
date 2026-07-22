from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import pathlib
import re
import time
import uuid
from typing import Any

from .common import (
    CASES_PATH, LEGACY_PACKAGE_ID, MIN_SDK, PACKAGE_ID, PRODUCT, STATUSES,
    Adb, fail, fingerprint, load_json, safe_text, select_device, sha256_file,
    verify_candidate, write_json,
)


def now() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()


def _prop(adb: Adb, key: str) -> str:
    return adb.shell("getprop", key)


def _installed(adb: Adb, package: str) -> bool:
    return f"package:{package}" in adb.shell("pm", "list", "packages", package).splitlines()


def _version(text: str) -> tuple[str | None, int | None]:
    name, code = re.search(r"\bversionName=([^\s]+)", text), re.search(r"\bversionCode=(\d+)", text)
    return (name.group(1) if name else None, int(code.group(1)) if code else None)


def _check(check_id: str, title: str, status: str, detail: str) -> dict[str, str]:
    return {"id": check_id, "title": title, "status": status, "detail": detail.strip()}


def _capture(path: pathlib.Path, text: str) -> None:
    path.write_text(text.rstrip() + "\n", encoding="utf-8")


def _device_info(adb: Adb, metadata: dict[str, str]) -> dict[str, Any]:
    storage, battery = adb.shell("df", "-k", "/data"), adb.shell("dumpsys", "battery")
    return {
        "serial_sha256_prefix": hashlib.sha256((adb.serial or "").encode()).hexdigest()[:12],
        "manufacturer": _prop(adb, "ro.product.manufacturer"),
        "model": _prop(adb, "ro.product.model") or metadata.get("model", ""),
        "device": _prop(adb, "ro.product.device") or metadata.get("device", ""),
        "android_version": _prop(adb, "ro.build.version.release"),
        "api_level": int(_prop(adb, "ro.build.version.sdk") or -1),
        "abi": _prop(adb, "ro.product.cpu.abi"),
        "build_fingerprint": _prop(adb, "ro.build.fingerprint"),
        "locale": _prop(adb, "persist.sys.locale") or _prop(adb, "ro.product.locale"),
        "screen_size": adb.shell("wm", "size").replace("Physical size:", "").strip(),
        "density": adb.shell("wm", "density").replace("Physical density:", "").strip(),
        "storage_summary": " | ".join(line.strip() for line in storage.splitlines()[-2:]),
        "battery_level": next((line.split(":", 1)[1].strip() for line in battery.splitlines() if line.strip().startswith("level:")), "unknown"),
    }


def _crashes(logcat: str) -> list[str]:
    patterns = [re.compile(r"FATAL EXCEPTION", re.I), re.compile(rf"ANR in\s+{re.escape(PACKAGE_ID)}", re.I), re.compile(r"Fatal signal", re.I)]
    lines, result = logcat.splitlines(), []
    for index, line in enumerate(lines):
        if any(pattern.search(line) for pattern in patterns):
            result.append("\n".join(lines[max(0, index - 2):min(len(lines), index + 4)])[:1200])
    return result[:5]


def prepare(args: argparse.Namespace) -> pathlib.Path:
    from .report import render_session

    candidate_dir, session_dir = pathlib.Path(args.candidate_dir).resolve(), pathlib.Path(args.session_dir).resolve()
    if session_dir.exists() and any(session_dir.iterdir()):
        fail(f"session directory is not empty: {session_dir}")
    session_dir.mkdir(parents=True, exist_ok=True)
    evidence = session_dir / "evidence-local-only"
    evidence.mkdir()
    candidate = verify_candidate(candidate_dir, args.version, args.expected_commit, args.expected_fingerprint)
    adb, metadata = select_device(args.adb, args.serial)
    device = _device_info(adb, metadata)
    version_code = int(candidate["manifest"]["version"]["code"])
    checks = [
        _check("AUTO-01", "候选包哈希与 metadata", "PASS", "manifest、SHA256SUMS 和 candidate-verification 一致"),
        _check("AUTO-02", "APK 本地签名核验", "PASS" if candidate["apk_verified"] else "BLOCKED", candidate["apk_detail"]),
        _check("AUTO-03", "AAB 本地签名核验", "PASS" if candidate["aab_verified"] else "BLOCKED", candidate["aab_detail"]),
        _check("AUTO-04", "ADB 设备已授权", "PASS", "one explicitly selected or uniquely connected device is online"),
        _check("AUTO-05", "设备满足最低 API", "PASS" if device["api_level"] >= MIN_SDK else "FAIL", f"API {device['api_level']} (minimum {MIN_SDK})"),
    ]

    legacy_before, v2_before = _installed(adb, LEGACY_PACKAGE_ID), _installed(adb, PACKAGE_ID)
    install_output, install_ok = "installation skipped; existing package inspected", True
    if args.install_mode == "fresh":
        if v2_before:
            result = adb.run("uninstall", PACKAGE_ID, timeout=120)
            if result.returncode:
                fail(f"fresh install could not uninstall existing V2: {(result.stdout or '')}{(result.stderr or '')}")
        result = adb.run("install", str(candidate["artifacts"]["apk"]["path"]), timeout=300)
        install_output = ((result.stdout or "") + (result.stderr or "")).strip()
        install_ok = result.returncode == 0 and "Success" in install_output
    elif args.install_mode == "replace":
        result = adb.run("install", "-r", str(candidate["artifacts"]["apk"]["path"]), timeout=300)
        install_output = ((result.stdout or "") + (result.stderr or "")).strip()
        install_ok = result.returncode == 0 and "Success" in install_output
    else:
        install_ok = _installed(adb, PACKAGE_ID)
    checks.append(_check("AUTO-06", "候选 APK 安装或现有安装确认", "PASS" if install_ok else "FAIL", install_output[-1000:]))

    package_dump = adb.shell("dumpsys", "package", PACKAGE_ID, timeout=90)
    _capture(evidence / "dumpsys-package.txt", package_dump)
    installed_name, installed_code = _version(package_dump)
    version_ok = installed_name == args.version and installed_code == version_code
    checks.append(_check("AUTO-07", "设备已安装版本与候选一致", "PASS" if version_ok else "FAIL", f"installed {installed_name} ({installed_code}); expected {args.version} ({version_code})"))

    adb.run("logcat", "-c")
    adb.shell("am", "force-stop", PACKAGE_ID)
    started = time.monotonic()
    launched = adb.run("shell", "monkey", "-p", PACKAGE_ID, "-c", "android.intent.category.LAUNCHER", "1", timeout=60)
    time.sleep(max(0.0, args.launch_wait))
    elapsed = int((time.monotonic() - started) * 1000)
    pid = adb.shell("pidof", PACKAGE_ID)
    activity, window = adb.shell("dumpsys", "activity", "activities", timeout=90), adb.shell("dumpsys", "window", "windows", timeout=90)
    _capture(evidence / "dumpsys-activity.txt", activity)
    _capture(evidence / "dumpsys-window.txt", window)
    foreground = PACKAGE_ID in activity or PACKAGE_ID in window
    launch_ok = launched.returncode == 0 and bool(pid) and foreground
    checks.append(_check("AUTO-08", "应用可启动并进入前台", "PASS" if launch_ok else "FAIL", f"pid={pid or 'missing'}, foreground={foreground}, elapsed_ms={elapsed}"))

    screenshot = adb.run("exec-out", "screencap", "-p", text=False)
    if screenshot.returncode == 0 and screenshot.stdout:
        (evidence / "launch.png").write_bytes(screenshot.stdout)
    adb.shell("uiautomator", "dump", "/sdcard/nowen-video-p0-window.xml")
    ui = adb.run("exec-out", "cat", "/sdcard/nowen-video-p0-window.xml", text=False)
    if ui.returncode == 0 and ui.stdout:
        (evidence / "window.xml").write_bytes(ui.stdout)
    log_args = ("logcat", "-d", "--pid", pid, "-v", "threadtime") if pid else ("logcat", "-d", "-v", "threadtime")
    logs = adb.run(*log_args, timeout=120)
    logcat = (logs.stdout or "") + (logs.stderr or "")
    _capture(evidence / "launch-logcat.txt", logcat)
    crashes = _crashes(logcat)
    checks.append(_check("AUTO-09", "启动期间未发现崩溃或 ANR", "PASS" if not crashes else "FAIL", "no blocking pattern found" if not crashes else "\n---\n".join(crashes)))
    evidence_files = sorted(path.name for path in evidence.iterdir())
    required_evidence = {"dumpsys-package.txt", "dumpsys-activity.txt", "dumpsys-window.txt", "launch-logcat.txt"}
    checks.append(_check("AUTO-10", "本地证据已采集", "PASS" if required_evidence.issubset(evidence_files) else "BLOCKED", ", ".join(evidence_files)))

    case_defs = load_json(CASES_PATH)
    if not isinstance(case_defs, list):
        fail(f"P0 case catalog must be an array: {CASES_PATH}")
    manual = []
    for item in case_defs:
        relevant = args.scope == "full" or bool(item.get("startup"))
        manual.append({
            "id": item["id"], "section": item["section"], "title": item["title"],
            "status": "PENDING" if relevant else "N/A",
            "note": "" if relevant else "not included in startup-only scope", "updated_at_utc": None,
        })
    payload = {
        "schema_version": 1, "product": PRODUCT,
        "session_id": f"p0-{dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ')}-{uuid.uuid4().hex[:8]}",
        "scope": args.scope, "created_at_utc": now(), "updated_at_utc": now(),
        "candidate": {
            "version_name": args.version, "version_code": version_code,
            "source_commit": args.expected_commit.lower(), "certificate_sha256": fingerprint(args.expected_fingerprint),
            "apk_name": candidate["artifacts"]["apk"]["name"], "apk_sha256": candidate["artifacts"]["apk"]["sha256"],
            "workflow_run_id": candidate["run_id"],
        },
        "environment": {"tester": safe_text(args.tester), "server_version": safe_text(args.server_version), "network_profile": args.network_profile},
        "device": device,
        "installation": {
            "mode": args.install_mode, "legacy_package_installed_before": legacy_before,
            "v2_package_installed_before": v2_before, "success": install_ok,
            "installed_version_name": installed_name, "installed_version_code": installed_code,
        },
        "launch": {"success": launch_ok, "elapsed_ms": elapsed, "pid_observed": bool(pid), "foreground_observed": foreground},
        "case_catalog_sha256": sha256_file(CASES_PATH),
        "required_case_ids": [item["id"] for item in case_defs if args.scope == "full" or bool(item.get("startup"))],
        "automatic_checks": checks, "manual_cases": manual,
        "evidence": {
            "directory": "evidence-local-only", "files": evidence_files, "public_upload_allowed": False,
            "warning": "Raw logcat, screenshot and UI XML may contain private server information. Review and redact before sharing.",
        },
        "final": {
            "verdict": "PENDING", "finalized_at_utc": None, "blocking_items": [],
            "meaning": "A session PASS applies only to this device and scope; it is not the overall RC release decision.",
        },
        "sensitive_values_included": False,
    }
    write_json(session_dir / "p0-session.json", payload)
    render_session(payload, session_dir / "P0_REPORT.md")
    print(f"Prepared P0 session: {session_dir}")
    print(f"Session ID: {payload['session_id']}")
    return session_dir


def _assignment(raw: str, flag: str) -> tuple[str, str]:
    if "=" not in raw:
        fail(f"{flag} must use ID=value syntax: {raw}")
    key, value = raw.split("=", 1)
    return key.strip(), value.strip()


def validate_session(payload: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    catalog = load_json(CASES_PATH)
    if not isinstance(catalog, list):
        return ["P0 case catalog is not an array"]
    expected_ids = [item["id"] for item in catalog]
    scope = payload.get("scope")
    expected_required = [item["id"] for item in catalog if scope == "full" or bool(item.get("startup"))]
    manual = payload.get("manual_cases", [])
    manual_ids = [item.get("id") for item in manual]
    if payload.get("product") != PRODUCT:
        errors.append("session product is invalid")
    if payload.get("sensitive_values_included") is not False:
        errors.append("session sensitive_values_included must be false")
    if scope not in {"full", "startup"}:
        errors.append("session scope must be full or startup")
    if payload.get("case_catalog_sha256") != sha256_file(CASES_PATH):
        errors.append("P0 case catalog hash does not match the repository")
    if manual_ids != expected_ids or len(set(manual_ids)) != len(expected_ids):
        errors.append("manual case IDs/order do not match the catalog")
    if payload.get("required_case_ids") != expected_required:
        errors.append("required_case_ids do not match the scope")
    automatic_ids = [item.get("id") for item in payload.get("automatic_checks", [])]
    if len(automatic_ids) != len(set(automatic_ids)):
        errors.append("automatic check IDs are duplicated")
    required = set(expected_required)
    for item in manual:
        status = item.get("status")
        if status not in STATUSES:
            errors.append(f"{item.get('id')} has invalid status: {status}")
        if item.get("id") in required and status in {"FAIL", "BLOCKED", "N/A"} and not str(item.get("note", "")).strip():
            errors.append(f"{item.get('id')} {status} requires a public-safe note")
    return errors


def record(args: argparse.Namespace) -> None:
    from .report import render_session

    directory, path = pathlib.Path(args.session_dir).resolve(), pathlib.Path(args.session_dir).resolve() / "p0-session.json"
    payload = load_json(path)
    if not isinstance(payload, dict):
        fail("p0-session.json root must be an object")
    cases = {item["id"]: item for item in payload.get("manual_cases", [])}
    notes: dict[str, str] = {}
    for raw in args.note or []:
        case_id, note = _assignment(raw, "--note")
        if case_id not in cases:
            fail(f"unknown case ID: {case_id}")
        notes[case_id] = safe_text(note)
    if not args.result:
        fail("record requires at least one --result ID=STATUS")
    for raw in args.result:
        case_id, status = _assignment(raw, "--result")
        status = status.upper()
        if case_id not in cases:
            fail(f"unknown case ID: {case_id}")
        if status not in STATUSES - {"PENDING"}:
            fail(f"invalid status for {case_id}: {status}")
        proposed_note = notes.get(case_id, str(cases[case_id].get("note", "")).strip())
        if status in {"FAIL", "BLOCKED", "N/A"} and not proposed_note:
            fail(f"{case_id} {status} requires --note {case_id}=<public-safe reason>")
        cases[case_id]["status"] = status
        if case_id in notes:
            cases[case_id]["note"] = notes[case_id]
        cases[case_id]["updated_at_utc"] = now()
    payload["updated_at_utc"] = now()
    payload["final"] = {
        "verdict": "PENDING", "finalized_at_utc": None, "blocking_items": [],
        "meaning": "Results changed; run finalize again. A session PASS applies only to this device and scope.",
    }
    write_json(path, payload)
    render_session(payload, directory / "P0_REPORT.md")
    print(f"Updated {len(args.result)} result(s)")


def blockers(payload: dict[str, Any]) -> tuple[str, list[str]]:
    integrity = validate_session(payload)
    blocked = [f"SYSTEM BLOCKED: {item}" for item in integrity]
    items = list(payload.get("automatic_checks", [])) + list(payload.get("manual_cases", []))
    for item in items:
        if item.get("status") in {"PENDING", "FAIL", "BLOCKED"}:
            blocked.append(f"{item['id']} {item['status']}: {item['title']}")
    statuses = {item.get("status") for item in items}
    if integrity:
        verdict = "BLOCKED"
    elif "FAIL" in statuses:
        verdict = "FAIL"
    elif "BLOCKED" in statuses:
        verdict = "BLOCKED"
    elif "PENDING" in statuses:
        verdict = "PENDING"
    else:
        verdict = "PASS"
    return verdict, blocked


def finalize(args: argparse.Namespace) -> int:
    from .report import render_session

    directory, path = pathlib.Path(args.session_dir).resolve(), pathlib.Path(args.session_dir).resolve() / "p0-session.json"
    payload = load_json(path)
    if not isinstance(payload, dict):
        fail("p0-session.json root must be an object")
    verdict, blocked = blockers(payload)
    payload["updated_at_utc"] = now()
    payload["final"] = {
        "verdict": verdict, "finalized_at_utc": now(), "blocking_items": blocked,
        "meaning": "PASS applies only to this device/scope; overall release requires a PASS matrix and release-owner approval.",
    }
    write_json(path, payload)
    render_session(payload, directory / "P0_REPORT.md")
    print(f"P0 session verdict: {verdict}")
    for item in blocked:
        print(f"- {item}")
    return {"PASS": 0, "FAIL": 1, "PENDING": 2, "BLOCKED": 3}[verdict]
