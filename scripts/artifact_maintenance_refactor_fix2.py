#!/usr/bin/env python3
from __future__ import annotations

import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


write(
    "cmd/server/runtime_playback_removed_test.go",
    '''package main

import (
\t"errors"
\t"os"
\t"strings"
\t"testing"
)

func TestRetiredRuntimePlaybackImplementationRemainsDeleted(t *testing.T) {
\tfor _, path := range []string{
\t\t"../../internal/service/stream_startup_bridge.go",
\t\t"../../internal/service/stream_startup_bridge_contract.go",
\t\t"../../internal/service/transcode_handoff_attestation.go",
\t\t"../../internal/service/transcode_startup.go",
\t\t"../../internal/service/transcode_startup_continuation.go",
\t\t"../../internal/service/transcode_process_shutdown.go",
\t} {
\t\t_, statErr := os.Stat(path)
\t\tif statErr == nil {
\t\t\tt.Fatalf("retired runtime playback file still exists: %s", path)
\t\t}
\t\tif !errors.Is(statErr, os.ErrNotExist) {
\t\t\tt.Fatalf("stat retired runtime playback file %s: %v", path, statErr)
\t\t}
\t}

\tassertSourceOmits(t, "../../internal/service/ondemand.go", []string{
\t\t"ExecutionRuntime().Run",
\t\t"KindOnDemand",
\t\t"GetOnDemandOutputDir",
\t\t"-hls_playlist_type",
\t})
\tassertSourceOmits(t, "../../internal/service/stream_artifacts.go", []string{
\t\t"ResolveHLSOutputDir",
\t\t"importLegacyHLSArtifact",
\t\t"WaitForFirstSegmentForMedia",
\t\t"TouchArtifactAccess",
\t\t"ServeContent",
\t})
\tassertSourceOmits(t, "../../internal/service/media_probe_warmup.go", []string{
\t\t"runOnProbed(",
\t\t"startupSubmitted.Add",
\t\t"startupSkipped.Add",
\t\t"startupFailed.Add",
\t})
}

func assertSourceOmits(t *testing.T, path string, forbidden []string) {
\tt.Helper()
\tcontent, err := os.ReadFile(path)
\tif err != nil {
\t\tt.Fatalf("read %s: %v", path, err)
\t}
\tfor _, marker := range forbidden {
\t\tif strings.Contains(string(content), marker) {
\t\t\tt.Fatalf("%s reintroduced retired runtime marker %q", path, marker)
\t\t}
\t}
}
''',
)

write(
    "cmd/server/transcode_shutdown_test.go",
    '''package main

import (
\t"errors"
\t"os"
\t"strings"
\t"testing"
)

func TestFullServerOwnsOrderedArtifactMaintenanceShutdown(t *testing.T) {
\tmainSource, err := os.ReadFile("main.go")
\tif err != nil {
\t\tt.Fatalf("read full server main.go: %v", err)
\t}
\tsource := string(mainSource)

\trequired := []string{
\t\t"signal.Stop(quit)",
\t\t"srv.Shutdown(httpCtx)",
\t\t"services.ArtifactMaintenance.Shutdown(transcodeCtx)",
\t\t"context.WithTimeout(context.Background(), 30*time.Second)",
\t}
\tfor _, token := range required {
\t\tif !strings.Contains(source, token) {
\t\t\tt.Fatalf("full server shutdown contract missing %q", token)
\t\t}
\t}

\thttpShutdown := strings.Index(source, "srv.Shutdown(httpCtx)")
\tmaintenanceShutdown := strings.Index(source, "services.ArtifactMaintenance.Shutdown(transcodeCtx)")
\tif httpShutdown < 0 || maintenanceShutdown < 0 || httpShutdown >= maintenanceShutdown {
\t\tt.Fatalf("full server must stop accepting HTTP requests before stopping artifact maintenance")
\t}
}

func TestLegacyFullSignalHookIsPhysicallyRemoved(t *testing.T) {
\tpath := "../../internal/service/transcode_process_shutdown.go"
\t_, err := os.Stat(path)
\tif err == nil {
\t\tt.Fatalf("legacy transcode process shutdown bridge still exists: %s", path)
\t}
\tif !errors.Is(err, os.ErrNotExist) {
\t\tt.Fatalf("stat legacy transcode process shutdown bridge: %v", err)
\t}
}
''',
)
