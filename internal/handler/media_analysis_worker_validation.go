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

// ValidateMediaAnalysisWorkerComplete 同时校验 V1 highlight body 与 V2 generic result envelope。
// 已注册 job 在进入业务层前先做图片魔数校验；服务层继续负责 fingerprint、条数、单图/总大小等业务约束。
func ValidateMediaAnalysisWorkerComplete(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMediaAnalysisWorkerRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "客户端媒体计算结果超过允许大小"})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "无法读取客户端媒体计算结果"})
		return
	}

	var payload service.MediaComputeTaskComplete
	if err := json.Unmarshal(body, &payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "客户端媒体计算结果格式无效"})
		return
	}

	highlights := payload.Highlights
	jobType := strings.TrimSpace(payload.JobType)
	if len(payload.Result) > 0 && (jobType == "" || jobType == service.MediaComputeJobHighlightV1) {
		var result struct {
			Highlights []service.MediaAnalysisWorkerHighlight `json:"highlights"`
		}
		if err := json.Unmarshal(payload.Result, &result); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "精彩片段计算 result 格式无效"})
			return
		}
		highlights = result.Highlights
	}
	if jobType == "" || jobType == service.MediaComputeJobHighlightV1 {
		if err := validateMediaAnalysisHighlightThumbnails(highlights); err != nil {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
	}
	if jobType == service.MediaComputeJobPreviewThumbnailV1 {
		var result service.MediaComputePreviewThumbnailResult
		if len(payload.Result) == 0 || json.Unmarshal(payload.Result, &result) != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "预览图计算 result 格式无效"})
			return
		}
		for _, frame := range result.Frames {
			data, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(frame.DataBase64))
			if decodeErr != nil || !validMediaAnalysisThumbnail(frame.Mime, data) {
				c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": "客户端预览帧格式与内容不匹配"})
				return
			}
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Next()
}

func validateMediaAnalysisHighlightThumbnails(highlights []service.MediaAnalysisWorkerHighlight) error {
	for _, item := range highlights {
		encoded := strings.TrimSpace(item.ThumbnailBase64)
		if encoded == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || !validMediaAnalysisThumbnail(item.ThumbnailMime, data) {
			return errors.New("客户端精彩片段缩略图格式与内容不匹配")
		}
	}
	return nil
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
