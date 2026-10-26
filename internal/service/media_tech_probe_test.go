package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

// Re-exec the test binary as ffprobe, including on Windows. This exercises the
// real command, timeout and JSON parser without requiring a remote service.
func TestMain(m *testing.M) {
	if os.Getenv("NOWEN_TECH_PROBE_HELPER") == "1" {
		f, err := os.OpenFile(os.Getenv("NOWEN_TECH_PROBE_CALLS"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(2)
		}
		_ = json.NewEncoder(f).Encode(os.Args[1:])
		_ = f.Close()
		if delay, err := time.ParseDuration(os.Getenv("NOWEN_TECH_PROBE_DELAY")); err == nil {
			time.Sleep(delay)
		}
		fmt.Print(os.Getenv("NOWEN_TECH_PROBE_OUTPUT"))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

const techProbeFixture = `{"streams":[
{"index":0,"codec_type":"video","codec_name":"hevc","width":3840,"height":2160,"r_frame_rate":"24000/1001","pix_fmt":"yuv420p10le","color_transfer":"smpte2084"},
{"index":1,"codec_type":"audio","codec_name":"aac","channels":6,"channel_layout":"5.1","tags":{"language":"eng"}},
{"index":2,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"chi"}}
],"format":{"format_name":"matroska,webm","duration":"120.5","size":"10485760","bit_rate":"8000000","nb_streams":3}}`

func newTechProbeTestService(t *testing.T) (*MediaService, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	calls := filepath.Join(t.TempDir(), "calls.jsonl")
	t.Setenv("NOWEN_TECH_PROBE_HELPER", "1")
	t.Setenv("NOWEN_TECH_PROBE_CALLS", calls)
	t.Setenv("NOWEN_TECH_PROBE_OUTPUT", techProbeFixture)
	cfg := &config.Config{}
	cfg.App.FFprobePath = exe
	cfg.STRM.RemoteProbe = true
	cfg.STRM.RemoteProbeTimeout = 2
	return &MediaService{cfg: cfg, logger: zap.NewNop().Sugar()}, calls
}

func techProbeCalls(t *testing.T, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, args)
	}
	return calls
}

func TestSTRMTechSpecsResolvesTargetAndHeaders(t *testing.T) {
	s, calls := newTechProbeTestService(t)
	s.cfg.STRM.DefaultUserAgent = "global-agent"
	s.cfg.STRM.DefaultReferer = "https://default.example/"
	s.cfg.STRM.DomainUserAgents = map[string]string{"example.test": "domain-agent"}
	path := filepath.Join(t.TempDir(), "movie.strm")
	if err := os.WriteFile(path, []byte("# cookie=session=secret\n# header=X-Token:file-token\nhttps://example.test/download?id=123\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".json", []byte(`{"referer":"https://sidecar.example/"}`), 0600); err != nil {
		t.Fatal(err)
	}
	media := &model.Media{FilePath: path, StreamUA: "override-agent", StreamHeaders: `{"X-Token":"override-token"}`}
	specs := s.probeTechSpecs(media)
	if specs == nil || len(specs.Streams) != 3 || specs.Streams[0].CodecName != "hevc" || specs.Streams[0].FrameRate != "23.976" || specs.Streams[0].ColorTransfer != "smpte2084" || specs.Streams[1].Channels != 6 || specs.Streams[2].Language != "chi" || specs.Format.Size != "10485760" {
		t.Fatalf("missing full remote specs: %+v", specs)
	}
	if media.StreamURL != "" || media.StreamUA != "override-agent" {
		t.Fatal("detail probing mutated media")
	}
	invocations := techProbeCalls(t, calls)
	if len(invocations) != 1 {
		t.Fatalf("got %d probe calls", len(invocations))
	}
	args := strings.Join(invocations[0], "|")
	for _, want := range []string{"-i|https://example.test/download?id=123", "-user_agent|override-agent", "Referer: https://sidecar.example/\r\n", "Cookie: session=secret\r\n", "X-Token: override-token\r\n"} {
		if !strings.Contains(args, want) {
			t.Errorf("missing argument %q", want)
		}
	}
	if strings.Contains(args, path) {
		t.Fatal("passed STRM text to ffprobe")
	}
}

func TestSTRMTechSpecsCacheAndInvalidation(t *testing.T) {
	s, calls := newTechProbeTestService(t)
	t.Setenv("NOWEN_TECH_PROBE_DELAY", "100ms")
	media := &model.Media{FilePath: "movie.strm", StreamURL: "https://example.test/stream.m3u8", StreamUA: "agent-one"}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.probeTechSpecs(media) == nil {
				t.Error("concurrent remote probe failed")
			}
		}()
	}
	wg.Wait()
	_ = s.probeTechSpecs(media)
	if n := len(techProbeCalls(t, calls)); n != 1 {
		t.Fatalf("expected one shared probe, got %d", n)
	}
	media.StreamUA = "agent-two"
	_ = s.probeTechSpecs(media)
	media.StreamURL = "https://example.test/new-signed-target"
	_ = s.probeTechSpecs(media)
	if n := len(techProbeCalls(t, calls)); n != 3 {
		t.Fatalf("header/URL changes did not invalidate cache: %d", n)
	}
	for key, entry := range s.techProbes {
		entry.expiresAt = time.Now().Add(-time.Second)
		s.techProbes[key] = entry
	}
	_ = s.probeTechSpecs(media)
	if n := len(techProbeCalls(t, calls)); n != 4 {
		t.Fatalf("expired cache did not retry: %d", n)
	}
}

func TestSTRMTechSpecsOptOutAndUnsupportedSources(t *testing.T) {
	s, calls := newTechProbeTestService(t)
	media := &model.Media{FilePath: "movie.strm", StreamURL: "https://example.test/movie.mkv"}
	s.cfg.STRM.RemoteProbe = false
	if s.probeTechSpecs(media) != nil {
		t.Fatal("ignored remote probe opt-out")
	}
	s.cfg.STRM.RemoteProbe = true
	for _, source := range []string{"magnet:?xt=abc", "file:///private/video.mkv", "rtsp://example.test/live", "https:///missing-host"} {
		media.StreamURL = source
		if s.probeTechSpecs(media) != nil {
			t.Errorf("probed unsupported source %q", source)
		}
	}
	if len(techProbeCalls(t, calls)) != 0 {
		t.Fatal("unexpected ffprobe invocation")
	}
}

func TestSTRMTechSpecsTimeoutAndFailureCache(t *testing.T) {
	s, calls := newTechProbeTestService(t)
	s.cfg.STRM.RemoteProbeTimeout = 1
	t.Setenv("NOWEN_TECH_PROBE_DELAY", "3s")
	media := &model.Media{StreamURL: "https://example.test/slow"}
	start := time.Now()
	if s.probeTechSpecs(media) != nil || time.Since(start) > 2500*time.Millisecond {
		t.Fatal("remote probe did not fail within its timeout")
	}
	_ = s.probeTechSpecs(media)
	if n := len(techProbeCalls(t, calls)); n != 1 {
		t.Fatalf("failed probe was immediately repeated: %d", n)
	}
}

func TestTechSpecsLocalAndInvalidOutput(t *testing.T) {
	s, calls := newTechProbeTestService(t)
	s.cfg.STRM.RemoteProbe = false
	if s.probeTechSpecs(&model.Media{FilePath: "local.mkv"}) == nil {
		t.Fatal("remote opt-out disabled local probing")
	}
	if got := techProbeCalls(t, calls)[0]; got[len(got)-1] != "local.mkv" {
		t.Fatalf("wrong local source: %v", got)
	}
	s.cfg.STRM.RemoteProbe = true
	for i, output := range []string{"invalid JSON", `{}`, `{"streams":[]}`} {
		t.Setenv("NOWEN_TECH_PROBE_OUTPUT", output)
		if s.probeTechSpecs(&model.Media{StreamURL: fmt.Sprintf("https://example.test/bad/%d", i)}) != nil {
			t.Errorf("accepted invalid/empty probe output %q", output)
		}
	}
}

func TestSTRMTechSpecsHTTPIntegration(t *testing.T) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	root := t.TempDir()
	videoPath := filepath.Join(root, "sample.mkv")
	output, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=320x240:r=25:d=1", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:v", "mpeg4", "-c:a", "aac", "-shortest", videoPath).CombinedOutput()
	if err != nil {
		t.Fatalf("generate fixture: %v: %s", err, output)
	}
	video, err := os.ReadFile(videoPath)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("User-Agent") != "probe-test-agent" || r.Header.Get("Referer") != "https://library.example/" || r.Header.Get("Cookie") != "session=test" || r.Header.Get("X-Token") != "test-token" {
			http.Error(w, "missing authentication headers", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/download" {
			http.Redirect(w, r, "/video", http.StatusFound)
			return
		}
		http.ServeContent(w, r, "sample.mkv", time.Time{}, bytes.NewReader(video))
	}))
	defer server.Close()
	path := filepath.Join(root, "sample.strm")
	if err := os.WriteFile(path, []byte("# ua=probe-test-agent\n# referer=https://library.example/\n# cookie=session=test\n# header=X-Token:test-token\n"+server.URL+"/download\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.App.FFprobePath = ffprobe
	cfg.STRM.RemoteProbe = true
	cfg.STRM.RemoteProbeTimeout = 5
	s := &MediaService{cfg: cfg, logger: zap.NewNop().Sugar()}
	media := &model.Media{FilePath: path, FileSize: 128, VideoCodec: "strm_mkv"}
	specs := s.probeTechSpecs(media)
	if specs == nil || len(specs.Streams) != 2 || specs.Streams[0].Width != 320 || specs.Streams[0].Height != 240 || specs.Streams[0].FrameRate != "25.000" || specs.Streams[1].CodecName != "aac" || specs.Format.Size != fmt.Sprint(len(video)) {
		t.Fatalf("real HTTP ffprobe did not return complete specs: %+v", specs)
	}
	count := requests.Load()
	if count < 2 {
		t.Fatal("redirect was not exercised")
	}
	_ = s.probeTechSpecs(media)
	if requests.Load() != count {
		t.Fatal("cached detail fetched remote video again")
	}
	if media.FileSize != 128 || media.VideoCodec != "strm_mkv" || media.StreamURL != "" {
		t.Fatal("detail probing changed stored index metadata")
	}
}
