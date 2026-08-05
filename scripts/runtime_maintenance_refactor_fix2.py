from pathlib import Path
import re

# Lite handler assembly uses the same stateless execution boundary as Full.
path = Path("internal/handler/lite.go")
content = path.read_text()
content = content.replace("\t\t\ttranscodeService: services.Transcode,\n", "")
content = content.replace(
    "\t\t\ttranscodeService:  services.Transcode,\n",
    "\t\t\tmediaExecution:    services.MediaExecution,\n",
)
content = content.replace("\t\t\tmediaRepo:         repos.Media,\n", "")
path.write_text(content)

# Playback-position, bandwidth and throttle endpoints belonged to the retired
# media-keyed Runtime Job. Session heartbeat/status now own this lifecycle.
path = Path("internal/handler/stream.go")
content = path.read_text()
content, count = re.subn(
    r"\nfunc \(h \*StreamHandler\) Playback\(.*?\nfunc \(h \*StreamHandler\) STRMSegment",
    "\nfunc (h *StreamHandler) STRMSegment",
    content,
    count=1,
    flags=re.S,
)
if count != 1:
    raise SystemExit("legacy playback control handler block missing")
path.write_text(content)

for route_path in ["cmd/server/main.go", "cmd/server-lite/routes_stream.go"]:
    path = Path(route_path)
    if not path.exists():
        continue
    content = path.read_text()
    for method in ["Playback", "Bandwidth", "ThrottleStatus"]:
        content = content.replace(
            f"handlers.Stream.{method}",
            "handlers.Stream.RetiredRuntimeHLS",
        )
    path.write_text(content)

# The adapter no longer exists; verify that MediaExecution owns one stable
# process-local Runtime directly.
Path("internal/service/media_execution_test.go").write_text('''package service

import (
\t"fmt"
\t"path/filepath"
\t"testing"
\t"time"

\t"github.com/glebarez/sqlite"
\t"github.com/nowen-video/nowen-video/internal/config"
\t"go.uber.org/zap"
\t"gorm.io/gorm"
)

func TestMediaExecutionOwnsSingleProcessLocalRuntime(t *testing.T) {
\tdsn := fmt.Sprintf("file:media-execution-%d?mode=memory&cache=shared", time.Now().UnixNano())
\tdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tcfg := &config.Config{}
\tcfg.App.FFprobePath = "ffprobe"
\tcfg.App.FFmpegPath = "ffmpeg"
\tcfg.Cache.CacheDir = filepath.Join(t.TempDir(), "cache")

\texecution, err := NewMediaExecutionService(db, cfg, zap.NewNop().Sugar())
\tif err != nil {
\t\tt.Fatal(err)
\t}
\truntime := execution.ExecutionRuntime()
\tif runtime == nil {
\t\tt.Fatal("media execution did not expose FFmpeg runtime")
\t}
\tif execution.ExecutionRuntime() != runtime {
\t\tt.Fatal("media execution created more than one runtime")
\t}
}
''')
