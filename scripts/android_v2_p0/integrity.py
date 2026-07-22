from __future__ import annotations

import pathlib
from typing import Any

from .common import fail, load_json

EXPECTED_AUTOMATIC_IDS = [f"AUTO-{index:02d}" for index in range(1, 11)]


def validate_session_file(session_dir: str | pathlib.Path) -> dict[str, Any]:
    directory = pathlib.Path(session_dir).resolve()
    payload = load_json(directory / "p0-session.json")
    if not isinstance(payload, dict):
        fail(f"invalid P0 session payload: {directory}")
    actual = [item.get("id") for item in payload.get("automatic_checks", [])]
    if actual != EXPECTED_AUTOMATIC_IDS:
        fail("automatic check IDs/order do not match AUTO-01 through AUTO-10")
    return payload
