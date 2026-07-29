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

Lite 不启动：

- 音乐库、图片库、服务器联邦、插件系统
- Pulse
- 视频/字幕预处理 worker 和 GPU 常驻监控
- AI 场景、精彩片段、封面分析
- Emby/Infuse 兼容路由和 ID 预热
- 番号刮削、Python 子进程和调度器
- 多用户 Profile、服务端离线下载
- 评论与弹幕

`GET /api/health` 会返回 `profile: lite` 以及实际能力清单。

## Full（兼容）

完整服务仍保留在原来的 `cmd/server`，用于需要历史高级能力的部署：

```bash
make build-full
make dev-full

docker build -f Dockerfile.full -t nowen-video:full .
```

Full 镜像继续包含 Python 番号刮削依赖。Lite 镜像不安装 Python，也不复制刮削微服务源码。

## UI 调整

普通用户一级导航收敛为：

1. 首页
2. 影视库
3. 搜索
4. 我的

收藏、历史、播放列表、统计和个人设置统一进入“我的”。管理员侧栏只保留“管理中心”。旧的预处理 URL 会兼容跳转到管理概览，不再作为独立产品入口。

## 兼容性

- 现有数据库仍使用同一份 SQLite 文件，Lite 与 Full 可以回退切换。
- 不删除旧表和高级功能数据。
- Web、桌面端、Android V2 依赖的登录、媒体库、搜索、播放、收藏和进度 API 保持不变。
- 需要 Emby/Infuse、预处理或番号功能时，使用 Full 构建。
