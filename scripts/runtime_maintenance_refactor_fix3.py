from pathlib import Path

path = Path("internal/handler/stream.go")
content = path.read_text().replace('\t"strconv"\n', "")
path.write_text(content)
