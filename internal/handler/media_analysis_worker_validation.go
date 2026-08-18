package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

const maxMediaAnalysisWorkerRequestBytes int64 = 5 * 1024 * 1024

// ValidateMediaAnalysisWorkerComplete 在业务层持久化之前校验客户端上传负载。
// 服务层仍会继续执行条数、时间范围、单图大小、总大小和 fingerprint 校验；
// 这里专门阻断过大的 JSON 与“声明为图片但实际不是图片”的二进制内容。
func ValidateMediaAnalysisWorkerComplete(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMediaAnalysisWorkerRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "客户端精彩片段结果超过允许大小"})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "无法读取客户端精彩片段结果"})
		return
	}

	var payload service.MediaAnalysisWorkerComplete
	if err := json.Unmarshal(body, &payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "客户端精彩片段结果格式无效"})
		return
	}
	for _, item := range payload.Highlights {
		encoded := strings.TrimSpace(item.ThumbnailBase64)
		if encoded == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || !validMediaAnalysisThumbnail(item.ThumbnailMime, data) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "客户端精彩片段缩略图格式与内容不匹配"})
			return
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Next()
}

func validMediaAnalysisThumbnail(mime string, data []byte) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
	case "image/png":
		return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	case "image/webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	default:
		return false
	}
}
