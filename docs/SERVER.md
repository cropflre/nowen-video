# Nowen Video 服务端

Nowen Video 现在只有一个对外正式服务端版本。

此前名为 **Server Lite** 的 NAS 核心架构已经正式扶正，成为 Nowen Video 默认且唯一的正式发行服务端。`Lite` 不再是产品版本、镜像版本或用户可选版本。

## 正式版定位

正式版面向 NAS、自托管和家庭影音场景，默认提供：

- 本地电影、电视剧、合集媒体库
- 扫描、文件监听、FFprobe、NFO 与多源元数据
- 搜索、推荐、收藏、历史、播放列表和个人统计
- 直接播放、Remux、按需 HLS 转码与硬件加速
- 外挂/内嵌字幕与在线字幕搜索
- 多用户、媒体库权限和内容分级
- TMDb、豆瓣、Bangumi、TheTVDB、Fanart.tv 等元数据能力
- WebDAV、Alist、S3 等按配置启用的存储能力
- Web、桌面端、Android V2 核心 API
- 扫描、刮削、转码统一任务中心
- 服务端播放规划器
- 可选 AI 能力及待重启状态管理

为降低 NAS 常驻资源占用，已经退役或不再进入正式运行时的历史模块不会因为“旧版曾经存在”而继续作为正式版卖点。

## 默认构建与运行

正式版统一使用标准命令：

```bash
make build
make dev
make run
make docker
```

构建产物：

```text
bin/nowen-video
```

默认 Dockerfile 生成的也是正式 Nowen Video 镜像，不再使用 `lite` 镜像标签。

## 内部实现路径

当前正式服务端仍暂时由以下内部入口承载：

```text
cmd/server-lite
internal/service/lite.go
internal/handler/lite.go
AutoMigrateLite
```

这些名称属于历史实现和迁移兼容边界，**不代表对外仍存在 Lite 产品版本**。

暂时保留它们的原因是避免一次性重命名破坏：

- SQLite 迁移与已有部署升级
- 回滚协议与历史验证脚本
- Web / Desktop / Android 对能力契约的兼容判断
- CI 中针对历史迁移边界的回归测试

后续只有在兼容窗口关闭、旧客户端确认退出后，才可以单独进行内部符号与目录重命名。该重命名不得改变数据库语义或重新引入已退役模块。

## 能力契约

正式版继续公开：

- `GET /api/capabilities`
- `GET /api/health`

能力字段区分实际运行状态和持久化配置状态：

- `available`
- `enabled`
- `configured`
- `configurable`
- `requires_restart`
- `pending_restart`
- `mode`

当前协议中的历史 `profile=lite` 值暂时作为客户端兼容标识保留。Web 用户界面不会再显示 `Lite`，而是显示 **正式版**。

在修改该协议值前，必须先确认旧 Web、桌面端和 Android 客户端不会把未知 profile 错判成旧 Full 服务，从而重新展示不可用功能。

## 旧版兼容运行时

旧的 `cmd/server` / `Dockerfile.full` 不再与正式版并列发布，只保留用于：

- 数据迁移验证
- 必要回滚
- 历史能力兼容测试

兼容命令仍可使用：

```bash
make build-full
make dev-full
docker build -f Dockerfile.full -t nowen-video:legacy .
```

这些产物不得使用 `latest`、正式 Release 主资产或正式部署文档进行分发。

## CI

正式服务端 CI 工作流为：

```text
.github/workflows/server-ci.yml
```

工作流名称为 **Server CI**，在 `main` 推送时验证：

- Go 全包与关键转码/播放契约
- 正式服务端构建
- Web typecheck / production build
- 正式 Docker 镜像构建与持久卷重启烟测
- 旧版兼容运行时的最小回归验证

CI 中对旧版运行时的验证只用于保证升级/回滚安全，不表示存在两个正式产品版本。

## 发布原则

从本次扶正开始：

1. 对外统一名称为 **Nowen Video**。
2. Release、Docker、README、UI 和部署文档不再使用 `Lite` 作为版本名称。
3. 默认 Dockerfile、`make build`、`make dev` 均指向正式版。
4. 旧 Full 服务仅作为 legacy compatibility / rollback 实现存在。
5. 内部 `lite` 符号只有兼容理由，没有产品语义。
6. 不为“去掉 Lite 字样”而修改历史数据库迁移语义。
