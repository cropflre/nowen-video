# 弄文影视飞牛 fnOS 安装包

应用中心和桌面入口显示「弄文影视」。同一份 `.fpk` 支持 AMD64（x86_64）与 ARM64 设备，安装时由 Docker 按 NAS 架构选择镜像。

官方文档：<https://developer.fnnas.com/docs/cli/fnpack/>

## 前置条件

- Node.js 20+
- 本地单独打包默认使用 `cropflre/nowen-video:vX.Y.Z`
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

通过 `FNPACK_CACHE_DIR` 可自定义缓存目录，通过 `FNPACK_TOOL_VERSION` 可临时覆盖工具版本。发版流水线固定使用 `1.2.3`，以便复现构建。

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
dist-fpk/弄文影视-1.2.6.fpk
dist-fpk/SHA256SUMS-fpk.txt
```

脚本按官方文档使用：

```bash
fnpack build --directory <path>
```

模板同时保留官方打包检查要求的 `manifest`、`config/privilege`、`config/resource`、`ICON.PNG`、`ICON_256.PNG`、`app/`、`cmd/`、`wizard/` 和桌面 UI 目录。应用中心读取 `manifest` 中的 `desc`。两个 PNG 由 `web/public/assets/nowen-video.svg` 分别以 64 和 256 像素导出，打包时也用于飞牛桌面入口。

## GitHub 发版

推送 `vX.Y.Z` 或 `vX.Y.Z-rc.N` 版本标签后，`release-server` 自动构建 Linux AMD64 和 ARM64 服务端归档及飞牛通用包，发布同版本的多架构 GHCR 镜像。飞牛包使用 `ghcr.io/cropflre/nowen-video:X.Y.Z` 这类固定版本镜像。流水线验证镜像包含 AMD64 和 ARM64，随后才上传安装包和校验文件。

GitHub Release 以「弄文影视-X.Y.Z.fpk」作为下载项的显示标签，下载文件使用稳定的 `nowen-video-X.Y.Z.fpk` 文件名。对应 Actions 产物 `nowen-video-fnos-release` 内含中文文件名的原始 `.fpk`。两处均附带适用于各自文件名的 `SHA256SUMS-fpk.txt`。

推荐从 `main` 执行：

```bash
./scripts/release.sh
```

脚本等待 GitHub Actions 打包并核对下载的 FPK 的 SHA256。`--no-fpk` 跳过本地脚本的 FPK 核验，版本标签仍会触发 GitHub Actions 打包。

## fnOS 数据目录

安装后创建并挂载：

- `nowen-video/data` → `/data`
- `nowen-video/cache` → `/cache`
- `nowen-video/media` → `/media`

首次启动后可在弄文影视中把 `/media` 添加为媒体库目录。
