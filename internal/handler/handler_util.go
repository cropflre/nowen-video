package handler

import (
	cryptoRand "crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
)

// parseIntDefault 将字符串解析为 int，失败返回 defaultVal 且不报错
func parseIntDefault(s string, defaultVal int) (int, error) {
	if s == "" {
		return defaultVal, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal, err
	}
	return v, nil
}

// generateSecureToken 生成 byteLen 字节的随机数，返回 2*byteLen 长度的十六进制字符串
func generateSecureToken(byteLen int) (string, error) {
	if byteLen <= 0 {
		byteLen = 8
	}
	b := make([]byte, byteLen)
	if _, err := cryptoRand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机令牌失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}
