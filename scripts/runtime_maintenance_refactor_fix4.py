from pathlib import Path

for route_path in [
    "cmd/server/main.go",
    "cmd/server-lite/routes_admin.go",
    "cmd/server-lite/routes_core.go",
    "cmd/server-lite/routes_stream.go",
]:
    path = Path(route_path)
    if not path.exists():
        continue
    content = path.read_text()
    content = content.replace(
        "handlers.Admin.RetiredRuntimeTranscodeTask",
        "handlers.Admin.RetiredRuntimeTranscode",
    )
    for method in ["Playback", "Bandwidth", "ThrottleStatus"]:
        content = content.replace(
            f"handlers.Stream.{method}",
            "handlers.Stream.RetiredRuntimeHLS",
        )
    path.write_text(content)
