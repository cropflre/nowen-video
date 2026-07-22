<div align="center">

<h1>🎬 nowen-video</h1>

<p><b>你的私人家庭影音中心 — 轻量、自托管、为 NAS 而生。</b></p>

<p>
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/React-18-61DAFB?style=flat-square&logo=react" alt="React">
  <img src="https://img.shields.io/badge/Android-Kotlin%20%2B%20Compose-3DDC84?style=flat-square&logo=android" alt="Android">
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?style=flat-square&logo=sqlite" alt="SQLite">
  <img src="https://img.shields.io/badge/Docker-Alpine-2496ED?style=flat-square&logo=docker" alt="Docker">
  <img src="https://img.shields.io/badge/License-GPL--3.0-blue?style=flat-square" alt="License">
</p>

<p>
  <a href="./README_EN.md">English</a> •
  <a href="#-快速开始">快速开始</a> •
  <a href="#-核心特性">特性</a> •
  <a href="#%EF%B8%8F-配置说明">配置</a> •
  <a href="./desktop/README.md">桌面客户端</a> •
  <a href="./clients/android-v2/README.md">Android V2</a>
</p>

</div>

---

基于 **Go + React** 构建的轻量级家庭媒体服务器，类似 Jellyfin / Emby，专为 NAS 部署优化。
**单二进制 + SQLite**，Docker 一键启动，零配置即可使用。

> 🖥️ **PC 桌面客户端** 已上线，支持 **MKV / HEVC / HDR / 杜比视界 / DTS / Atmos 零转码播放** → 详见 [desktop/README.md](./desktop/README.md)
>
> 📱 **Android V2 RC1 候选版** 已具备服务器发现、扫码登录、媒体库、聚合搜索、剧集导航、原生播放和离线下载 → 详见 [clients/android-v2/README.md](./clients/android-v2/README.md)

## 📸 功能截图

![截图1](1.png)
![截图2](2.png)

## ✨ 核心特性

- 🎬 **媒体库** — 自动扫描（MKV/MP4/AVI/MOV/WebM/TS/RMVB 等），FFprobe 元数据提取，外挂字幕，NFO 兼容（Kodi/Emby/Jellyfin），fsnotify 实时监控
- 📺 **智能播放** — 浏览器兼容格式直接播放，不兼容格式按需 HLS 转码，ABR 自适应码率，键盘快捷键，画中画，书签
- ⚡ **硬件加速** — 自动检测 Intel QSV / VAAPI / NVIDIA NVENC，软件兜底，转码缓存复用
- 🎨 **多源刮削** — Provider Chain 调度链：TMDb → 豆瓣 → TheTVDB → Bangumi → Fanart.tv → AI 兜底
- 📂 **剧集与合集** — 自动识别 `S01E01` / `1x01` / `第01集` / `EP01`；TMDb 电影系列合集
- 🔤 **字幕** — 外挂字幕（SRT/ASS/SSA/VTT/SUB/IDX/SUP）+ 内嵌提取，在线搜索，AI ASR 生成
- 👨‍👩‍👧‍👦 **多用户** — JWT 认证，独立观看历史/收藏/播放列表，家长控制，每日观看时长，内容分级
- 🧠 **AI 助手** — 自然语言搜索、推荐理由、元数据增强、智能重命名、场景识别（章节/精彩片段）
- 📡 **Emby API 兼容** — Infuse / Kodi / Emby 原生客户端无缝接入（140+ 接口）
- 📱 **Android 原生客户端** — Kotlin + Compose，局域网发现、二维码添加、聚合搜索、Media3 播放、Paging 3 和离线下载
- 💻 **投屏** — DLNA / Chromecast 设备发现与控制
- 📊 **统计** — 观影时长、按日图表、管理员仪表板
- 📁 **文件管理** — 浏览/导入/重命名/批量刮削，AI 重命名建议，操作审计日志
- 🔗 **分享与标签** — 带密码与过期时间的分享链接，自定义标签，批量移动，匹配规则
- 💓 **Pulse 动态** — 社区活动流，点赞与评论
- 🛡️ **安全** — JWT、bcrypt、CORS、安全响应头、限流、访问日志
- 🌐 **国际化** — 简体中文 / English / 日本語
- 🪶 **轻量** — 单二进制 + SQLite (WAL)，Alpine Docker 镜像，健康检查，PUID/PGID

## 🚀 快速开始

### 一、Docker 部署（推荐）

```bash
git clone https://github.com/your-repo/nowen-video.git
cd nowen-video
docker-compose up -d
```

打开浏览器访问 `http://你的主机IP:8080` — 默认管理员：`admin` / `admin123`

### 二、NAS 部署（群晖 / 威联通 / Unraid）

编辑 `docker-compose.yml`：

```yaml
services:
  nowen-video:
    image: nowen-video:latest
    container_name: nowen-video
    ports:
      - "8080:8080"
    environment:
      - PUID=1000                                    # 通过 `id` 命令查看
      - PGID=1000
      - NOWEN_SECRETS_JWT_SECRET=change-me-please    # ⚠️ 必须修改
      - TZ=Asia/Shanghai
    volumes:
      - ./data:/data                                 # 数据库与配置
      - ./cache:/cache                               # 转码缓存
      - /volume1/Media:/media:ro                     # 你的媒体目录
    devices:
      - /dev/dri:/dev/dri                            # 可选：硬件加速
    restart: unless-stopped
```

**容器参数说明：**

| 环境变量 / 参数 | 默认值 | 说明 |
|---|---|---|
| `PUID` / `PGID` | `1000` | 运行用户的 UID/GID（必须匹配宿主机媒体目录权限） |
| `TZ` | `UTC` | 时区，建议 `Asia/Shanghai` |
| `NOWEN_APP_PORT` | `8080` | HTTP 端口 |
| `NOWEN_SECRETS_JWT_SECRET` | *(必填)* | JWT 签名密钥 — **首次部署务必修改** |
| `NOWEN_APP_DATA_DIR` | `/data` | 数据目录（数据库 + 上传文件） |
| `NOWEN_LOGGING_LEVEL` | `info` | 日志级别：`debug` / `info` / `warn` / `error` |
| `/dev/dri` 设备 | — | 透传 Intel/AMD GPU 用于硬件转码（NVIDIA 需用 `runtime: nvidia`） |

### 三、源码构建

环境要求：**Go 1.22+**、**Node.js 20+**、**FFmpeg**

```bash
go mod tidy
cd web && npm install && cd ..

# 开发模式
make dev          # 启动后端
make dev-web      # 启动前端（另开终端）

# 生产构建
make build
./bin/nowen-video
```

### 四、Android V2 RC1

Android V2 最低支持 Android 8.0 / API 26。Debug 构建需要 JDK 17 和 Android SDK 35：

```bash
chmod +x android/gradlew
./android/gradlew -p clients/android-v2 testDebugUnitTest lintDebug assembleDebug
adb install -r clients/android-v2/app/build/outputs/apk/debug/app-debug.apk
```

正式签名、公开测试包安装、覆盖升级、旧版并行策略和真机检查表见：

- [Android V2 README](./clients/android-v2/README.md)
- [Release Guide](./clients/android-v2/RELEASE.md)
- [Migration Guide](./clients/android-v2/MIGRATION.md)
- [RC1 Smoke Test](./clients/android-v2/SMOKE_TEST.md)

## ⚙️ 配置说明

配置加载顺序（后者覆盖前者）：

```
1. 内置默认值        → 零配置可运行
2. config.yaml       → 主配置文件（旧版扁平 / 新版嵌套均支持）
3. config/*.yaml     → 分片配置文件（按模块分类）
4. NOWEN_* 环境变量  → 例如 NOWEN_APP_PORT=8080
```

`config/` 目录下常用分片：

| 文件 | 用途 |
|---|---|
| `app.yaml` | 端口、调试、路径、FFmpeg 位置 |
| `database.yaml` | SQLite 路径、WAL、连接池 |
| `secrets.yaml` | JWT 密钥、第三方 API Key（⚠️ 切勿提交到 Git） |
| `logging.yaml` | 日志级别、格式、轮转 |
| `cache.yaml` | 转码缓存目录与清理策略 |
| `ai.yaml` | LLM 提供商配置（OpenAI / DeepSeek / 通义千问 / Ollama） |

> **提示**：硬件加速 / 并发数 / 转码预设 / CPU 资源限制等参数已由系统启动时自动调优，不再暴露为配置项。

### AI 提供商示例

```yaml
# OpenAI
ai: { provider: openai,   api_base: https://api.openai.com/v1,                  model: gpt-4o-mini }
# DeepSeek
ai: { provider: deepseek, api_base: https://api.deepseek.com/v1,                model: deepseek-chat }
# 通义千问
ai: { provider: qwen,     api_base: https://dashscope.aliyuncs.com/compatible-mode/v1, model: qwen-turbo }
# Ollama（本地部署）
ai: { provider: ollama,   api_base: http://localhost:11434/v1,                  model: llama3 }
```

## 🏗️ 技术栈

**后端** Go 1.22 · Gin · GORM + SQLite (WAL) · Zap · Viper · gorilla/websocket · fsnotify · FFmpeg

**Web 前端** React 18 · TypeScript · Vite · Tailwind CSS · Zustand · HLS.js · React Router · Framer Motion

**Android V2** Kotlin · Jetpack Compose · Media3 · Paging 3 · WorkManager · Hilt · Retrofit · Android Keystore

**部署** Docker (Alpine 3.19) · docker-compose

## 🗺️ 路线图

- ✅ **v0.1 – v0.9** 核心播放、刮削、多用户、AI 助手、文件管理、Pulse 动态、分享、标签
- ✅ **v0.9.5** 完整 Emby API 兼容层（Infuse / Kodi / Emby 原生客户端）
- 🔄 **v1.0** ABR 无缝码率切换、FFmpeg 节流、移动端响应式、PWA、测试覆盖率 > 60%、Prometheus 监控
- 🧪 **Android V2 RC1** 原生媒体库、搜索、播放、下载、迁移与正式签名发布验证
- 🚀 **v1.1+** 4K/HDR、WebDAV、分布式转码、AV1、插件市场

## 💬 交流与反馈

- **QQ 群**：`1093473044`
- **Issues**：欢迎在 GitHub 上提交问题与建议
- **Android V2 问题**：请使用 [RC1 缺陷模板](./clients/android-v2/SMOKE_TEST.md#缺陷记录模板)，不要公开 Token、密钥或私人服务器地址

## ☕ 赞赏支持

如果这个项目对你有帮助，欢迎请作者喝杯咖啡 / 买个键盘 / 修个 Bug 🙏

<p align="center">
  <img src="./weixin.jpg" alt="微信赞赏码" width="260">
  <br>
  <i>Drug 的赞赏码 — "支持作者买键盘 / 修 Bug"</i>
</p>

## 📜 开源协议

本项目采用 [GNU General Public License v3.0](./LICENSE)（GPL-3.0）开源协议发布。

你可以自由地运行、研究、修改和分发本软件。基于本项目的任何派生作品在对外分发时，**必须同样以 GPL-3.0 协议开源**，并保留原作者版权声明。本软件按"原样"提供，不附带任何明示或默示的担保。
