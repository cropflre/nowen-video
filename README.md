# Nowen Video（弄文影视）—— 个人 fork

这是 **[cropflre/nowen-video](https://github.com/cropflre/nowen-video)** 的个人 fork，用于在自建 NAS 上部署，
并补上官方 Docker 镜像缺失的 Emby 客户端兼容能力。

> **上游项目主页（完整功能说明、开发文档、Release 安装包都在那里）**
> 👉 **https://github.com/cropflre/nowen-video**

## 这个 fork 改了什么

| 改动 | 说明 |
| --- | --- |
| **修复配置写盘路径** | 容器里保存配置原本会 `permission denied` 而静默失败；现在正确写入 `/data/config.yaml`，重启后配置不丢。 |
| **把 Emby 兼容层挂到 server-lite** | 上游的 Emby 兼容层（`internal/handler/emby/`）只挂在 `cmd/server` 上，而官方 Docker 镜像构建的是 `cmd/server-lite`，所以官方镜像默认没有任何 Emby 端点。 |
| **补齐 Emby 端点** | 客户端能力查询、搜索、媒体库等接口补齐；根前缀与 `/emby` 双挂载，JWT 兼作 Emby token。 |
| **UDP 7359 局域网自动发现** | Emby / Jellyfin / Infuse / Hills / Yamby 可在同一局域网内自动搜到服务器。 |
| **HEVC 直接播放** | 识别 Hills / Yamby 客户端，HEVC 片源走直接播放而非转码。 |
| **精简 compose** | `docker-compose.yml` 只保留部署必需项，写法与上游 README 对齐。 |

为保持仓库精简，本 fork 还删除了上游根目录里误提交的编译产物（`server` / `server-lite`，共 58 MB）、
三份无引用的开发草稿（`PR11_*.md`）、作者个人赞赏码以及重复的英文 README 与 compose 文件。
**源码树（`cmd/` `internal/` `web/` `android/` `desktop/` 等）完整保留。**

## 部署

```bash
docker compose up -d
```

把 `docker-compose.yml` 里的 `/volume1/Media` 改成你自己的媒体目录即可。
然后访问 `http://<主机IP>:8080` —— **第一个注册的用户自动成为管理员**。

镜像 `ghcr.io/grimm-yh/nowen-video:fix-emby-compat` 已公开，无需登录即可拉取（linux/amd64）。

## 上游 PR

| PR | 内容 |
| --- | --- |
| [#90](https://github.com/cropflre/nowen-video/pull/90) | Emby 客户端兼容 + 配置写盘路径修复 |
| [#91](https://github.com/cropflre/nowen-video/pull/91) | docker-compose 文档修正 |

## 与上游同步

```bash
git remote add upstream https://github.com/cropflre/nowen-video.git
git fetch upstream
git merge upstream/main
```

## 许可

沿用上游的 [GPL-3.0](./LICENSE)。
