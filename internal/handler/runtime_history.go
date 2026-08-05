package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RuntimeHistoryHandler struct {
	service *service.RuntimeHistoryService
	logger  *zap.SugaredLogger
}

func NewRuntimeHistoryHandler(history *service.RuntimeHistoryService, logger *zap.SugaredLogger) *RuntimeHistoryHandler {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &RuntimeHistoryHandler{service: history, logger: logger}
}

func (h *RuntimeHistoryHandler) List(c *gin.Context) {
	query, err := runtimeHistoryQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.List(query)
	if err != nil {
		h.logger.Errorf("读取 Runtime 历史失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行历史失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *RuntimeHistoryHandler) Summary(c *gin.Context) {
	result, err := h.service.Summary()
	if err != nil {
		h.logger.Errorf("汇总 Runtime 历史失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行历史汇总失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *RuntimeHistoryHandler) Detail(c *gin.Context) {
	result, err := h.service.Detail(c.Param("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "运行历史记录不存在"})
			return
		}
		h.logger.Errorf("读取 Runtime 历史详情失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取运行历史详情失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func runtimeHistoryQuery(c *gin.Context) (service.RuntimeHistoryQuery, error) {
	page, err := positiveQueryInt(c.Query("page"), 1)
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	pageSize, err := positiveQueryInt(c.Query("page_size"), 25)
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	if pageSize > 100 {
		pageSize = 100
	}
	from, err := optionalHistoryTime(c.Query("from"))
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	to, err := optionalHistoryTime(c.Query("to"))
	if err != nil {
		return service.RuntimeHistoryQuery{}, err
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 128 {
		return service.RuntimeHistoryQuery{}, errors.New("search 最多 128 个字符")
	}
	return service.RuntimeHistoryQuery{
		Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status")),
		Intent: strings.TrimSpace(c.Query("intent")), MediaID: strings.TrimSpace(c.Query("media_id")),
		Search: search, From: from, To: to,
	}, nil
}

func positiveQueryInt(value string, fallback int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New("分页参数必须是正整数")
	}
	return parsed, nil
}

func optionalHistoryTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return &parsed, nil
		}
	}
	return nil, errors.New("时间参数必须是 RFC3339 或 YYYY-MM-DD")
}
