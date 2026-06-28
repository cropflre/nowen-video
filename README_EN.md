<div align="center">

<h1>🎬 nowen-video</h1>

<p><b>Your personal home media center — lightweight, self-hosted, NAS-friendly.</b></p>

<p>
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/Docker-Alpine-2496ED?style=flat-square&logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square" alt="License">
</p>

<p>
  <a href="./README.md">简体中文</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-features">Features</a> •
  <a href="#-configuration">Configuration</a> •
  <a href="./desktop/README.md">Desktop App</a>
</p>

</div>

---

A lightweight home media server built with **Go + React**, similar to Jellyfin / Emby, optimized for NAS deployment.
**Single binary + SQLite**, one-click Docker startup, zero configuration required.

> 🖥️ **Desktop client** is available with **MKV / HEVC / HDR / Dolby Vision / DTS / Atmos zero-transcode playback** → see [desktop/README.md](./desktop/README.md)

## 📸 Screenshots

![screenshot1](1.png)
![screenshot2](2.png)

## ✨ Features

- 🎬 **Media library** — auto scan (MKV/MP4/AVI/MOV/WebM/TS/RMVB/...), FFprobe metadata, external subtitles, NFO compatibility (Kodi/Emby/Jellyfin), real-time file watching
- 📺 **Smart playback** — direct play for browser-compatible formats, on-demand HLS transcoding for the rest, ABR adaptive bitrate, keyboard shortcuts, Picture-in-Picture, bookmarks
- ⚡ **Hardware acceleration** — auto-detected Intel QSV / VAAPI / NVIDIA NVENC, software fallback, transcode cache reuse
- 🎨 **Multi-source scraping** — Provider Chain: TMDb → Douban → TheTVDB → Bangumi → Fanart.tv → AI fallback
- 📂 **Series & collections** — auto detection of `S01E01`, `1x01`, `第01集`, `EP01`, `Episode 01`; movie collections from TMDb
- 🔤 **Subtitles** — external (SRT/ASS/SSA/VTT/SUB/IDX/SUP) + embedded extraction, online search, AI ASR generation
- 👨‍👩‍👧‍👦 **Multi-user** — JWT auth, per-user history & favorites, playlists, parental controls, daily watch quota, content rating
- 🧠 **AI assistant** — natural language search, recommendation reasons, metadata enhancement, smart rename, scene detection (chapters/highlights)
- 📡 **Emby API compatibility** — Infuse / Kodi / Emby native clients work out of the box (140+ endpoints)
- 💻 **Cast** — DLNA / Chromecast device discovery and control
- 📊 **Analytics** — watch time stats, daily charts, admin dashboard
- 📁 **File manager** — browse / import / rename / batch scrape, AI-assisted rename, audit log
- 🔗 **Sharing & tagging** — share links with password & expiry, custom tags, bulk move, match rules
- 💓 **Pulse feed** — community activity stream, likes & comments
- 🛡️ **Security** — JWT, bcrypt, CORS, security headers, rate limiting, access log
- 🌐 **i18n** — Chinese / English / Japanese
- 🪶 **Lightweight** — single binary + SQLite (WAL), Alpine Docker image, healthcheck, PUID/PGID

## 🚀 Quick Start

### 1. Docker (recommended)

```bash
git clone https://github.com/your-repo/nowen-video.git
cd nowen-video
docker-compose up -d
```

Visit `http://your-host:8080` — default admin: `admin` / `admin123`

### 2. NAS deployment (Synology / QNAP / Unraid)

Edit `docker-compose.yml`:

```yaml
services:
  nowen-video:
    image: nowen-video:latest
    container_name: nowen-video
    ports:
      - "8080:8080"
    environment:
      - PUID=1000                                    # match your host user
      - PGID=1000
      - NOWEN_SECRETS_JWT_SECRET=change-me-please    # IMPORTANT
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/data                                 # database & config
      - ./cache:/cache                               # transcode cache
      - /volume1/Media:/media:ro                     # YOUR media folder
    devices:
      - /dev/dri:/dev/dri                            # optional: HW accel
    restart: unless-stopped
```

| Env / Param | Default | Description |
|---|---|---|
| `PUID` / `PGID` | `1000` | Run as this UID/GID (must match your media folder permissions) |
| `TZ` | `UTC` | Timezone |
| `NOWEN_APP_PORT` | `8080` | HTTP port |
| `NOWEN_SECRETS_JWT_SECRET` | *(required)* | JWT signing secret — **must be changed** |
| `NOWEN_APP_DATA_DIR` | `/data` | Data dir (DB + uploads) |
| `NOWEN_LOGGING_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `/dev/dri` device | — | Pass through Intel/AMD GPU for hardware transcoding |

### 3. Build from source

Requires **Go 1.22+**, **Node.js 20+**, **FFmpeg**.

```bash
go mod tidy
cd web && npm install && cd ..

# dev
make dev          # backend
make dev-web      # frontend (another terminal)

# production
make build
./bin/nowen-video
```

## ⚙️ Configuration

Configuration is loaded in this order (later overrides earlier):

```
1. Built-in defaults     → run with zero config
2. config.yaml           → main file (legacy flat or new nested)
3. config/*.yaml         → per-module split files
4. NOWEN_* env vars      → e.g. NOWEN_APP_PORT=8080
```

Common split files under `config/`:

| File | Purpose |
|---|---|
| `app.yaml` | port, debug, paths, FFmpeg location |
| `database.yaml` | SQLite path, WAL, connection pool |
| `secrets.yaml` | JWT secret, third-party API keys (⚠️ do **not** commit) |
| `logging.yaml` | level, format, rotation |
| `cache.yaml` | transcode cache directory & cleanup |
| `ai.yaml` | LLM provider config (OpenAI / DeepSeek / Qwen / Ollama) |

> **Note**: Hardware acceleration / concurrency / transcode preset / CPU limits are auto-tuned at startup and are **no longer exposed as config**.

### AI provider examples

```yaml
# OpenAI
ai: { provider: openai,   api_base: https://api.openai.com/v1,                  model: gpt-4o-mini }
# DeepSeek
ai: { provider: deepseek, api_base: https://api.deepseek.com/v1,                model: deepseek-chat }
# Qwen
ai: { provider: qwen,     api_base: https://dashscope.aliyuncs.com/compatible-mode/v1, model: qwen-turbo }
# Ollama (local)
ai: { provider: ollama,   api_base: http://localhost:11434/v1,                  model: llama3 }
```

## 🏗️ Tech Stack

**Backend** Go 1.22 · Gin · GORM + SQLite (WAL) · Zap · Viper · gorilla/websocket · fsnotify · FFmpeg

**Frontend** React 18 · TypeScript · Vite · Tailwind CSS · Zustand · HLS.js · React Router · Framer Motion

**Deploy** Docker (Alpine 3.19) · docker-compose

## 🗺️ Roadmap

- ✅ **v0.1 – v0.9** Core playback, scraping, multi-user, AI assistant, file manager, Pulse, sharing, tags
- ✅ **v0.9.5** Full Emby API compatibility layer (Infuse / Kodi / Emby native clients)
- 🔄 **v1.0** ABR seamless bitrate switching, FFmpeg throttling, mobile responsive, PWA, test coverage > 60%, Prometheus metrics
- 🚀 **v1.1+** Native mobile apps, 4K/HDR, WebDAV, distributed transcoding, AV1, plugin marketplace

## 💬 Community

- **QQ group**: `1093473044`
- **Issues**: please open a GitHub issue

## ☕ Sponsor

If this project helps you, consider buying the author a coffee / keyboard / bug-fix 🙏

<p align="center">
  <img src="./weixin.jpg" alt="WeChat Sponsor QR" width="260">
  <br>
  <i>Drug's WeChat sponsor QR — "Buy the author a keyboard / fix a bug"</i>
</p>

## 📜 License

Released under the [GNU General Public License v3.0](./LICENSE).

You may freely run, study, modify and distribute this software. Any derivative work distributed externally **must** also be released under GPL-3.0 with the original copyright notice preserved. The software is provided "as is", without warranty of any kind.
