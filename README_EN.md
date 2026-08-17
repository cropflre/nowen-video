<div align="center">

<h1>🎬 Nowen Video</h1>

<p><b>A private home media platform built for NAS and self-hosted environments.</b></p>

<p>
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
  <img src="https://img.shields.io/badge/Android-Kotlin%20%2B%20Compose-3DDC84?style=flat-square&logo=android" alt="Android">
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/FFmpeg-8.1.2-007808?style=flat-square&logo=ffmpeg" alt="FFmpeg">
  <img src="https://img.shields.io/badge/Docker-Alpine-2496ED?style=flat-square&logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square" alt="License">
</p>

<p>
  <a href="./README.md">简体中文</a> •
  <a href="#-core-features">Core Features</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-clients--platforms">Clients & Platforms</a> •
  <a href="#%EF%B8%8F-configuration">Configuration</a> •
  <a href="./docs/SERVER.md">Server Architecture</a> •
  <a href="./desktop/README.md">Desktop</a> •
  <a href="./android/README.md">Android</a>
</p>

</div>

---

Nowen Video is a home media platform built with **Go + React + SQLite + FFmpeg** for NAS devices, home servers, and self-hosted environments.

It covers the complete workflow from **media scanning, metadata scraping, library management, detail browsing, search, favorites and history to direct play, remux, on-demand HLS transcoding, subtitles, and multi-client access**. The project is optimized for long-running servers, low maintenance overhead, and consistent cross-client behavior.

> **Product identity:** Nowen Video now has one public production server edition. The historical Lite / Full product split is no longer part of the official product surface. The legacy compatibility runtime is retained only for migration, rollback, and historical validation. See [docs/SERVER.md](./docs/SERVER.md).

## 📸 Screenshots

![Nowen Video Screenshot 1](1.png)
![Nowen Video Screenshot 2](2.png)

## ✨ Core Features

### 🎬 Media Library & Metadata

- Automatically scans common media formats including MKV / MP4 / AVI / MOV / WebM / TS / RMVB
- Uses FFprobe to inspect streams, codecs, duration, and other media metadata
- Supports NFO files, external subtitles, filesystem watching, and media asset refresh
- Integrates metadata sources such as TMDb, Douban, TheTVDB, Bangumi, and Fanart.tv
- Recognizes common movie, TV series, season, and episode naming patterns
- Supports movie collections, episode navigation, and cinematic media detail workspaces

### ▶️ Playback, Remux & Transcoding

- A unified server-side playback planner selects **direct play / remux / on-demand HLS transcoding** based on media and client capabilities
- Automatic fallback paths reduce manual playback troubleshooting
- The official Docker image includes FFmpeg 8.1.2
- Supports Intel / AMD `/dev/dri` hardware acceleration environments with software fallback
- Transcode jobs include persistent execution state, cached artifacts, validation, recovery, and cleanup
- Player controls cover subtitles, playback speed, episode switching, and other common playback actions

### ✨ Local Media Analysis & Highlights

- Supports local media analysis jobs with real-time progress events
- Uses sparse two-stage highlight analysis to avoid unnecessary full-file processing
- Provides a Highlights tab and dedicated highlight clip playback mode
- Animated previews are generated on demand and can be persisted lazily
- Detail-page media assets can refresh after analysis completes

> Local media analysis is an evolving capability. Normal browsing and playback do not require every media item to be pre-analyzed.

### 🎨 Aurora / Neo Glass Experience

- Home, library, search, favorites, history, detail, and player experiences are being unified under the Aurora visual system
- Media detail pages support standalone backdrops, Hero slideshows, status sidebars, and real tab navigation
- Favorites, history, and continue-watching use a consistent media workspace
- The sidebar supports collapse behavior and the player chrome follows the same glass-based visual language
- Layouts are continuously refined for long titles, empty states, narrow screens, and dense media libraries

### 🔤 Subtitles

- External subtitle scanning and playback selection
- Embedded subtitle track handling
- Online subtitle search
- Shared subtitle-related APIs across Web, Desktop, and Android playback clients

### 👨‍👩‍👧‍👦 Users & Personal Space

- JWT authentication with bcrypt password storage
- The first registered user becomes an administrator automatically
- Per-user favorites, watch history, continue watching, and playback progress
- Media library permissions and content controls
- A unified task center reports scan, scraping, and transcode-maintenance lifecycle and progress

### 🌐 Remote Storage & Optional Capabilities

- WebDAV / Alist / S3 capabilities can be enabled through configuration
- AI-related capabilities are activated according to configuration and actual runtime state instead of being forced into the resident runtime
- WebSocket events are used for task and media-analysis progress updates

### 🛡️ NAS & Long-Running Runtime

- SQLite WAL persistence
- Official Alpine-based Docker image
- Separate persistent `/data` and `/cache` directories
- PUID / PGID runtime identity support
- Container health checks
- Dedicated handling for transcode cache, task recovery, resource boundaries, and long-running NAS workloads

## 🚀 Quick Start

### 1. Docker (Recommended)

Use the official Docker Hub image directly:

```bash
docker run -d \
  --name nowen-video \
  -p 8080:8080 \
  -e PUID=1000 \
  -e PGID=1000 \
  -e TZ=Asia/Shanghai \
  -v $(pwd)/data:/data \
  -v $(pwd)/cache:/cache \
  -v /path/to/media:/media:ro \
  --restart unless-stopped \
  cropflre/nowen-video:latest
```

For Intel / AMD hardware acceleration on Linux / NAS hosts, pass through `/dev/dri` as well:

```bash
--device /dev/dri:/dev/dri
```

Open:

```text
http://your-host:8080
```

The first registered user automatically becomes the administrator.

### 2. Docker Compose

```yaml
services:
  nowen-video:
    image: cropflre/nowen-video:latest
    container_name: nowen-video
    ports:
      - "8080:8080"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/data
      - ./cache:/cache
      - /volume1/Media:/media:ro
    devices:
      - /dev/dri:/dev/dri
    restart: unless-stopped
```

> Remove the `devices` section if hardware acceleration is not required.

If no JWT secret is configured explicitly, the server generates a random secret on first startup and persists it in the data directory. `NOWEN_SECRETS_JWT_SECRET` remains available for installations that use centralized secret management.

### 3. Build from Source

Requires **Go 1.25**, **Node.js 20+**, and **FFmpeg**.

```bash
git clone https://github.com/cropflre/nowen-video.git
cd nowen-video

go mod download
cd web && npm ci && cd ..

# official server development mode
make dev

# frontend dev server in another terminal
make dev-web

# production build
make build
./bin/nowen-video
```

`make build`, `make dev`, and the default `Dockerfile` all target the same official Nowen Video production server.

## 📱 Clients & Platforms

### Web

The Web app is the primary management and viewing experience, covering library browsing, search, media details, playback, favorites, history, continue watching, task status, and administration.

### 🖥️ Desktop

The desktop client is built with **Tauri 2.0 + libmpv** for Windows / macOS / Linux and supports both Web `<video>` and native mpv playback engines.

For local playback it can cover media scenarios that browsers commonly struggle with, including MKV, HEVC, AV1, HDR, Dolby Vision, DTS, TrueHD, and Atmos.

See [desktop/README.md](./desktop/README.md).

### 📱 Android

The repository root `android/` directory contains the only official Android client:

- Kotlin + Jetpack Compose
- Media3
- Hilt
- Retrofit / OkHttp
- Paging 3
- WorkManager
- Android Keystore
- Minimum Android 8.0 / API 26
- targetSdk API 35
- applicationId: `com.nowen.video`

The project no longer maintains separate V1 / V2 Android products.

> Older V1 installations may use a different signing identity and can require uninstalling the old app before installing the current official client. Future official releases continue using the current production signing identity for normal in-place upgrades.

See [android/README.md](./android/README.md).

### 🐮 fnOS

Nowen Video has an official fnOS `.fpk` build and release flow covering package resources, Docker Project integration, desktop entry, install / upgrade / uninstall lifecycle hooks, permissions, and fnpack validation.

Official releases provide the corresponding `.fpk` asset through GitHub Release.

## 📦 Release Channels

Official releases are distributed through:

- **Docker Hub**: `cropflre/nowen-video:<version>` / `cropflre/nowen-video:latest`
- **Android**: APK / AAB
- **fnOS**: `.fpk`
- **GitHub Release**: source, release notes, and release assets

The release pipeline validates Server CI, Release Contract, production client candidates, remote Docker manifests, Git tags, GitHub Release assets, and channel artifact integrity before considering a release successful.

## ⚙️ Configuration

Configuration precedence, where later sources override earlier ones:

```text
1. Built-in defaults
2. config.yaml
3. config/*.yaml
4. NOWEN_* environment variables
```

Common runtime configuration:

| Setting | Default | Description |
|---|---|---|
| `PUID` / `PGID` | image runtime user | Container runtime UID / GID |
| `TZ` | `Asia/Shanghai` in official image | Timezone |
| `NOWEN_APP_PORT` | `8080` | HTTP port |
| `NOWEN_APP_DATA_DIR` | `/data` | Persistent data directory |
| `NOWEN_DATABASE_DB_PATH` | `/data/nowen.db` | SQLite database path |
| `NOWEN_CACHE_CACHE_DIR` | `/cache` | Transcode and task cache directory |
| `NOWEN_LOGGING_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `NOWEN_SECRETS_JWT_SECRET` | generated automatically | Optional custom JWT signing secret |

Common split files under `config/`:

| File | Purpose |
|---|---|
| `app.yaml` | port, debug, paths, FFmpeg location |
| `database.yaml` | SQLite path, WAL, connection pool |
| `secrets.yaml` | JWT secret and third-party API keys |
| `logging.yaml` | log level, format, rotation |
| `cache.yaml` | transcode cache and cleanup |
| `ai.yaml` | AI provider and model configuration |

> Never commit real tokens, API keys, JWT secrets, or private server addresses to Git.

## 🏗️ Tech Stack

**Backend**: Go 1.25 · Gin · GORM · SQLite (WAL) · Zap · Viper · gorilla/websocket · fsnotify · FFmpeg 8.1.2

**Web**: React 18 · TypeScript · Vite · Tailwind CSS · Fluent UI · Framer Motion · Zustand · HLS.js · React Router

**Desktop**: Tauri 2.0 · Rust · WebView · mpv / libmpv

**Android**: Kotlin · Jetpack Compose · Media3 · Paging 3 · WorkManager · Hilt · Retrofit · Android Keystore

**Deployment**: Docker (Alpine) · Docker Compose · fnOS

## 🗺️ Current Direction

- ✅ One official production server identity
- ✅ Direct play / remux / HLS transcoding with automatic fallback
- ✅ Persistent transcode execution state, leases, recovery, and artifact governance
- ✅ Aurora / Neo Glass Web visual system
- ✅ Local media analysis and highlight capabilities
- ✅ Modular official Android client
- ✅ Unified Docker / Android / fnOS / GitHub Release pipeline
- 🚀 Continued work on playback stability, subtitles, cross-client consistency, media analysis, and NAS resource efficiency

## 💬 Community

- **QQ Group**: `1093473044`
- **GitHub Issues**: bug reports, feature requests, and compatibility feedback are welcome
- Do not publish tokens, secrets, or private server addresses in public issues

## ☕ Sponsor

If this project helps you, consider buying the author a coffee / keyboard / bug fix 🙏

<p align="center">
  <img src="./weixin.jpg" alt="WeChat Sponsor QR" width="260">
  <br>
  <i>Drug's WeChat sponsor QR — “Buy the author a keyboard / fix a bug”</i>
</p>

## 📜 License

Released under the [GNU General Public License v3.0](./LICENSE).

You may run, study, modify, and distribute this software under the terms of GPL-3.0. Derivative works distributed externally must comply with GPL-3.0 and preserve the applicable copyright and license notices. The software is provided “as is”, without warranty of any kind.
