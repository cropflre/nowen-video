# nowen-video

> 你的私人家庭影音中心 🎬

一个基于 **Go + React** 构建的轻量级家庭媒体服务器，类似 Emby / Jellyfin，专为 NAS 部署优化。  
单二进制 + SQLite，Docker 一键启动，零配置即可使用。

---

## ✨ 核心特性

### 🎬 媒体库管理
- 自动扫描目录中的视频文件（MKV / MP4 / AVI / MOV / WebM 等 9 种格式）
- 使用 FFprobe 自动提取视频编码、分辨率、时长等元数据
- 自动发现同目录下的外挂字幕文件（SRT / ASS / SSA / VTT）
- 支持自动发现海报图片（同名 JPG/PNG/WebP、poster、cover、folder）

### 📺 智能播放
- **直接播放** — MP4 / WebM / M4V 等浏览器兼容格式零延迟直接播放，支持 Range 请求（拖动进度条）
- **HLS 转码播放** — MKV / AVI 等不兼容格式自动按需转码为 HLS 自适应流，支持多码率（360p / 480p / 720p / 1080p）
- **智能模式选择** — 前端自动检测文件格式，优先直接播放，不兼容时走 HLS 转码
- 全功能播放器控制栏：播放/暂停、进度拖动、音量调节、全屏、画质切换
- 键盘快捷键：`空格/K` 播放暂停、`←→` 快进快退、`↑↓` 音量、`F` 全屏、`M` 静音

### ⚡ 硬件加速转码
- 自动检测可用硬件加速方式（启动时检测 FFmpeg 编码器能力）
- 支持 **Intel QSV** / **VAAPI** / **NVIDIA NVENC** / 软件编码（libx264 兜底）
- 可配置转码预设和并发任务数，NAS 低功耗设备友好
- 转码缓存机制，相同质量只需转码一次

### 🎨 TMDb / 豆瓣 元数据刮削
- 集成 TMDb（The Movie Database）API 主源
- 集成豆瓣作为补充源（TMDb 失败或信息不完整时自动回落豆瓣）
- 自动匹配电影/剧集，获取海报、简介、评分、类型标签
- 剧集合集级刮削 — 以合集名称搜索，元数据自动同步到各集
- 剧集合集级刮削 — 以合集名称搜索，元数据自动同步到各集
- 支持从文件名智能提取搜索关键词和年份
- 自动清理文件名中的 BluRay / x264 / 1080p 等标记
- API Key 可在管理后台在线配置，无需重启

### � 剧集合集识别
- 基于文件夹的剧集自动识别（每个子目录 = 一部剧集）
- 支持常见剧集命名格式：`S01E01` / `1x01` / `第01集` / `EP01` / `Episode 01` / `E01`
- 自动提取季号（Season）和集号（Episode）
- 支持 `Season XX` / `S01` / `第1季` / `Specials` 目录结构
- 支持层级目录（剧集名/Season XX/视频文件）和扁平化目录
- 自动创建剧集合集条目，保持播放顺序
- 季视图和全部剧集视图两种浏览模式
- 剧集详情页（Banner + 海报 + 简介 + 季切换 + 集列表）
- 下一集查询接口（用于连续播放）

### �🔤 字幕支持- 自动扫描外挂字幕文件（SRT / ASS / SSA / VTT / SUB）
- 读取视频内嵌字幕轨道信息（通过 FFprobe）
- 按需提取内嵌字幕为 WebVTT 格式供浏览器使用
- 从文件名自动检测字幕语言（中 / 英 / 日 / 韩等）

### 👨‍👩‍👧‍👦 多用户支持
- 家庭成员独立账号，支持管理员/普通用户角色
- 每用户独立的观看历史、播放进度记录
- 每用户独立的收藏夹
- 自定义播放列表（创建、添加、排序、删除）
- 播放进度自动上报（每 15 秒），换设备续播

### 📶 WebSocket 实时通知
- 扫描进度实时推送（新发现文件数、当前处理文件名）
- 元数据刮削进度实时推送（当前/总数、成功/失败计数、进度条）
- 转码进度实时推送（百分比进度 + FFmpeg 转码速度）
- 自动重连机制（断线后 3 秒自动重连，最多 10 次）
- 心跳保活（Ping/Pong，60秒超时断开）
- 活动日志流（管理后台实时展示最近 10 条操作记录）

### � 智能推荐
- 基于观看历史、收藏、类型偏好的个性化推荐

### 💻 投屏支持
- DLNA/Chromecast 设备发现与控制

### �🛡️ 安全认证
- JWT Token 认证，支持 Token 刷新
- 请求头 Bearer Token + URL Query Token 双模式（视频流场景兼容）
- CORS 跨域中间件
- 管理员权限独立守护（中间件级别）
- bcrypt 密码加密存储

### 🖥️ 管理后台
- 系统状态仪表板（CPU / 内存 / 协程数 / Go 版本 / 硬件加速状态）
- 媒体库 CRUD + 一键扫描
- **WebSocket 实时进度面板** — 扫描/刮削/转码进度条实时更新
- 活动日志流（最近 10 条操作实时滚动展示）
- 转码任务监控（状态、进度、速度）
- 用户管理（列表、删除）
- TMDb API Key 在线配置（配置 / 修改 / 清除 / 掩码展示）
- WebSocket 连接状态指示器（实时连接 / 未连接）

### 🪶 轻量部署
- 后端单二进制，无外部依赖（除 FFmpeg）
- SQLite 数据库 + WAL 模式（高性能单文件存储）
- Docker 多阶段构建，最小运行镜像
- 支持环境变量 / 配置文件 / 命令行多种配置方式
- 前端构建产物内嵌，单端口同时服务 API + 静态文件

---

## 📸 功能页面

| 页面 | 说明 |
|------|------|
| 首页 | 继续观看 + 最近添加 |
| 媒体库 | 按媒体库浏览内容（支持剧集合集视图） |
| 剧集详情 | 剧集合集页，季视图/全部视图切换 |
| 媒体详情 | 海报、简介、评分、编码信息、播放按钮 |
| 播放器 | 全屏沉浸式播放，支持直接播放/HLS双模式 |
| 搜索 | 关键词搜索全部媒体 |
| 收藏夹 | 收藏的媒体列表 |
| 观看历史 | 观看记录，支持清除 |
| 播放列表 | 自定义播放列表管理 |
| 管理后台 | 系统状态、媒体库管理、用户管理、TMDb 配置 |
| 登录/注册 | 用户认证 |

---

## 🚀 快速开始

### Docker 部署（推荐）

```bash
# 1. 克隆项目
git clone https://github.com/your-repo/nowen-video.git
cd nowen-video

# 2. 修改配置
cp config.example.yaml data/config.yaml
# 编辑 data/config.yaml，修改 jwt_secret 等配置

# 3. 修改 docker-compose.yml 中的媒体目录挂载路径
#    将 /volume1/video 改为你的实际媒体目录

# 4. 启动
docker-compose up -d

# 5. 访问 http://你的NAS地址:8080
#    默认管理员: admin / admin123
```

### 本地开发

```bash
# 前置要求: Go 1.22+, Node.js 20+, FFmpeg

# 1. 后端
go mod tidy
cp config.example.yaml config.yaml  # 按需修改配置
go run ./cmd/server

# 2. 前端（另一个终端）
cd web
npm install
npm run dev

# 3. 访问 http://localhost:3000 （Vite 自动代理 API 到 :8080）
#    默认管理员: admin / admin123
```

### 生产构建

```bash
# 构建前端
cd web && npm run build && cd ..

# 构建后端（内嵌前端静态文件）
go build -o nowen-video ./cmd/server

# 运行
./nowen-video
# 访问 http://localhost:8080
```

---

## 📁 项目结构

```
nowen-video/
├── cmd/server/main.go        # Go 入口（路由注册、依赖注入）
├── internal/
│   ├── config/                # 配置管理（Viper，支持 YAML + 环境变量）
│   ├── handler/               # HTTP 处理器层
│   │   ├── auth.go            #   登录 / 注册 / Token刷新
│       ├── media.go           #   媒体列表 / 详情 / 搜索
│       ├── series.go          #   剧集合集列表 / 详情 / 季视图 / 下一集
│       ├── library.go         #   媒体库 CRUD + 扫描│   │   ├── stream.go          #   直接播放 / HLS流 / 海报
│   │   ├── subtitle.go        #   字幕轨道 / 提取 / 外挂
│   │   ├── metadata.go        #   TMDb 元数据刮削
│   │   ├── playlist.go        #   播放列表管理
│   │   ├── user.go            #   用户信息 / 进度 / 收藏 / 历史
│   │   └── admin.go           #   系统管理 / TMDb配置
│   ├── middleware/             # 中间件（JWT认证、CORS、管理员权限）
│   ├── model/                 # GORM 数据模型（8张表，自动迁移）
│   ├── repository/            # 数据访问层
│   └── service/               # 业务逻辑层
│       ├── auth.go            #   JWT 认证服务
│       ├── media.go           #   媒体查询服务
│       ├── series.go          #   剧集合集服务（季视图/集视图/下一集）
│       ├── scanner.go         #   文件扫描 + FFprobe + 剧集合集识别 + 字幕发现
│       ├── stream.go          #   流媒体服务（直接播放 + HLS + 海报）
│       ├── transcode.go       #   FFmpeg 转码（硬件加速 + 工作协程池 + 进度解析）
│       ├── metadata.go        #   TMDb API 集成 + 豆瓣补充源 + 剧集合集刮削
│       ├── douban.go          #   豆瓣元数据刮削服务
│       ├── ws_hub.go          #   WebSocket Hub（连接管理 + 事件广播 + 心跳）
│       ├── library.go         #   媒体库管理（扫描 + 刮削 + 剧集合集刮削）
│       ├── playlist.go        #   播放列表服务
│       └── user.go            #   用户服务
├── web/                       # React 前端
│   ├── src/
│   │   ├── api/               #   Axios 客户端 + API 封装
│   │   ├── components/        #   通用组件（Layout / VideoPlayer / MediaCard 等）
│   │   ├── hooks/             #   自定义 Hooks（useWebSocket 等）
│   │   ├── pages/             #   11 个页面（首页 / 播放器 / 剧集详情 / 管理 / 搜索等）
│   │   ├── stores/            #   Zustand 状态管理（auth / player）
│   │   └── types/             #   TypeScript 类型定义
│   ├── tailwind.config.js     #   暗色主题 Tailwind 配置
│   └── vite.config.ts         #   Vite 配置（路径别名 + API代理）
├── Dockerfile                 # 多阶段构建（Node → Go → Alpine）
├── docker-compose.yml         # NAS 部署配置（含硬件加速设备挂载）
└── config.example.yaml        # 配置模板
```

---

## 🔌 API 接口总览

### 公开接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 用户登录 |
| POST | `/api/auth/register` | 用户注册 |
| POST | `/api/auth/refresh` | 刷新 Token |

### 认证接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/libraries` | 获取媒体库列表 |
| POST | `/api/libraries` | 创建媒体库 🔒 |
| POST | `/api/libraries/:id/scan` | 扫描媒体库 🔒 |
| DELETE | `/api/libraries/:id` | 删除媒体库 🔒 |
| GET | `/api/media` | 媒体列表（分页） |
| GET | `/api/media/:id` | 媒体详情 |
| GET | `/api/media/recent` | 最近添加 |
| GET | `/api/media/continue` | 继续观看 |
| GET | `/api/series` | 剧集合集列表（分页） |
| GET | `/api/series/:id` | 剧集合集详情（含所有剧集） |
| GET | `/api/series/:id/seasons` | 季列表（季视图） |
| GET | `/api/series/:id/seasons/:season` | 指定季的剧集列表（集视图） |
| GET | `/api/series/:id/next` | 下一集（连续播放） |
| GET | `/api/media/:id/poster` | 获取海报图片 |
| GET | `/api/search` | 搜索媒体 |
| GET | `/api/stream/:id/info` | 获取播放信息 |
| GET | `/api/stream/:id/direct` | 直接播放视频文件 |
| GET | `/api/stream/:id/master.m3u8` | HLS 主播放列表 |
| GET | `/api/stream/:id/:quality/:segment` | HLS 分片 |
| GET | `/api/subtitle/:id/tracks` | 字幕轨道列表 |
| GET | `/api/subtitle/:id/extract/:index` | 提取内嵌字幕 |
| GET | `/api/subtitle/external` | 获取外挂字幕文件 |
| POST | `/api/media/:id/scrape` | 刮削元数据（TMDb + 豆瓣） 🔒 |
| GET | `/api/users/me` | 当前用户信息 |
| PUT | `/api/users/me/progress/:mediaId` | 更新播放进度 |
| GET/POST/DELETE | `/api/users/me/favorites` | 收藏管理 |
| GET/DELETE | `/api/users/me/history` | 观看历史 |
| GET/POST/DELETE | `/api/playlists` | 播放列表管理 |
| GET | `/api/ws` | WebSocket 实时通知 |

### 管理接口 🔒

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/users` | 用户列表 |
| DELETE | `/api/admin/users/:id` | 删除用户 |
| GET | `/api/admin/system` | 系统信息 |
| GET | `/api/admin/transcode/status` | 转码任务状态 |
| GET/PUT/DELETE | `/api/admin/settings/tmdb` | TMDb API Key 配置 |

> 🔒 = 需要管理员权限

---

## ⚙️ 配置说明

支持三种配置方式（优先级从高到低）：
1. **环境变量** — 前缀 `NOWEN_`，如 `NOWEN_PORT=9090`
2. **配置文件** — `config.yaml`（搜索路径：`.` → `./data` → `/etc/nowen-video`）
3. **默认值** — 内置合理默认值，零配置可运行

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| `port` | `NOWEN_PORT` | `8080` | 服务端口 |
| `debug` | `NOWEN_DEBUG` | `false` | 调试模式 |
| `data_dir` | `NOWEN_DATA_DIR` | `./data` | 数据目录 |
| `cache_dir` | `NOWEN_CACHE_DIR` | `./cache` | 转码缓存目录 |
| `web_dir` | `NOWEN_WEB_DIR` | `./web/dist` | 前端静态文件目录 |
| `db_path` | `NOWEN_DB_PATH` | `./data/nowen.db` | SQLite 数据库路径 |
| `jwt_secret` | `NOWEN_JWT_SECRET` | *(需修改)* | JWT 签名密钥 |
| `ffmpeg_path` | `NOWEN_FFMPEG_PATH` | `ffmpeg` | FFmpeg 可执行文件路径 |
| `ffprobe_path` | `NOWEN_FFPROBE_PATH` | `ffprobe` | FFprobe 可执行文件路径 |
| `hw_accel` | `NOWEN_HW_ACCEL` | `auto` | 硬件加速模式 |
| `vaapi_device` | `NOWEN_VAAPI_DEVICE` | `/dev/dri/renderD128` | VAAPI 设备路径 |
| `transcode_preset` | `NOWEN_TRANSCODE_PRESET` | `veryfast` | FFmpeg 转码预设 |
| `max_transcode_jobs` | `NOWEN_MAX_TRANSCODE_JOBS` | `2` | 最大并发转码数 |
| `tmdb_api_key` | `NOWEN_TMDB_API_KEY` | *(空)* | TMDb API Key |

---

## 🔧 硬件加速

| 模式 | 适用场景 | FFmpeg 编码器 | 典型设备 |
|------|---------|-------------|---------|
| `auto` | 自动检测（推荐） | — | 任何设备 |
| `qsv` | Intel 核显 | `h264_qsv` | 群晖 NAS (Celeron / Pentium / Core) |
| `vaapi` | Linux 通用 | `h264_vaapi` | Linux 带 Intel/AMD GPU |
| `nvenc` | NVIDIA 独显 | `h264_nvenc` | 有 NVIDIA 显卡的服务器 |
| `none` | 纯软件编码 | `libx264` | 无硬件加速设备（兜底） |

设置方式：配置 `hw_accel: auto` 自动检测，或手动指定具体模式。  
群晖 NAS 部署时确保已挂载 `/dev/dri` 设备。

---

## 📋 技术栈

| 组件 | 技术 | 版本 |
|------|------|------|
| 后端框架 | Go + Gin | Go 1.22 / Gin 1.9 |
| ORM | GORM + SQLite (WAL) | GORM 1.25 |
| 认证 | golang-jwt/jwt/v5 | 5.2 |
| WebSocket | gorilla/websocket | 1.5 |
| 日志 | Uber Zap | 1.27 |
| 配置 | Spf13 Viper | 1.18 |
| 前端框架 | React + TypeScript | React 18 / TS 5.4 |
| 构建工具 | Vite | 5.3 |
| CSS 框架 | Tailwind CSS | 3.4 |
| 状态管理 | Zustand | 4.5 |
| 播放器 | HLS.js | 1.5 |
| HTTP 客户端 | Axios | 1.7 |
| 图标库 | Lucide React | 0.379 |
| 转码引擎 | FFmpeg + FFprobe | — |
| 部署 | Docker (Alpine) | — |

---

## 🗄️ 数据模型

```
┌─────────────┐     ┌─────────────┐     ┌───────────────┐
│   User      │     │   Library   │     │     Series      │
├─────────────┤     ├─────────────┤     ├───────────────┤
│ id          │     │ id          │◄────│ library_id    │
│ username    │     │ name        │     │ title         │
│ password    │     │ path        │     │ folder_path   │
│ role        │     │ type        │     │ poster_path   │
│ avatar      │     │ last_scan   │     │ overview      │
└──────┬──────┘     └─────────────┘     │ season_count  │
       │                                │ episode_count │
       │                                └───────┬───────┘
       │                                        │
       │                                ┌───────┴───────┐
       │                                │    Media      │
       │                                ├───────────────┤
       │                                │ library_id    │
       │                                │ series_id     │ ◄ 剧集关联
       │                                │ title         │
       │                                │ file_path     │
       │                                │ media_type    │ ◄ movie/episode
       │                                │ season_num    │ ◄ 季号
       │                                │ episode_num   │ ◄ 集号
       │                                │ episode_title │ ◄ 单集标题
       │                                │ video_codec   │
       │                                │ resolution    │
       │                                │ poster_path   │
       │                                │ overview      │
       │                                │ rating        │
       │                                └───────┬───────┘
       │  ┌──────────────────┐          │
       ├──│  WatchHistory    │──────────┘
       │  │ position/duration│
       │  │ completed        │
       │  └──────────────────┘
       │
       │  ┌──────────────────┐     ┌────────────────┐
       └──│  Playlist        │───►│ PlaylistItem   │
          └──────────────────┘     │ media_id       │
                                        │ sort_order     │
          ┌──────────────────┐     └────────────────┘
          │ TranscodeTask    │
          │ media_id/quality │
          │ status/progress  │
          └──────────────────┘
```
---

## 📜 License

MIT
