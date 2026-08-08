# Nowen Video 服务端精简模式

`nowen-video` 现在提供两个明确的服务端组合，避免所有能力默认集中启动。

## Lite（默认）

默认 `Dockerfile`、`make build`、`make dev` 使用 `cmd/server-lite`。

Lite 保留：

- 本地电影、电视剧、合集媒体库
- 扫描、文件监听、FFprobe、NFO 和多源元数据
- 搜索、推荐、收藏、历史、播放列表和个人统计
- 直接播放、Remux、按需 HLS 转码、硬件加速
- 外挂/内嵌字幕与在线字幕搜索
- 多用户、媒体库权限、内容分级
- 文件管理、手动匹配、批量刮削
- TMDb、豆瓣、Bangumi、TheTVDB、Fanart.tv
- WebDAV、Alist、S3（仅配置后初始化）
- Web、桌面端、Android V2 核心 API
- 扫描、刮削、转码统一任务中心
- 服务端播放规划器

Lite 不启动：

- 音乐库、图片库、服务器联邦、插件系统
- Pulse 页面、接口和运行组件
- 视频/字幕预处理 worker 和 GPU 常驻监控
- AI 场景、精彩片段、封面分析
- Emby/Infuse 兼容路由和 ID 预热
- 番号刮削、Python 子进程和调度器
- 多用户 Profile、服务端离线下载
- 评论、弹幕、DLNA/Chromecast 投屏

## 能力契约

Lite 和 Full 都通过公开接口声明真实运行能力：

- `GET /api/capabilities`：稳定的类型化能力清单
- `GET /api/health`：健康信息，以及兼容旧客户端的 `features`

Lite 的 `/api/health` 同时包含类型化能力清单。Full 通过独立的公开能力提供器原生响应 `/api/capabilities`，不再依赖 SPA 回退或前端猜测服务类型。旧版服务端仍可由 Web 通过 `/api/health` 兼容识别。

能力字段含义：

- `available`：当前构建是否包含该能力
- `enabled`：本次进程中是否真正可用
- `configured`：当前持久化配置是否要求启用
- `configurable`：管理员是否可以配置
- `requires_restart`：改变启停状态后是否需要重启
- `pending_restart`：配置状态和实际运行状态是否不一致
- `mode`：`core`、`optional`、`on_demand`、`full_only`、`lite_only` 或 `full`

`enabled` 与 `configured` 被刻意分开。例如 AI 从关闭改为开启后，接口会返回：

```json
{
  "available": true,
  "enabled": false,
  "configured": true,
  "requires_restart": true,
  "pending_restart": true
}
```

这可以防止 Web、桌面端或移动端在服务尚未重启时误调用尚未注册的运行接口。兼容旧客户端的 `features.ai_enabled` 同样只反映实际运行状态。

Web 客户端启动时会读取能力清单。管理员可在“我的”页面查看当前 Lite/Full 模式、已启用扩展和待重启配置。AI 配置保存后客户端会立即重新读取能力状态。

### AI 生命周期

AI 在 Lite 中属于可选能力：

- AI 关闭时，状态、配置、连接测试和提供商预设接口仍可访问
- AI 缓存、用量、路由、AI 搜索和 AI 重命名等运行接口只在启动时已开启 AI 的情况下注册
- 从关闭改为开启后需要重启服务；重启时会完成 AI 表迁移、运行路由和 AIRouter 装配
- 从开启改为关闭后会立即停止向客户端声明 AI 可用，并提示重启完成运行组件回收

这样既保留了管理端开启入口，也避免关闭 AI 时创建相关表或启动运行组件。

### 管理端能力保护

共享管理页面会按照能力清单修正交互：

- Lite 不展示“扫描后自动预处理”，避免保存一个当前构建永远不会执行的设置
- AI 配置与实际运行状态不一致时，AI 标签页顶部会显示待重启原因
- 开启 AI 后提示重启完成路由、数据表和后台组件装配
- 关闭 AI 后提示重启完成运行组件回收
- 提供“重新检测”入口，重新读取 `/api/capabilities`
- Full 保留自动预处理设置和高级管理入口

当前保护逻辑集中在 `CapabilityAdminGuard`，用于约束仍由 Lite/Full 共用的大型管理组件。后续管理页拆分后，可将能力判断下沉到具体设置区块。

## 统一任务中心

Lite 不再要求管理员分别进入媒体库、刮削管理和转码面板查看进度。

- `GET /api/admin/tasks` 返回统一任务快照
- 支持 `active=true` 只查询活动任务
- 支持 `limit=1..200` 控制最近任务数量
- 聚合媒体库扫描内存状态、刮削任务表和转码任务表
- 不增加新的数据库表，也不改变原有任务执行队列
- 统一任务类型：`scan`、`scrape`、`transcode`
- 统一状态：`queued`、`running`、`completed`、`failed`、`cancelled`

Lite 管理员页面右上角提供全局任务入口，任务数量和进度会通过现有 WebSocket 事件刷新，并以 30 秒轮询作为断线兜底。Full 保留原来的高级任务页面。

## 播放规划器

Lite 的常规播放信息接口已经是统一入口：

```text
GET /api/stream/:id/info
```

响应保留现有 `MediaPlayInfo` 字段，并额外返回 `playback_plan`。Web 客户端可以用一次请求同时获得媒体信息与服务端决策，不再额外请求规划接口。独立诊断接口仍保留：

```text
GET /api/stream/:id/plan
```

两个接口都支持以下客户端参数：

- `supports_direct`：是否支持原始文件直放
- `supports_remux`：是否支持 fragmented MP4 Remux
- `supports_hevc`：是否支持 HEVC 解码
- `force_transcode`：是否强制使用兼容转码
- `max_bitrate`：HLS 最大码率建议，单位 bit/s

规划器不会启动 FFmpeg，只返回播放方式、地址、原因和回退地址。Lite 的默认优先级为：

1. STRM 服务端代理直放
2. 原生直接播放
3. 零转码 Remux
4. HLS 转码

返回示例：

```json
{
  "media_id": "media-id",
  "can_direct_play": false,
  "can_remux": true,
  "playback_plan": {
    "method": "remux",
    "url": "/api/stream/media-id/remux",
    "reason_code": "container_remux",
    "reason": "编码兼容，仅需转换容器，无需重新编码",
    "requires_transcode": false,
    "fallback_method": "transcode",
    "fallback_url": "/api/stream/media-id/master.m3u8"
  }
}
```

服务端会在同一次请求中复用已加载的媒体信息生成规划，避免重复读库。连接旧版或 Full 服务时，Web 自动退回历史播放信息逻辑。Android V2 的原有播放契约保持不变，额外字段不会改变现有调用。

## Pulse 完整移除

Pulse 已从 Lite 和 Full 的产品及运行时中删除：

- 新版侧边栏、页面组件和正常路由不再提供 Pulse 入口
- `/api/capabilities` 在 Lite 和 Full 中都不再返回 `pulse` 键
- Full 不再包含 `PulseRepo`、`PulseService`、`PulseHandler` 或对应聚合字段
- Full 不再注册任何 `/api/admin/pulse/*` 路由，旧请求自然返回普通 `404 Not Found`
- `internal/repository/repo_pulse.go`、`internal/service/pulse.go`、`internal/handler/pulse.go` 已删除
- 旧 `/pulse`、`/pulse/*` 页面地址仍统一跳转到 `/admin`，用于清理旧书签和旧应用壳
- Service Worker 升级为 v5，不再在安装阶段预缓存 `/` 旧应用壳
- 新 Worker 激活后删除旧缓存并主动重新导航已打开标签页
- `/sw.js` 在 Lite 和 Full 中均使用 `no-store`，每次启动主动检查更新
- 运行时语言词典屏蔽旧 `nav.pulse` 键，旧前端代码无法重新渲染该菜单文案
- 不执行破坏性数据库清理；Pulse 原先复用的播放统计和媒体数据继续服务现有统计功能
- `cmd/server/pulse_removed_test.go` 阻止旧路由或运行文件被重新引入

## Full（兼容）

完整服务仍保留在原来的 `cmd/server`，用于需要历史高级能力的部署：

```bash
make build-full
make dev-full

docker build -f Dockerfile.full -t nowen-video:full .
```

Full 镜像继续包含 Python 番号刮削依赖。Lite 镜像不安装 Python，也不复制刮削微服务源码。Pulse 在 Full 中同样已经完整删除。

Full 原生提供 schema v2 的 `/api/capabilities`。预处理、字幕预处理、Emby 兼容、番号、投屏、音乐、图片、联邦、插件、离线下载、多用户 Profile、评论、弹幕和 AI 场景会标记为 `full` 模式；统一任务中心标记为 `lite_only`。Pulse 不出现在能力清单中。

Web 客户端会通过能力契约自动适配服务端：Lite 隐藏预处理等高级入口；Full 会恢复文件管理、视频预处理和字幕预处理路由与侧栏入口。同一套前端构建产物可以连接两种服务端模式。

## UI 调整

普通用户一级导航收敛为：

1. 首页
2. 影视库
3. 搜索
4. 我的

收藏、历史、播放列表、统计和个人设置统一进入“我的”。Lite 管理员侧栏只保留“管理中心”；Full 根据能力清单恢复高级管理入口。旧的预处理 URL 在 Lite 中会兼容跳转到管理概览，在 Full 中继续正常打开。

## 兼容性

- 现有数据库仍使用同一份 SQLite 文件，Lite 与 Full 可以回退切换。
- 不删除旧表和高级功能数据。
- 新安装的 Lite 只迁移核心表和启动时已启用的可选能力表。
- Web、桌面端、Android V2 依赖的登录、媒体库、搜索、播放、收藏和进度 API 保持不变。
- 需要 Emby/Infuse、预处理、投屏或番号功能时，使用 Full 构建。
