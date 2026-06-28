# PR #11 改进方案实施总结

## 已完成的改进

### 1. IDMapper V2 - 碰撞处理 ✅

**文件：** `internal/handler/emby/id_mapper_v2.go`

**改进点：**
- 使用 int31 空间避免 Kodi `setDbId` 大整数崩溃
- 添加碰撞处理：当发生哈希碰撞时，使用 salt 递增生成新 ID
- 支持启动时预热 ID 映射
- 添加统计信息和碰撞检测

**关键代码：**
```go
func (m *IDMapperV2) generateID(uuid string, salt int) int32 {
    hash := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", uuid, salt)))
    return int32(binary.BigEndian.Uint32(hash[:4]) & 0x7FFFFFFF)
}
```

**测试文件：** `internal/handler/emby/id_mapper_v2_test.go`

---

### 2. 日志脱敏中间件 ✅

**文件：** `internal/middleware/log_sanitizer.go`

**改进点：**
- 脱敏 query 参数中的敏感信息
- 脱敏 header 中的敏感信息
- 脱敏 X-Emby-Authorization 中的 Token
- 提供日志消息脱敏函数

**敏感参数列表：**
- api_key
- ApiKey
- X-Emby-Token
- X-MediaBrowser-Token
- Pw
- Password
- Authorization
- X-Emby-ApiKey

**测试文件：** `internal/middleware/log_sanitizer_test.go`

---

### 3. AuthenticateByName 多格式支持 ✅

**文件：** `internal/handler/emby/system.go`

**改进点：**
- 支持 JSON body 登录
- 支持 form body 登录（application/x-www-form-urlencoded）
- 支持 query 参数登录（需要配置开关）
- 字段大小写兼容：Username/username, Pw/pw, Password/password

**配置开关：**
```yaml
emby:
  allow_query_login: false  # 默认关闭，更安全
```

**测试文件：** `internal/handler/emby/auth_test.go`

---

### 4. 配置扩展 ✅

**文件：** `internal/config/config.go`

**新增配置：**
```go
type EmbyConfig struct {
    // ... 现有配置 ...
    
    // 是否允许 query 参数登录（默认关闭，更安全）
    // 某些旧版客户端（如 EmbyCon）可能需要 query 登录
    AllowQueryLogin bool `mapstructure:"allow_query_login"`
}
```

**默认值：**
```go
viper.SetDefault("emby.allow_query_login", false)
```

---

## 待完成的改进

### 1. 启动时预热 ID 映射

**需要修改：** `cmd/server/main.go`

**方案：**
```go
// 在 main.go 启动时调用
func main() {
    // ... 初始化服务 ...

    // 预热 ID 映射
    libraries, _ := libraryRepo.GetAll()
    media, _ := mediaRepo.GetAll()
    series, _ := seriesRepo.GetAll()
    
    embyHandler.IDMapper().WarmupAll(
        extractIDs(libraries),
        extractIDs(media),
        extractIDs(series),
    )

    // ... 启动服务器 ...
}
```

### 2. /Sessions 返回当前会话

**需要修改：** `internal/handler/emby/router.go`

**当前实现：**
```go
// Sessions 列表（最小实现：返回空数组，Infuse 会忽略）
g.GET("/Sessions", func(c *gin.Context) {
    c.JSON(http.StatusOK, []SessionInfo{})
})
```

**改进方案：**
```go
// Sessions 列表（返回当前活跃会话）
g.GET("/Sessions", h.SessionsHandler)
```

**新增 Handler：**
```go
func (h *Handler) SessionsHandler(c *gin.Context) {
    userID := c.GetString("user_id")
    if userID == "" {
        c.JSON(http.StatusOK, []SessionInfo{})
        return
    }

    // 构建当前会话信息
    authHdr := parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
    sess := SessionInfo{
        Id:                 newSessionID(userID, authHdr.DeviceId),
        UserId:             h.idMap.ToEmbyID(userID),
        UserName:           c.GetString("username"),
        Client:             authHdr.Client,
        DeviceName:         authHdr.Device,
        DeviceId:           authHdr.DeviceId,
        ApplicationVersion: authHdr.Version,
        ServerId:           h.serverID,
        IsActive:           true,
        PlayableMediaTypes: []string{"Video", "Audio"},
        SupportedCommands:  []string{},
        LastActivityDate:   formatEmbyTime(nowUTC()),
    }

    c.JSON(http.StatusOK, []SessionInfo{sess})
}
```

### 3. 短期 Stream Token（长期方案）

**需要新增：** `internal/service/stream_token.go`

**方案：**
```go
type StreamToken struct {
    MediaID   string    `json:"media_id"`
    UserID    string    `json:"user_id"`
    ExpiresAt time.Time `json:"expires_at"`
    Token     string    `json:"token"`
}

type StreamTokenService struct {
    tokens   *sync.Map
    secret   string
}

func (s *StreamTokenService) Generate(userID, mediaID string, duration time.Duration) (*StreamToken, error) {
    // 生成短期 token
}

func (s *StreamTokenService) Validate(tokenStr string) (*StreamToken, error) {
    // 验证 token
}
```

---

## 使用指南

### 1. 启用 IDMapper V2

在 `internal/handler/emby/handler.go` 中替换 IDMapper：

```go
// 替换
idMap: NewIDMapper(),

// 为
idMap: NewIDMapperV2(),
```

### 2. 启用日志脱敏中间件

在 `cmd/server/main.go` 中添加中间件：

```go
import "github.com/nowen-video/nowen-video/internal/middleware"

// 在路由注册前添加
r.Use(middleware.LogSanitizer())
```

### 3. 配置 query 登录

在 `config.yaml` 中添加：

```yaml
emby:
  allow_query_login: false  # 默认关闭，如需支持 EmbyCon 可开启
```

---

## 测试覆盖

### 已覆盖的测试

1. **IDMapper V2 测试**
   - 基本映射
   - 空 UUID
   - 数字 UUID
   - 反向解析
   - 碰撞处理
   - 并发访问
   - 批量注册
   - 预热功能
   - 统计信息

2. **日志脱敏测试**
   - Query 参数脱敏
   - Header 脱敏
   - Emby Authorization 脱敏
   - 普通参数不被脱敏
   - 日志消息脱敏

3. **AuthenticateByName 测试**
   - JSON body 解析
   - Password 字段兼容
   - Form body 解析
   - 大小写兼容
   - Query 参数解析
   - 配置开关

### 待补充的测试

1. **集成测试**
   - 完整的登录流程
   - /Sessions 返回当前会话
   - /Items/Prefixes 返回首字母列表
   - NameStartsWith 筛选

2. **端到端测试**
   - Infuse 客户端连接测试
   - EmbyCon 客户端连接测试
   - Kodi Emby 插件连接测试

---

## 合并检查清单

- [x] IDMapper 碰撞处理已实现
- [x] 日志脱敏已添加
- [x] query 登录配置开关已添加
- [x] 核心测试已补充
- [ ] 启动时 ID 预热已实现
- [ ] /Sessions 返回当前会话已实现
- [ ] 公网反代路径测试通过

---

## 下一步行动

1. **短期（1-2 天）**
   - 完成启动时 ID 预热
   - 实现 /Sessions 返回当前会话
   - 补充集成测试

2. **中期（1 周）**
   - 实现短期 Stream Token
   - 补充端到端测试
   - 性能优化

3. **长期（1 个月）**
   - 持久化 ID 映射表
   - 完善文档
   - 社区反馈收集

---

## 总结

本次改进针对 PR #11 的关键问题进行了修复：

1. **ID 稳定性**：通过碰撞处理确保 ID 唯一性
2. **安全性**：通过日志脱敏保护敏感信息
3. **兼容性**：通过多格式登录支持更多客户端
4. **可配置性**：通过配置开关控制敏感功能

这些改进保留了 PR #11 的兼容性修复，同时解决了 ID 稳定性和 token 安全问题，为长期稳定运行奠定了基础。
