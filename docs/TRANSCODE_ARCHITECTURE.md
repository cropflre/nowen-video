# Nowen Video 转码与全端秒播架构

本文档定义 Web、PC、Android 与 Emby/Infuse 共用的长期媒体执行架构。目标不是以临时前端隐藏、旁路 FFmpeg 或重复队列解决单点问题，而是建立可持续演进的播放决策、任务编排、资源治理、进程执行和产物管理边界。

## 产品目标

对服务端能够解析且未损坏、未加密的媒体：

1. 客户端不需要理解容器与编码差异。
2. 优先使用零转码路径，无法原生播放时自动降级。
3. Web、PC、Android 与兼容客户端使用同一份服务端播放规划。
4. 本地与 NAS 媒体以 P95 首帧时间作为主要性能指标。
5. 所有媒体进程均可取消、重试、诊断、限流和恢复。

不能承诺 DRM、损坏文件、上游失效或网络不可用时固定秒开；这些情况必须返回明确诊断，而不是无限等待。

## 播放决策顺序

1. `direct`：容器、视频和音频均可由客户端稳定解码。
2. `remux`：音视频编码兼容，只转换为 fragmented MP4 容器。
3. `smart_remux`：视频 bit-for-bit copy，仅将不兼容音频转换为 AAC-LC。
4. `startup_stream`：使用预生成的兼容启动片段，后台衔接持续转码。
5. `transcode`：完整视频与音频兼容转码。
6. `preprocessed`：复用已经发布的完整长期产物。

当前已落地 direct、remux、smart_remux、runtime HLS，以及持久化 Job Claim / Lease 执行基础。`startup_stream` 与长期预处理迁移属于后续阶段，不应通过伪造缓存或短路播放状态代替。

## 领域边界

```text
Probe
  -> Playback Planner
  -> Job Orchestrator
  -> Resource Governor
  -> Process Runtime
  -> Artifact Publisher
  -> Web / PC / Android / Emby adapters
```

### Probe

统一缓存 ffprobe 结果，包括容器、所有流、真实帧率、位深、色彩信息、音轨、字幕、文件指纹和远程输入能力。

### Playback Planner

只做纯决策，不启动 FFmpeg。输入媒体探测结果、客户端能力、系统策略和已有产物，输出版本化播放计划与 fallback。

### Job Orchestrator

`transcode_jobs` 是长期任务状态源，负责幂等 Active Key、优先级、取消意图、领取、租约、心跳、重试与重启恢复。旧 `transcode_tasks` 在迁移期间仅作为现有管理 API 的兼容投影。

领取使用带状态条件的数据库原子 `UPDATE`。每次成功领取生成独立 `lease_token`，后续进入运行态、续租和提交终态都必须匹配该令牌。旧 Worker 丢失或过期 Lease 后，即使进程延迟返回成功，也不能覆盖恢复器已经提交的最终状态。

Lease 心跳独立于 FFmpeg 进度：任务等待 CPU/GPU 资源槽时仍会续租；数据库拒绝续租时立即取消该 Worker 的 Context。服务启动时会立即回收上一个进程遗留的 queued 或升级前无 Lease 记录，仍在有效期内的 Lease 则等待超时后由周期回收器原子终结。

### Attempt

一次 Job 可以包含多个 `transcode_attempts`。硬件失败转软件时创建新的 Attempt，不能覆盖失败证据或修改服务级硬件状态。硬件与软件 Attempt 复用同一 Job Lease，但每次启动都会更新 `current_attempt_id`。

### Resource Governor

独立管理：

- 软件视频转码槽
- 硬件视频转码槽
- Remux / Smart Remux 槽
- On-demand 视频与音频分片槽

后续优先级编排必须保证交互播放高于后台预热和完整预处理。

### Process Runtime

所有 FFmpeg 进程使用 `exec.CommandContext`，通过 `-progress pipe:2 -nostats` 读取机器协议，记录 PID、心跳、退出码、stderr 尾部、取消和超时结果。

### Artifact Store

`transcode_artifacts` 记录产物种类、Profile、Attempt、临时路径、发布路径、大小、校验、时长和过期策略。写入临时路径后必须原子发布，播放器不得读取半成品。

## 当前已落地

- 持久化 `transcode_jobs`、`transcode_attempts`、`transcode_artifacts`。
- Active Key 幂等与终态释放。
- 数据库原子 Claim，跨 Worker 只能有一个领取者。
- Worker ID、Lease Token、领取时间、独立心跳与过期时间。
- Lease 所有权约束运行态和终态写入，阻止旧 Worker 覆盖恢复结果。
- 服务重启恢复 queued、过期 Lease 和升级前无 Lease 记录。
- 排队和运行中任务的持久取消语义。
- Context 驱动的 FFmpeg 生命周期。
- 机器可读进度与 stderr 诊断。
- 硬件失败独立软件 Attempt，并在同一 Lease 下推进 Attempt。
- CPU/GPU/Remux/On-demand 独立资源槽。
- Runtime HLS 渐进 ABR：首次只启动一个基础档位。
- Smart Remux：兼容视频 copy，不兼容音频转 AAC。
- On-demand 分片限流、锁回收和临时文件原子发布。
- Web、Android、Emby/Infuse 共用 Smart Remux 路径。

## 后续正式阶段

### Phase B：持久优先级编排与运行控制

已完成：

- 数据库原子 Claim。
- Worker ID、Lease、Heartbeat 和过期回收。
- 启动升级兼容与旧 Worker fencing。

仍需完成：

- 交互任务抢占后台任务。
- 数据库优先级队列替换进程内 FIFO channel。
- 服务优雅关闭：停止领取、延长或释放 Lease、等待进程退出。
- 可配置 Lease 与恢复阈值及管理端诊断。

### Phase C：统一媒体 Probe 与编码配置

- 删除重复质量阶梯。
- 基于真实 FPS 生成 GOP。
- 基于色彩元数据识别 HDR。
- 统一缩放、音轨、字幕和容器策略。

### Phase D：启动流

- 扫描后为必须完整转码的媒体生成版本化启动产物。
- 启动片段与持续转码使用同一 Timeline 和可验证衔接。
- Lite 与 Full 共用轻量预热编排。
- 产物按源文件指纹自动失效。

### Phase E：预处理迁移

- 删除 PreprocessService 的独立 worker、取消表和进程管理。
- 缩略图、雪碧图、完整 HLS 和独立音轨作为统一工作流步骤。

### Phase F：性能验收

按 Direct、Remux、Smart Remux、启动流、GPU 冷转码、CPU 冷转码分别采集首帧 P50/P95/P99，并覆盖本地磁盘、NAS、WebDAV 和并发用户场景。

## 禁止事项

- 不得通过修改全局硬件字段实现单任务回退。
- 不得使用无缓冲一次性消息表示取消状态。
- 不得在 HTTP Handler 中绕过 Runtime 直接启动媒体 FFmpeg。
- 不得让未持有有效 Lease 的 Worker 写入 Job 运行态或终态。
- 不得把多个码率常量复制到前端。
- 不得在正式目录直接写半成品。
- 不得以“能编译”替代首帧、取消、恢复和资源竞争验收。
