# PostgreSQL 多实例与共享 Artifact Store 认证

## 目标

本认证验证两个 Nowen Video 服务实例共享一个 PostgreSQL 数据库和同一 NFS/SMB Artifact Store 时，转码领域仍保持单一权威状态。

它不是把本地 SQLite 目录直接放到网络盘。多实例部署要求：

- 数据库使用 PostgreSQL；
- Artifact Store 使用所有实例可见的共享目录；
- 每个实例使用不同 `instance_id`；
- Job Lease、Storage Reservation、Artifact 状态、Cleanup Lease 和 Storage Incident 全部以数据库为准；
- 文件系统只保存 Workspace 与已发布 Artifact，不承担调度锁。

## 认证拓扑

```text
instance-a ─┐
            ├── PostgreSQL 16
instance-b ─┘

instance-a ─┐
            ├── shared Artifact Store
instance-b ─┘
```

CI 使用两个独立 `psql` 会话模拟两个服务进程。共享目录位于同一个 Runner 文件系统，用于验证所有权协议；生产可替换为支持同文件系统原子 rename 的 NFS/SMB 挂载。

## 不变量

### 1. Job Lease 单所有者

两个实例可以同时看到同一 queued Job，但只能有一个条件更新成功：

```sql
status = 'queued'
desired_state = 'running'
lease_expires_at IS NULL OR lease_expires_at <= now()
```

成功实例写入独立 `lease_token`。后续 Attempt、进度、发布和终态都必须携带该 Token。

### 2. Reservation 总账串行化

空间分配不能依赖每个实例的内存计数。所有实例更新同一个 Storage Ledger 行，使 PostgreSQL 对该行加写锁，然后在同一事务内计算活动承诺。

认证使用 1000 字节预算，同时提交两个 700 字节请求，最终只能保留一个活动 Reservation。

### 3. Artifact 发布单版本

不同实例的 Attempt 使用不同 Workspace。发布前必须再次验证 Job Lease。

只有当前 Lease 所有者可以将完整 Workspace 原子 rename 到不可变 Artifact 路径。旧 Worker 即使仍持有本地进程和 Workspace，也不能创建 published 数据库证据。

### 4. Cleanup Lease 单删除者

Artifact 进入 pending 后，两个实例可以同时尝试 Claim Cleanup。数据库只允许一个实例写入 `cleanup_token` 和 Lease 到期时间。

目录删除完成后，只有持有同一个 Token 的实例可以删除元数据。失去 Cleanup Lease 的实例不能把其他节点正在服务或已经替换的 Artifact 删除。

### 5. Incident 集群去重

同一存储错误由多个实例同时观察时，通过稳定 `active_key` 聚合为一个活动 Incident，并原子累加 occurrences。

恢复必须由完整共享目录写探针确认：

```text
create → write → sync → atomic rename → remove
```

恢复后清空 active key，但保留历史事件。未来再次故障会生成新的 Incident 周期。

## 共享存储要求

共享 Artifact Store 必须满足：

- 所有实例解析到同一逻辑目录；
- Workspace 与发布目标位于同一挂载和文件系统；
- 同文件系统 rename 具备原子语义；
- 文件创建、fsync、rename 和删除错误能返回给进程；
- 不使用对象存储挂载模拟 POSIX rename；
- 不把 SQLite 数据库文件放在共享 NFS/SMB 上作为多实例数据库。

若挂载只支持最终一致性或 rename 不是原子的，必须改用对象存储专用发布协议，不能复用当前 Artifact Store。

## 故障边界

- PostgreSQL 暂时不可用：不允许新 Claim、Reservation、发布提交或 Cleanup 元数据删除；
- 共享目录暂时不可写：Storage Incident 阻断新 Claim，已有 Lease 到期后由恢复器重新排队；
- Worker 在 rename 后、数据库提交前崩溃：启动恢复根据 publishing 状态与目标路径对账；
- Worker 在目录删除后、元数据删除前崩溃：Cleanup Lease 到期后，下一实例将不存在目录视为幂等成功并删除元数据；
- 节点时钟轻微偏差：Lease 判断使用数据库提交的时间值和带边界条件的更新；生产应保持 NTP，同一集群不接受明显失同步节点。

## CI

工作流：

```text
.github/workflows/transcode-postgres-shared-storage-cert.yml
```

执行脚本：

```text
.github/scripts/certify_postgres_shared_storage.sh
```

覆盖：

1. PostgreSQL 双会话 Job Claim；
2. Ledger 串行化 Reservation；
3. 共享 Workspace 单一原子发布；
4. Cleanup Lease 单所有者删除；
5. Storage Incident 去重、累计与写探针恢复；
6. 健康探针和 Artifact 文件零残留。

## 当前范围

本节点认证转码核心协议在 PostgreSQL 和共享目录上的并发语义。完整产品切换到 PostgreSQL 仍需数据库连接层、全部业务表迁移、SQLite 数据导入、部署配置和升级回滚方案；在这些工作完成前，默认 Lite/Full 运行模式仍使用现有 SQLite 单实例配置。
