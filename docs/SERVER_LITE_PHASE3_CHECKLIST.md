# Server Lite 第三阶段验收清单

## 统一任务中心

- [ ] 管理员登录 Lite 后右上角显示任务入口
- [ ] 普通用户不显示任务入口
- [ ] Full 模式不显示 Lite 任务抽屉
- [ ] 媒体库扫描开始后出现 `scan` 任务
- [ ] 扫描阶段、当前数量和总量持续更新
- [ ] 刮削任务显示数据源、进度和失败原因
- [ ] 转码任务显示清晰度、进度和错误信息
- [ ] WebSocket 断开时，30 秒轮询仍能刷新
- [ ] `GET /api/admin/tasks?active=true` 只返回活动任务
- [ ] 非管理员访问任务接口返回权限错误

## 播放规划器

- [ ] MP4 + H.264 + AAC 返回 `direct`
- [ ] MKV + H.264 + AAC 返回 `remux`
- [ ] DTS / TrueHD 等浏览器不兼容音频返回 `transcode`
- [ ] HEVC 且客户端未声明 HEVC 能力返回 `transcode`
- [ ] HEVC 且客户端声明支持、容器兼容时返回 `direct` 或 `remux`
- [ ] STRM 返回服务端代理 `direct`
- [ ] `force_transcode=true` 强制返回 `transcode`
- [ ] `max_bitrate` 正确附加到实时 HLS 地址
- [ ] Remux 播放失败后 Web 能自动降级到 HLS
- [ ] 旧 Full 服务没有 `/plan` 时 Web 自动使用原播放逻辑

## 兼容与回归

- [ ] Android V2 登录、媒体库、搜索和播放契约不变
- [ ] Lite 与 Full 使用同一 SQLite 数据目录可切换
- [ ] Lite 不创建 Full-only 数据表
- [ ] Lite Docker 不包含 Python
- [ ] Full Docker 继续构建并包含历史高级能力
