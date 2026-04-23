package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// CORS 跨域中间件
// allowedOrigins 为允许的源列表，为空则仅允许同源请求（不设置CORS头）
func CORS(allowedOrigins ...string) gin.HandlerFunc {
	// 构建允许的 Origin 集合
	originSet := make(map[string]bool, len(allowedOrigins))
	allowAll := false
	for _, o := range allowedOrigins {
		o = strings.TrimRight(o, "/")
		if o == "*" {
			allowAll = true
		}
		originSet[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// 判断是否允许该 Origin
		if origin != "" && (allowAll || originSet[origin]) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
			c.Writer.Header().Set("Vary", "Origin")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Claims JWT令牌声明
type Claims struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion int    `json:"tv"` // 令牌版本号，用于服务端吊销旧 Token（改密/封禁/降级时自增）
	jwt.RegisteredClaims
}

// TokenVersionProvider 令牌版本号提供者（解耦，避免中间件依赖 repository）
// 返回 (当前版本号, 是否被禁用, 错误)
type TokenVersionProvider func(userID string) (version int, disabled bool, err error)

// JWTAuth JWT认证中间件（基础版：只验签、不检查版本号）
func JWTAuth(secret string) gin.HandlerFunc {
	return JWTAuthWithValidator(secret, nil)
}

// JWTAuthWithValidator JWT认证中间件（带服务端令牌校验）
// 当 validator 非空时，会在令牌签名通过后，进一步校验：
//  1. 用户是否已被禁用（disabled=true 则拒绝）
//  2. 令牌版本号是否已失效（claims.TokenVersion < 数据库中的 token_version 则拒绝）
func JWTAuthWithValidator(secret string, validator TokenVersionProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		// 优先从Header获取
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌格式错误"})
				c.Abort()
				return
			}
		} else {
			// 其次从URL query参数获取（用于视频流等无法设置Header的场景）
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少认证令牌"})
			c.Abort()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "认证令牌无效或已过期"})
			c.Abort()
			return
		}

		// 服务端校验：用户状态 + 令牌版本号
		if validator != nil {
			curVersion, disabled, verr := validator(claims.UserID)
			if verr != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在或已被删除"})
				c.Abort()
				return
			}
			if disabled {
				c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用，请联系管理员"})
				c.Abort()
				return
			}
			if claims.TokenVersion < curVersion {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "登录凭证已失效，请重新登录"})
				c.Abort()
				return
			}
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// AdminOnly 管理员权限中间件
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
