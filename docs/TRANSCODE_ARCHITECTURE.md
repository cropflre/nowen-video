# Nowen Video 转码与全端秒播架构

本文档定义 Web、PC、Android 与 Emby/Infuse 共用的长期媒体执行架构。目标不是以临时前端隐藏、旁路 FFmpeg、共享可变目录或重复队列解决单点问题，而是建立可持续演进的播放决策、任务编排、资源治理、进程执行和产物管理边界。

## 产品目标

对服务端能够解析且未损坏、未加密的媒体：

1. 客户端不需要理解容器与编码差异。
2. 优先使用零转码路径，无法原生播放时自动降级。
3. Web、PC、Android 与兼容客户端使用同一份服务端播放规划。
4. 本地与 NAS 媒体以 P95 首帧时间作为主要性能指标。
5. 所有媒体进程均可取消、重试、诊断、限流和恢复。
6. 失去 Lease 的旧 Worker 在数据库和文件系统两个层面都不能污染新执行结果。

不能承诺 DRM、损坏文件、上游失效或网络不可用时固定秒开；这些情况必须返回明确诊断，而不是无限等待。

## 播放决策顺序

1. `direct`：容器、视频和音频均可由客户端稳定解码。
2. `remux`：音视频编码兼容，只转换为 fragmented MP4 容器。
3. `smart_remux`：视频 bit-for-bit copy，仅将不兼容音频转换为 AAC-LC。
4. `startup_stream`：使用预生成的兼容启动片段，后台衔接持续转码。
5. `transcode`：完整视频与音频兼容转码。
6. `preprocessed`：复用已经发布的完整长期产物。

当前已落地 direct、remux、smart_remux、runtime HLS，以及数据库 Job Claim / Lease、Priority + FIFO 调度、重启续队、优雅关闭、统一 Probe、统一质量目录和 Attempt 隔离 Artifact Store。`startup_stream` 与长期预处理迁移属于后续阶段，不应通过伪造缓存或短路播放状态代替。

## 领域边界

```text
Probe
  -> Playback Planner
  -> Job Orchestrator
  -> Resource Governor
  -> Executor / Process Runtime
  -> Artifact Store
  -> Client Playback Adapters
```

### Probe

统一缓存 ffprobe 结果，包括容器、所有流、真实帧率、位深、色彩信息、音轨、字幕、文件指纹和远程输入能力。

当前实现：

- 按源路径、大小、修改时间和 Probe Version 生成指纹。
- 单飞合并并发 FFprobe。
- 源文件变化自动失效。
- 提供不执行 FFprobe 的缓存读取接口，供低延迟播放规划使用。
- 真实 FPS 用于 GOP 计算。
- HDR 仅依据色彩传递函数或明确 side data 判断，不再根据 HEVC 编码名猜测。
- 扫描后 Probe Warmup 与父调度器使用同一生命周期。

### Playback Planner

只做纯决策，不启动 FFmpeg。输入媒体探测结果、客户端能力、系统策略和已有产物，输出版本化播放计划与 fallback。

### Job Orchestrator

`transcode_jobs` 是任务状态和排队顺序的唯一权威，负责幂等 Active Key、优先级、取消意图、领取、租约、心跳、重试与重启恢复。旧 `transcode_tasks` 在迁移期间仅作为现有管理 API 的兼容投影。

Worker 不再依赖进程内 Job 堆。它直接查询数据库中 `queued + desired_state=running` 的记录，按 `priority DESC, created_at ASC, id ASC` 选择候选，再通过带状态条件的原子 `UPDATE` Claim。多个服务实例可以看到同一候选，但只有一个实例能获得 Lease。

每次成功领取生成独立 `lease_token`。进入运行态、续租、优雅释放、Artifact 发布和提交终态都必须匹配该令牌。旧 Worker 丢失或释放 Lease 后，即使进程延迟返回成功，也不能覆盖新 Worker 或恢复器提交的状态。

Lease 心跳独立于 FFmpeg 进度：任务等待 CPU/GPU 资源槽时仍会续租。Worker 在启动 FFmpeg 前会立即按同一 Lease 条件做一次续租预检，从而阻止 Claim 后恰好收到取消请求的竞态启动。

服务重启时：

- 原有 queued Job 保持 queued，不再判为失败。
- 升级前无 Lease 的 running/claimed 记录返回 queued。
- 过期且仍期望运行的 Lease 返回 queued，之后可被新 Worker Claim。
- 已请求取消的过期 Lease 进入 cancelled。
- 对应 Attempt 的 staging / publishing Artifact 进入 abandoned，不再可读。
- 重建执行载荷时从 Job、兼容任务和 Media 表读取数据；坏载荷会被单独终结，不阻塞后续队列。

领取顺序采用 Priority + FIFO：交互播放为 100，失败重试为 70，后台批量任务为 20；相同优先级按创建时间和 ID 稳定排序。若后台任务已通过 Active Key 排队，后续播放请求会原子提升数据库 Priority。已被 Worker Claim 或正在运行的任务不会被伪装成仍可重排。

### Attempt

一次 Job 可以包含多个 `transcode_attempts`。硬件失败转软件时创建新的 Attempt，不能覆盖失败证据或修改服务级硬件状态。硬件与软件 Attempt 复用同一 Job Lease，但每次启动都会更新 `current_attempt_id`。

每个 Attempt 拥有唯一工作区：

```text
cache/transcode/workspaces/<job_id>/<attempt_id>/hls/
```

硬件回退、Lease 恢复和服务重启均创建新 Attempt 与新工作区，不复用、不清空其他 Attempt 的文件。Attempt 编号从数据库历史最大值继续递增。

### Resource Governor

独立管理：

- 软件视频转码槽
- 硬件视频转码槽
- Remux / Smart Remux 槽
- On-demand 视频与音频分片槽

交互播放任务在领取前高于失败重试和后台批量任务。运行中抢占仍属于后续阶段，必须基于可续跑点和显式策略实现。

### Executor / Process Runtime

所有 FFmpeg 进程使用 `exec.CommandContext`，通过 `-progress pipe:2 -nostats` 读取机器协议，记录 PID、心跳、退出码、stderr 尾部、取消和超时结果。

Executor 只写当前 Attempt 工作区。On-demand 辅助分片使用独立缓存根，不写入 Runtime HLS 工作区或不可变发布目录。

### Artifact Store

`transcode_artifacts` 是播放器可读产物的唯一状态源，记录 Media、Profile、源指纹、Planner Version、Job、Attempt、工作区、发布路径、Manifest、大小、时长、错误和保留状态。

状态机：

```text
staging -> publishing -> published
    |           |
    +-> failed / cancelled / abandoned

published -> superseded / expired
```

发布协议：

1. 校验 Manifest 与全部已声明分片。
2. 使用 Job ID、Attempt ID、Lease Token、Current Attempt 和 Lease 过期时间原子进入 `publishing`。
3. 将工作区目录原子重命名到不可变版本：

```text
cache/transcode/artifacts/<media_id>/<profile_id>/<artifact_id>/
```

4. 单事务提交 Artifact `published` 与 Job `completed`，同时释放 Active Key 和 Lease。
5. 若数据库提交失败，物理孤儿不具备 `published` 状态，Resolver 不会返回，由保留清理器处理。

播放器请求 URL 不包含 Job、Attempt 或物理路径。服务端 Resolver 每次请求按以下顺序选择：

1. 当前有效 Lease 与 Current Attempt 对应的 staging / publishing Artifact。
2. 相同 Media、Profile、Source Fingerprint、Planner Version 的最新 published Artifact。
3. 观察期内通过正式 Legacy Artifact Adapter 导入的历史目录。

Lease 失效后旧 Attempt 立即失去可读资格，即使旧 FFmpeg 仍在写自己的工作区，也不会污染新 Timeline。

详细协议见 `docs/TRANSCODE_ARTIFACT_STORE.md`。

### Client Playback Adapters

Web、PC、Android、Emby/Infuse 继续消费 Planner 返回的统一 URL 与 fallback，不拼接 Job ID、Attempt ID 或物理目录。

Runtime HLS 路由由 Artifact-aware Handler Adapter 接管：

- Playlist 每次重新解析 Artifact，不跨任务缓存物理目录。
- staging manifest 使用 `no-cache`。
- published 分片使用不可变长缓存。
- Artifact 分片暂不可用时，仍可通过统一 Runtime 的 On-demand 分片能力兜底。

## 当前已落地

- 持久化 `transcode_jobs`、`transcode_attempts`、`transcode_artifacts`。
- Active Key 幂等与终态释放。
- 数据库直接 Priority + FIFO 领取，不再使用进程内 Job 堆作为执行来源。
- 数据库原子 Claim，跨进程只能有一个领取者。
- Worker ID、Lease Token、领取时间、独立心跳与过期时间。
- 启动前 Lease 预检，阻止 Claim 与取消之间的竞态启动。
- Lease 所有权约束运行态、释放、Artifact 发布和终态写入。
- queued、过期 Lease 和升级前无 Lease 记录的重启续队。
- Lease 失效 Artifact abandoned 与 Resolver fencing。
- 坏载荷隔离，不阻塞低优先级有效任务。
- 跨重启 Attempt 编号连续。
- 每 Attempt 独立工作区。
- Artifact 两阶段发布和不可变版本目录。
- Artifact 数据迁移回填、历史目录导入 Adapter 与保留清理。
- 统一 HLS Handler / Resolver，客户端 URL 不变。
- Artifact 状态统计、存储根和 Resolver 性能基准。
- 交互 100、重试 70、后台 20 的持久优先级分类。
- Active Key 复用时的排队任务优先级原子提升。
- 优雅关闭停止新 Claim；超时后由 Lease 所有者安全重新排队并 abandoned 当前 Artifact。
- 排队和运行中任务的持久取消语义。
- Context 驱动的 FFmpeg 生命周期。
- 机器可读进度与 stderr 诊断。
- 硬件失败独立软件 Attempt。
- CPU/GPU/Remux/On-demand 独立资源槽。
- Runtime HLS 渐进 ABR：首次只启动一个基础档位。
- Smart Remux：兼容视频 copy，不兼容音频转 AAC。
- On-demand 分片限流、锁回收和临时文件原子发布。
- 统一质量目录，同时保留 Runtime 与 Persistent 两类明确码率策略。
- 统一 Probe 缓存、真实 FPS GOP 与 HDR 色彩判断。

## 后续正式阶段

### Phase B：运行控制与可续跑抢占

已完成：

- 数据库原子 Claim、Worker ID、Lease、Heartbeat 和过期续队。
- 启动升级兼容与旧 Worker fencing。
- 数据库 Priority + FIFO 调度和跨实例竞争。
- 重启后执行载荷重建与 Attempt 连号。
- 后台 Active Key 被播放复用时的优先级提升。
- 优雅关闭与 Lease 安全释放。
- Attempt 独立工作区和 Artifact Store 发布协议。
- Queue、Lease、Artifact 与 Resource Governor 可观测性。

仍需完成：

- 可配置 Lease、轮询周期、队列容量与恢复阈值。
- 基于可续跑点的运行中后台任务抢占，而不是直接杀进程丢弃工作。
- 管理端展示 Worker、Lease、Attempt、Artifact、Workspace 和恢复原因。

### Phase C：统一媒体 Probe 与编码配置

已完成：

- 共享质量目录，删除转码和 ABR 的重复硬编码阶梯。
- 基于真实 FPS 生成 GOP。
- 基于色彩元数据和 side data 识别 HDR。
- Probe 单飞缓存、源指纹失效和扫描后 Warmup。

仍需完成：

- 音轨、字幕和容器策略统一到版本化 Encoding Plan。
- 不同后端的色彩格式、位深和 HDR 输出能力矩阵。
- 远程源 Probe 与 VFS 身份模型。

### Phase D：启动流

- 扫描后为必须完整转码的媒体生成版本化启动产物。
- 启动片段与持续转码使用同一 Timeline 和可验证衔接。
- Lite 与 Full 共用轻量预热编排。
- 产物按源文件指纹自动失效。

### Phase E：预处理迁移

- 删除 PreprocessService 的独立 worker、取消表和进程管理。
- 缩略图、雪碧图、完整 HLS 和独立音轨作为统一工作流步骤。
- 观察期结束后删除 Legacy Artifact Adapter 和旧共享目录读取实现。

### Phase F：性能验收

按 Direct、Remux、Smart Remux、启动流、GPU 冷转码、CPU 冷转码分别采集首帧 P50/P95/P99，并覆盖本地磁盘、NAS、WebDAV 和并发用户场景。

当前 CI 固定运行 Artifact Resolver benchmark；真实部署基线仍需在目标 NAS、Docker Volume 与并发播放器环境采集。

## 禁止事项

- 不得通过修改全局硬件字段实现单任务回退。
- 不得使用无缓冲一次性消息表示取消状态。
- 不得在 HTTP Handler 中绕过 Runtime 直接启动媒体 FFmpeg。
- 不得让未持有有效 Lease 的 Worker 写入 Job、Artifact 运行态或终态。
- 不得让低优先级请求降低已排队 Job 的 Priority。
- 不得重新引入进程内 Job 堆作为排队权威。
- 不得把多个码率常量复制到前端。
- 不得让多个 Attempt 写入同一目录。
- 不得在不可变发布目录直接写半成品。
- 不得根据文件存在性绕过 Artifact 状态源。
- 不得把 Legacy Adapter 变成长期双轨写入。
- 不得以“能编译”替代首帧、取消、恢复、发布和资源竞争验收。
