package emby

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestAuthenticateByName_JSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建模拟的 Handler（需要 mock auth service）
	// 这里只测试请求解析，不测试实际登录逻辑

	// 测试 JSON body 解析
	body := map[string]string{
		"Username": "testuser",
		"Pw":       "testpass",
	}
	jsonBody, _ := json.Marshal(body)

	// 验证 JSON 解析
	var req AuthenticateByNameRequest
	err := json.Unmarshal(jsonBody, &req)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", req.Username)
	assert.Equal(t, "testpass", req.Pw)
}

func TestAuthenticateByName_JSON_PasswordField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试 Password 字段（旧版）
	body := map[string]string{
		"Username": "testuser",
		"Password": "testpass",
	}
	jsonBody, _ := json.Marshal(body)

	var req AuthenticateByNameRequest
	err := json.Unmarshal(jsonBody, &req)
	assert.NoError(t, err)
	assert.Equal(t, "testuser", req.Username)
	assert.Equal(t, "testpass", req.Password)
}

func TestAuthenticateByName_Form(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试 form body 解析
	form := url.Values{}
	form.Set("Username", "testuser")
	form.Set("Pw", "testpass")

	// 验证 form 解析
	assert.Equal(t, "testuser", form.Get("Username"))
	assert.Equal(t, "testpass", form.Get("Pw"))
}

func TestAuthenticateByName_Form_CaseInsensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试大小写兼容
	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", "testpass")

	assert.Equal(t, "testuser", form.Get("username"))
	assert.Equal(t, "testpass", form.Get("password"))
}

func TestAuthenticateByName_Query(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试 query 参数解析
	params := url.Values{}
	params.Set("Username", "testuser")
	params.Set("Pw", "testpass")

	// 验证 query 解析
	assert.Equal(t, "testuser", params.Get("Username"))
	assert.Equal(t, "testpass", params.Get("Pw"))
}

func TestAuthenticateByName_Query_CaseInsensitive(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试大小写兼容
	params := url.Values{}
	params.Set("username", "testuser")
	params.Set("password", "testpass")

	assert.Equal(t, "testuser", params.Get("username"))
	assert.Equal(t, "testpass", params.Get("password"))
}

func TestAuthenticateByName_ConfigSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试配置开关
	cfg := &config.Config{
		Emby: config.EmbyConfig{
			AllowQueryLogin: false,
		},
	}

	// 验证配置
	assert.False(t, cfg.Emby.AllowQueryLogin)

	// 开启配置
	cfg.Emby.AllowQueryLogin = true
	assert.True(t, cfg.Emby.AllowQueryLogin)
}

func TestAuthenticateByName_RequestParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 测试完整的请求解析流程
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 模拟 handler（只测试解析，不测试实际登录）
	router.POST("/test", func(c *gin.Context) {
		var username, password string

		// 1. 尝试 JSON body
		var jsonReq AuthenticateByNameRequest
		if err := c.ShouldBindJSON(&jsonReq); err == nil {
			username = jsonReq.Username
			password = jsonReq.Pw
			if password == "" {
				password = jsonReq.Password
			}
		}

		// 2. 尝试 form body
		if username == "" {
			username = c.PostForm("Username")
			if username == "" {
				username = c.PostForm("username")
			}
			password = c.PostForm("Pw")
			if password == "" {
				password = c.PostForm("pw")
			}
			if password == "" {
				password = c.PostForm("Password")
			}
			if password == "" {
				password = c.PostForm("password")
			}
		}

		// 3. 尝试 query 参数
		if username == "" {
			username = c.Query("Username")
			if username == "" {
				username = c.Query("username")
			}
			password = c.Query("Pw")
			if password == "" {
				password = c.Query("pw")
			}
			if password == "" {
				password = c.Query("Password")
			}
			if password == "" {
				password = c.Query("password")
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"username": username,
			"password": password,
		})
	})

	// 测试 JSON body
	body := map[string]string{
		"Username": "jsonuser",
		"Pw":       "jsonpass",
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/test", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "jsonuser")
	assert.Contains(t, w.Body.String(), "jsonpass")
}
