# PR #11 改进方案

## 概述

PR #11 的方向正确，但需要在合并前修复以下关键问题。

## 保留的功能

### 1. Emby 兼容路由补全
- `/System/WakeOnLanInfo`
- `/Playback/BitrateTest`
- `/web/manifest.json`
- `/Items/Prefixes`
- lowercase route aliases
- HLS / stream HEAD 支持

### 2. AuthenticateByName 多格式兼容
- JSON body
- application/x-www-form-urlencoded
- 字段大小写兼容：Username/username, Pw/pw, Password/password

### 3. /Sessions 返回当前会话
- 返回 active session 而不是空数组

### 4. DTO 稳定 JSON 结构
- UserData 不省略
- ImageTags 不省略
- BackdropImageTags 不省略
- ChildCount
- RecursiveItemCount

### 5. /Items/Prefixes 和 NameStartsWith / NameLessThan

---

## 需要修改的问题

### 问题 1: IDMapper 碰撞风险

**当前问题：**
- int31 空间约 21 亿，存在哈希碰撞风险
- 后注册的会覆盖前一个，导致 ID 不稳定

**解决方案：**
```go
// IDMapper 碰撞处理
type IDMapper struct {
    mu        sync.RWMutex
    forward   map[string]int32   // uuid -> emby_id
    reverse   map[int32]string   // emby_id -> uuid
    salt      int                // 碰撞时递增的 salt
}

func (m *IDMapper) ToEmbyID(uuid string) int32 {
    m.mu.RLock()
    if id, ok := m.forward[uuid]; ok {
        m.mu.RUnlock()
        return id
    }
    m.mu.RUnlock()

    m.mu.Lock()
    defer m.mu.Unlock()

    // 双重检查
    if id, ok := m.forward[uuid]; ok {
        return id
    }

    // 生成 ID 并处理碰撞
    salt := 0
    for {
        id := m.generateID(uuid, salt)
        if existingUUID, exists := m.reverse[id]; !exists {
            // 未碰撞，直接注册
            m.forward[uuid] = id
            m.reverse[id] = uuid
            return id
        } else if existingUUID == uuid {
            // 同一 UUID，直接返回
            return id
        }
        // 碰撞，递增 salt 重试
        salt++
    }
}

func (m *IDMapper) generateID(uuid string, salt int) int32 {
    hash := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", uuid, salt)))
    return int32(binary.BigEndian.Uint32(hash[:4]) & 0x7FFFFFFF)
}
```

**单测覆盖：**
```go
func TestIDMapperCollision(t *testing.T) {
    mapper := NewIDMapper()

    // 测试正常映射
    id1 := mapper.ToEmbyID("uuid-1")
    id2 := mapper.ToEmbyID("uuid-2")
    assert.NotEqual(t, id1, id2)

    // 测试稳定性
    id1Again := mapper.ToEmbyID("uuid-1")
    assert.Equal(t, id1, id1Again)

    // 测试碰撞处理
    // 模拟碰撞场景...
}
```

---

### 问题 2: 旧 63-bit ID 兼容

**当前问题：**
- legacy ID 只有在 ToEmbyID 被调用后才注册
- 服务器重启后，客户端拿旧链接直接请求可能失败

**解决方案：**
```go
// 启动时预热 ID 映射
func (s *EmbyService) WarmupIDMappings() error {
    // 1. 预热所有 library
    libraries, err := s.libraryRepo.GetAll()
    if err != nil {
        return err
    }
    for _, lib := range libraries {
        s.idMapper.ToEmbyID(lib.UUID)
    }

    // 2. 预热所有 media
    media, err := s.mediaRepo.GetAll()
    if err != nil {
        return err
    }
    for _, m := range media {
        s.idMapper.ToEmbyID(m.UUID)
    }

    // 3. 预热所有 series
    series, err := s.seriesRepo.GetAll()
    if err != nil {
        return err
    }
    for _, s := range series {
        s.idMapper.ToEmbyID(s.UUID)
    }

    return nil
}

// 在 main.go 启动时调用
func main() {
    // ... 初始化服务 ...

    // 预热 ID 映射
    if err := embyService.WarmupIDMappings(); err != nil {
        logger.Warn("ID 映射预热失败", zap.Error(err))
    }

    // ... 启动服务器 ...
}
```

**长期方案：持久化映射表**
```sql
CREATE TABLE IF NOT EXISTS emby_id_mappings (
    uuid TEXT PRIMARY KEY,
    emby_id INTEGER NOT NULL,
    kind TEXT NOT NULL,  -- media/series/library/user/genre/studio
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_emby_id ON emby_id_mappings(emby_id);
```

---

### 问题 3: JWT Token 暴露在 URL

**当前问题：**
- 播放 URL 包含 `api_key={JWT}`
- URL 可能进入 access log、播放器日志、反代日志

**短期方案：日志脱敏**
```go
// 日志脱敏中间件
func LogSanitizer() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 脱敏敏感参数
        sensitiveKeys := []string{"api_key", "X-Emby-Token", "Pw", "Password", "Authorization"}

        // 处理 query 参数
        query := c.Request.URL.Query()
        for _, key := range sensitiveKeys {
            if query.Has(key) {
                query.Set(key, "***REDACTED***")
            }
        }

        // 处理 header
        for _, key := range sensitiveKeys {
            if c.Request.Header.Get(key) != "" {
                c.Request.Header.Set(key, "***REDACTED***")
            }
        }

        c.Next()
    }
}
```

**长期方案：短期 Stream Token**
```go
// 生成短期 stream token
type StreamToken struct {
    MediaID   string    `json:"media_id"`
    UserID    string    `json:"user_id"`
    ExpiresAt time.Time `json:"expires_at"`
    Token     string    `json:"token"`
}

func (s *AuthService) GenerateStreamToken(userID, mediaID string, duration time.Duration) (*StreamToken, error) {
    token := &StreamToken{
        MediaID:   mediaID,
        UserID:    userID,
        ExpiresAt: time.Now().Add(duration),
    }

    // 生成签名
    payload := fmt.Sprintf("%s:%s:%d", userID, mediaID, token.ExpiresAt.Unix())
    signature := hmac.New(sha256.New, []byte(s.jwtSecret))
    signature.Write([]byte(payload))
    token.Token = hex.EncodeToString(signature.Sum(nil))

    // 存储到 Redis/内存（用于验证）
    s.streamTokens.Set(token.Token, token, duration)

    return token, nil
}

func (s *AuthService) ValidateStreamToken(tokenStr string) (*StreamToken, error) {
    token, exists := s.streamTokens.Get(tokenStr)
    if !exists {
        return nil, errors.New("token 不存在或已过期")
    }

    streamToken := token.(*StreamToken)
    if time.Now().After(streamToken.ExpiresAt) {
        s.streamTokens.Delete(tokenStr)
        return nil, errors.New("token 已过期")
    }

    return streamToken, nil
}
```

---

### 问题 4: Query 登录密码

**当前问题：**
- AuthenticateByName 支持 query 传用户名密码
- password 出现在 URL 查询参数里，比 body 更容易被日志记录

**解决方案：配置开关**
```yaml
# config.yaml
emby:
  # 是否允许 query 参数登录（默认关闭）
  allow_query_login: false
```

```go
// 登录处理
func (h *EmbyHandler) AuthenticateByName(c *gin.Context) {
    var username, password string

    // 1. 尝试 JSON body
    var jsonReq struct {
        Username string `json:"Username"`
        Pw       string `json:"Pw"`
    }
    if err := c.ShouldBindJSON(&jsonReq); err == nil {
        username = jsonReq.Username
        password = jsonReq.Pw
    }

    // 2. 尝试 form body
    if username == "" {
        username = c.PostForm("Username")
        password = c.PostForm("Pw")
    }

    // 3. 尝试 query（需要配置开关）
    if username == "" && h.config.Emby.AllowQueryLogin {
        username = c.Query("Username")
        password = c.Query("Pw")
    }

    // 4. 大小写兼容
    if username == "" {
        username = c.Query("username")
        password = c.Query("password")
    }

    // ... 验证逻辑 ...
}
```

---

## 需要补充的测试

### 1. AuthenticateByName 测试
```go
func TestAuthenticateByName_JSON(t *testing.T) {
    // 测试 JSON body 登录
}

func TestAuthenticateByName_Form(t *testing.T) {
    // 测试 form body 登录
}

func TestAuthenticateByName_Query(t *testing.T) {
    // 测试 query 登录（需要开启配置）
}

func TestAuthenticateByName_CaseInsensitive(t *testing.T) {
    // 测试大小写兼容
}
```

### 2. /Sessions 测试
```go
func TestSessions_ReturnsActiveSession(t *testing.T) {
    // 登录后调用 /Sessions，应该返回当前会话
}
```

### 3. api_key 鉴权测试
```go
func TestApiKeyAuth_QueryParam(t *testing.T) {
    // 测试通过 query 参数传递 api_key
}

func TestApiKeyAuth_Header(t *testing.T) {
    // 测试通过 header 传递 X-Emby-Token
}
```

### 4. /Items/Prefixes 测试
```go
func TestItemsPrefixes_ReturnsPrefixes(t *testing.T) {
    // 测试返回首字母列表
}

func TestItems_NameStartsWith(t *testing.T) {
    // 测试 NameStartsWith 筛选
}
```

### 5. IDMapper 碰撞测试
```go
func TestIDMapper_CollisionHandling(t *testing.T) {
    // 测试碰撞处理
}

func TestIDMapper_Stability(t *testing.T) {
    // 测试 ID 稳定性
}
```

### 6. 大小写路由测试
```go
func TestLowercaseRoutes(t *testing.T) {
    // 测试 /emby/system/info 和 /emby/System/Info 都能访问
}
```

---

## 实施步骤

### 阶段 1: 核心修复（必须）
1. 修改 IDMapper，添加碰撞处理
2. 添加日志脱敏中间件
3. 添加配置开关控制 query 登录
4. 补充核心测试

### 阶段 2: 增强稳定性（建议）
1. 启动时预热 ID 映射
2. 实现短期 stream token
3. 补充完整测试覆盖

### 阶段 3: 长期优化（可选）
1. 持久化 ID 映射表
2. 完善文档

---

## 合并检查清单

- [ ] IDMapper 碰撞处理已实现
- [ ] 日志脱敏已添加
- [ ] query 登录配置开关已添加
- [ ] 核心测试已补充
- [ ] 启动时 ID 预热已实现
- [ ] 公网反代路径测试通过

---

## 一句话总结

**保留 PR 的兼容性修复，修复 ID 稳定性和 token 安全问题，补充测试后合并。**
