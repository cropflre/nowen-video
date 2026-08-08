# Server Lite SQLite 升级与回滚协议

## 目标

Server Lite 默认继续使用单文件 SQLite。升级必须满足两个核心要求：

1. Full 历史数据库切换到 Lite 后，核心数据和 Full-only 历史数据都不能丢失；
2. 用户保留升级前备份后，必须可以重新切回 Full。

Lite 的数据库迁移只负责添加当前运行需要的核心结构，不删除表、不删除列、不清空高级功能记录。

## 支持的切换路径

```text
旧 Full 数据库
    ↓ 备份
新 Lite 启动
    ↓ 可继续使用同一 nowen.db
Full 回滚启动
```

默认数据库文件仍为：

```text
/data/nowen.db
```

SQLite 的 `-wal` 与 `-shm` 文件属于活动数据库的一部分。人工复制前必须先停止容器，或使用 SQLite 在线备份能力生成一致快照，不能只在写入过程中复制主文件。

## 自动认证拓扑

认证使用真实文件型 SQLite，而不是纯内存数据库：

1. 使用 Full 迁移创建数据库；
2. 写入核心用户、媒体库、媒体、观看历史；
3. 写入 Full-only 视频和字幕预处理记录；
4. 写入持久转码 Job、空间 Reservation 和已恢复存储 Incident；
5. 执行 WAL checkpoint 并生成升级前备份；
6. 对原数据库执行 Lite 迁移；
7. 验证所有核心数据与 Full-only 数据保持不变；
8. 验证 Lite 没有重写 Full-only 表定义；
9. 再执行 Full 迁移，验证同一数据库可回滚；
10. 从升级前备份恢复出另一份数据库，再次验证 Full 可打开；
11. 每个阶段执行 `PRAGMA integrity_check`。

测试文件：

```text
internal/model/migrate_lite_roundtrip_test.go
```

独立门禁：

```text
.github/workflows/server-lite-sqlite-upgrade-cert.yml
```

## 被保护的数据

自动认证至少覆盖：

- 用户账号、密码哈希和角色；
- 媒体库路径；
- 媒体文件和技术摘要；
- 观看进度；
- Full 视频预处理任务；
- Full 字幕预处理任务；
- 持久转码 Job；
- 转码空间 Reservation；
- Storage Incident 历史。

未来新增 Lite 核心持久化表或 Full-only 表时，应同步扩展该认证，而不是只修改迁移列表。

## 新安装 Lite 的边界

全新 Lite 数据库只允许创建：

- Lite 核心业务表；
- 启动时明确开启的可选能力表；
- 转码执行、Artifact、Reservation 和 Storage Incident 等核心运行表。

AI 关闭时，不创建 AI 缓存、AI 用量和 AI 分析表。Lite 也不创建视频预处理、字幕预处理、场景章节、精彩片段等 Full-only 表。

自动认证会检查这些 Full-only 表在全新 Lite 数据库中不存在。

## 升级操作建议

升级前：

```bash
docker stop nowen-video
cp /path/to/data/nowen.db /path/to/backup/nowen-before-lite.db
docker start nowen-video
```

若目录中存在 `nowen.db-wal`，说明数据库可能没有完成 checkpoint。应保持容器停止后再复制整套数据库文件，或先通过 SQLite 工具执行一致备份。

升级后应检查：

- 管理员可以登录；
- 媒体库和媒体数量正确；
- 观看历史和收藏存在；
- 扫描、播放与转码可用；
- Task Center 没有异常迁移任务；
- `PRAGMA integrity_check` 返回 `ok`。

## 回滚协议

### 直接切回 Full

Lite 不删除 Full-only 表，因此正常情况下可以让 Full 直接打开同一份数据库。

### 使用升级前备份

发生不可接受的问题时：

1. 停止 Lite；
2. 保留故障现场数据库用于排查；
3. 将升级前备份恢复为 `nowen.db`；
4. 启动原 Full 版本；
5. 验证登录、媒体库、播放和高级任务。

不要在 Lite 和 Full 同时运行时共享同一个 SQLite 文件。SQLite 方案只支持单服务实例。

## 明确不做的事情

Lite 迁移不得：

- 删除 Full-only 表；
- 清空旧高级功能数据；
- 将未知表视为垃圾表；
- 自动降级或重写用户数据；
- 在没有一致备份的情况下执行破坏性迁移；
- 把 SQLite 文件放到多实例共享 NFS/SMB 上并同时写入。

## 发布门禁

以下任一情况失败时，Server Lite 不应进入正式发布：

- Full → Lite 后数据指纹变化；
- Full-only 表被删除或表定义被 Lite 重写；
- Lite → Full 无法重新迁移；
- 升级前备份无法被 Full 打开；
- 全新 Lite 创建了未启用的 Full-only 表；
- SQLite `integrity_check` 不是 `ok`。
