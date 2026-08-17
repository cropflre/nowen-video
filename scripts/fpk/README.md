# Nowen Video 飞牛 fnOS `.fpk` 发布

该目录参考 `nowen-note` 已验证的 FPK 发布契约，并对齐飞牛官方 `fnpack` 文档，用同一个正式 Docker 镜像生成飞牛安装包。

官方文档：<https://developer.fnnas.com/docs/cli/fnpack/>

## 前置条件

- Node.js 20+
- 正式发布时 Docker 镜像为 `cropflre/nowen-video:vX.Y.Z`
- **无需预先手动安装 `fnpack`**：脚本找不到可用工具时会自动从飞牛官方 CDN 下载当前固定版本

当前自动下载的官方 `fnpack` 版本：`1.2.3`。

官方文档当前提供以下开发机组合：

- Windows x86 / amd64
- Linux x86 / amd64
- Linux ARM / arm64
- macOS Intel / amd64
- macOS Apple Silicon / arm64

## fnpack 自动发现与下载顺序

`build-fpk.mjs` 按以下顺序解析 `fnpack`：

1. `FNPACK_BIN=/absolute/path/to/fnpack` 显式指定
2. 系统 `PATH` 中已有的 `fnpack`
3. 仓库根目录中与当前系统/架构匹配的 `fnpack-*`
4. 用户缓存目录中已经自动下载过的官方工具
5. 从 `https://static2.fnnas.com/fnpack/` 自动下载飞牛官方工具

自动下载不会写入 Git 仓库，因此不会破坏正式发版要求的 clean working tree。

默认缓存目录：

- Linux: `${XDG_CACHE_HOME:-~/.cache}/nowen-video/fnpack`
- macOS: `~/Library/Caches/nowen-video/fnpack`
- Windows: `%LOCALAPPDATA%\NowenVideo\fnpack`

也可以用 `FNPACK_CACHE_DIR` 自定义缓存目录；用 `FNPACK_TOOL_VERSION` 临时覆盖工具版本。正式发布默认固定使用文档当前版本 `1.2.3`，避免发版当天自动漂移到未知 CLI 版本。

下载后脚本会先执行 `fnpack --help` 做可执行性检查，再进入正式打包；下载失败或平台未被官方文档支持时会立即终止，并提示使用 `FNPACK_BIN`。

## 单独打包

```bash
FPK_VERSION=1.2.6 \
FPK_IMAGE_TAG=v1.2.6 \
DOCKERHUB_REPO=cropflre/nowen-video \
node scripts/fpk/build-fpk.mjs
```

产物：

```text
dist-fpk/nowen-video-1.2.6.fpk
dist-fpk/SHA256SUMS-fpk.txt
```

脚本按官方文档使用：

```bash
fnpack build --directory <path>
```

模板同时保留官方打包检查要求的 `manifest`、`config/privilege`、`config/resource`、`ICON.PNG`、`ICON_256.PNG`、`app/`、`cmd/`、`wizard/` 和桌面 UI 目录。

## 正式发版

推荐从 `main` 执行：

```bash
./scripts/release.sh
```

默认正式发布目标为：

- Docker `linux/amd64 + linux/arm64`
- Android APK + AAB
- 飞牛 fnOS `.fpk`
- Git tag + GitHub Release

FPK 的 manifest 使用纯 `X.Y.Z`；包内 compose 镜像使用 `vX.Y.Z`，与 Docker Hub 实际 tag 保持严格一致。Android 和 FPK 使用不同 checksum 文件，避免 Release 资产互相覆盖。

## fnOS 数据目录

安装后创建并挂载：

- `nowen-video/data` → `/data`
- `nowen-video/cache` → `/cache`
- `nowen-video/media` → `/media`

首次启动后可在 Nowen Video 中把 `/media` 添加为媒体库目录。

> 当前 FPK manifest 为 `x86_64`，与 nowen-note 的 fnOS 第三方应用基线保持一致。Docker 正式镜像本身仍同时发布 amd64/arm64。
