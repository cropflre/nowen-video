# Server Lite 第三阶段

第三阶段在不增加数据库迁移、不替换现有执行队列的前提下，完成两个核心收敛点：统一任务中心与播放规划器。

## 设计原则

1. 现有扫描、刮削、转码服务继续作为任务状态源。
2. 聚合层只负责标准化展示，不拥有任务生命周期。
3. 播放规划接口只做决策，不在查询阶段启动 FFmpeg。
4. 新接口均为增量契约，现有 Web、桌面端与 Android API 保持可用。
5. Lite Web 使用新规划；Full 与旧服务自动使用原有逻辑。

## 新增接口

- `GET /api/admin/tasks`
- `GET /api/stream/:id/plan`

详细字段、参数与人工验收项目参见：

- `docs/SERVER_LITE.md`
- `docs/SERVER_LITE_PHASE3_CHECKLIST.md`
