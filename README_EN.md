<div align="center">

<h1>🎬 nowen-video</h1>

<p><b>Your personal home media center — self-hosted and built for NAS.</b></p>

<p>
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
  <img src="https://img.shields.io/badge/Android-Kotlin%20%2B%20Compose-3DDC84?style=flat-square&logo=android" alt="Android">
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/Docker-Alpine-2496ED?style=flat-square&logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square" alt="License">
</p>

<p>
  <a href="./README.md">简体中文</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-features">Features</a> •
  <a href="#-configuration">Configuration</a> •
  <a href="./docs/SERVER.md">Server Architecture</a> •
  <a href="./desktop/README.md">Desktop App</a> •
  <a href="./android/README.md">Android</a>
</p>

</div>

---

A home media server built with **Go + React**, similar to Jellyfin / Emby and optimized for NAS and self-hosted deployments.
**Single binary + SQLite**, Docker-first deployment, and a playback stack designed for long-running home servers.

> **Server edition:** Nowen Video now has one public production server edition. The former NAS-focused Lite architecture has been promoted to the official server and is no longer distributed as a separate Lite product. The old all-in-one runtime is retained only for migration, rollback, and compatibility verification. See [docs/SERVER.md](./docs/SERVER.md).
>
> 🖥️ The **desktop client** supports advanced local playback capabilities including MKV / HEVC / HDR / Dolby Vision / DTS / Atmos → see [desktop/README.md](./desktop/README.md)
>
> 📱 **Android** is built with Kotlin + Jetpack Compose and includes server discovery, QR login, library browsing, search, episode navigation, native playback, and offline capabilities. The repository now keeps only this official Android implementation → see [android/README.md](./android/README.md)

## 📸 Screenshots

![screenshot1](1.png)
![screenshot2](2.png)

## ✨ Features

- 🎬 **Media library** — automatic scanning, FFprobe metadata, external subtitles, NFO support, and real-time file watching
- 📺 **Playback planning** — one server-side planner selects direct play, remux, or on-demand HLS transcoding and provides fallback paths
- ⚡ **Hardware acceleration** — hardware-aware playback/transcoding with software fallback, persistent artifacts, recovery, and NAS-oriented lifecycle handling
- 🎨 **Multi-source metadata** — TMDb, Douban, TheTVDB, Bangumi, Fanart.tv, and related metadata providers
- 📂 **Series & collections** — common episode naming detection, TV navigation, and movie collection support
- 🔤 **Subtitles** — external and embedded subtitle handling plus online subtitle search
- 👨‍👩‍👧‍👦 **Multi-user** — JWT authentication, per-user history/favorites/playlists, library permissions, and content controls
- 🧠 **Optional AI** — desired configuration and actual runtime state are tracked separately so disabled AI components do not become unnecessary resident services
- 🌐 **Remote storage** — WebDAV / Alist / S3 capabilities are enabled only when configured
- ✅ **Unified task center** — library scans, scraping, and transcode-maintenance tasks share consistent lifecycle and progress reporting
- 📱 **Multi-client API** — Web, desktop, and Android share stable authentication, library, search, playback, favorites, history, and progress APIs
- 🛡️ **Security** — JWT, bcrypt, CORS, security headers, rate limiting, and access logging
- 🌍 **i18n** — Chinese / English / Japanese
- 🪶 **NAS-first deployment** — single binary + SQLite (WAL), Alpine image, health checks, PUID/PGID, and persistent data/cache directories

> Historical modules such as Emby compatibility, music/photos, casting, plugins, federation, and preprocessing are no longer advertised as default capabilities of the production runtime. The legacy compatibility runtime exists only for migration, rollback, and historical verification.

## 🚀 Quick Start

### 1. Docker (recommended)

```bash
git clone https://github.com/cropflre/nowen-video.git
cd nowen-video
docker-compose up -d
```

Open `http://your-host:8080`.

### 2. NAS deployment (Synology / QNAP / Unraid)

```yaml
services:
  nowen-video:
    image: nowen-video:latest
    container_name: nowen-video
    ports:
      - "8080:8080"
    environment:
      - PUID=1000
      - PGID=1000
      - NOWEN_SECRETS_JWT_SECRET=change-me-please
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/data
      - ./cache:/cache
      - /volume1/Media:/media:ro
    devices:
      - /dev/dri:/dev/dri
    restart: unless-stopped
```

| Env / Param | Default | Description |
|---|---|---|
| `PUID` / `PGID` | `1000` | Runtime UID/GID; match media-directory permissions |
| `TZ` | `UTC` | Timezone |
| `NOWEN_APP_PORT` | `8080` | HTTP port |
| `NOWEN_SECRETS_JWT_SECRET` | *(required)* | JWT signing secret; change it for first deployment |
| `NOWEN_APP_DATA_DIR` | `/data` | Database and persistent data directory |
| `NOWEN_LOGGING_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `/dev/dri` | — | Optional Intel/AMD GPU passthrough |

### 3. Build from source

Requires **Go**, **Node.js 20+**, and **FFmpeg**.

```bash
go mod tidy
cd web && npm install && cd ..

# official server development mode
make dev

# frontend dev server in another terminal
make dev-web

# production build
make build
./bin/nowen-video
```

`make build`, `make dev`, and the default `Dockerfile` all target the same official Nowen Video server.

The legacy compatibility runtime is not a second production edition. See [docs/SERVER.md](./docs/SERVER.md).

### 4. Android

The official Android client lives at the repository root under `android/` and supports Android 8.0 / API 26 and newer. The old Android V1 source and `clients/android-v2` tree have been removed; the project no longer maintains V1/V2 tracks or legacy-app data migration.

- [Android README](./android/README.md)
- [Release Guide](./android/RELEASE.md)
- [Smoke Test](./android/SMOKE_TEST.md)

> The official Android app now starts a new long-term production signing identity. Devices with an older V1 installation signed by another key may need to uninstall it before installing the current app. Future releases will reuse the new production keystore for normal in-place upgrades.

## ⚙️ Configuration

Configuration precedence:

```text
1. Built-in defaults
2. config.yaml
3. config/*.yaml
4. NOWEN_* environment variables
```

Common split files under `config/`:

| File | Purpose |
|---|---|
| `app.yaml` | port, debug, paths, FFmpeg location |
| `database.yaml` | SQLite path, WAL, connection pool |
| `secrets.yaml` | JWT secret and third-party API keys; never commit secrets |
| `logging.yaml` | level, format, rotation |
| `cache.yaml` | transcode cache and cleanup |
| `ai.yaml` | AI provider/model configuration |

## 🏗️ Tech Stack

**Backend** Go · Gin · GORM + SQLite (WAL) · Zap · Viper · gorilla/websocket · fsnotify · FFmpeg

**Frontend** React · TypeScript · Vite · Tailwind CSS · Zustand · HLS.js · React Router

**Android** Kotlin · Jetpack Compose · Media3 · Paging 3 · WorkManager · Hilt · Retrofit · Android Keystore

**Deployment** Docker (Alpine) · docker-compose

## 🗺️ Roadmap

Current mainline priorities:

- ✅ NAS-focused server promoted to the single official production edition
- ✅ Playback planning, fallback, and unified task lifecycle
- ✅ Persistent transcode execution state, leases, recovery, and shutdown protocol
- ✅ Web design-system and player UX consolidation
- ✅ Modular Android client promoted to the only official implementation
- 🧪 Android new production signing and release validation
- 🚀 Ongoing playback stability, subtitles, cross-client UX, and NAS resource-efficiency work

## 💬 Community

- **QQ group**: `1093473044`
- **Issues**: open a GitHub issue and do not publish private tokens, secrets, or private server addresses

## ☕ Sponsor

If this project helps you, consider buying the author a coffee / keyboard / bug-fix 🙏

<p align="center">
  <img src="./weixin.jpg" alt="WeChat Sponsor QR" width="260">
  <br>
  <i>Drug's WeChat sponsor QR — "Buy the author a keyboard / fix a bug"</i>
</p>

## 📜 License

Released under the [GNU General Public License v3.0](./LICENSE).

You may freely run, study, modify and distribute this software. Any derivative work distributed externally must also be released under GPL-3.0 with the original copyright notice preserved. The software is provided "as is", without warranty of any kind.
