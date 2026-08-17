# Nowen Video 飞牛 fnOS `.fpk` 发布

该目录参考 `nowen-note` 已验证的 FPK 发布契约，用同一个正式 Docker 镜像生成飞牛安装包。

## 前置条件

- Node.js 20+
- `fnpack`：把官方二进制放到仓库根目录，或设置 `FNPACK_BIN=/absolute/path/to/fnpack`
- 正式发布时 Docker 镜像为 `cropflre/nowen-video:vX.Y.Z`

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

## 正式发版

推荐从 `main` 执行：

```bash
./scripts/release.sh
```

默认正式发布目标为：

- Docker `linux/amd64 + linux/arm64`
- Android APK + AAB
- 飞牛 fnOS `.fpk`
- Git tag + GitHub Draft Release

FPK 的 manifest 使用纯 `X.Y.Z`；包内 compose 镜像使用 `vX.Y.Z`，与 Docker Hub 实际 tag 保持严格一致。Android 和 FPK 使用不同 checksum 文件，避免 Release 资产互相覆盖。

## fnOS 数据目录

安装后创建并挂载：

- `nowen-video/data` → `/data`
- `nowen-video/cache` → `/cache`
- `nowen-video/media` → `/media`

首次启动后可在 Nowen Video 中把 `/media` 添加为媒体库目录。

> 当前 FPK manifest 为 `x86_64`，与 nowen-note 的 fnOS 第三方应用基线保持一致。Docker 正式镜像本身仍同时发布 amd64/arm64。
