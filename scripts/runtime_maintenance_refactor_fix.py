from pathlib import Path

path = Path("internal/service/stream_managed_remux.go")
content = path.read_text().replace("s.transcoder", "s.execution")
if "s.transcoder" in content:
    raise SystemExit("managed remux still references persistent transcoder")
path.write_text(content)

path = Path("internal/service/stream.go")
content = path.read_text().replace('\t"context"\n', "")
path.write_text(content)
