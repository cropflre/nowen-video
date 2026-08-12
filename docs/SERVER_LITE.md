# Server Lite 历史兼容说明

> 本文件仅为旧链接和历史开发记录保留。
>
> **Server Lite 已不再是 Nowen Video 的独立产品版本。** 原 Lite 架构已经正式扶正，成为 Nowen Video 默认且唯一的正式服务端。

当前权威服务端说明请阅读：

- [`docs/SERVER.md`](./SERVER.md)

## 为什么仍保留这个文件名

仓库中仍有历史 PR、测试、迁移文档和外部链接引用 `docs/SERVER_LITE.md`。直接删除会让这些链接失效，因此暂时保留一个兼容入口。

同理，以下内部名称目前也可能继续存在：

- `cmd/server-lite`
- `internal/service/lite.go`
- `internal/handler/lite.go`
- `AutoMigrateLite`
- 能力协议中的历史 `profile=lite`

它们现在只承担**升级、回滚和旧客户端兼容**职责，不再代表一个叫 Lite 的发行版本。

请勿在新的 UI、README、Docker 标签、Release、部署说明或用户文案中重新使用 `Lite` 作为产品版本名称。
