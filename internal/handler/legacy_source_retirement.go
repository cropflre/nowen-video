package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// LegacySourceRetirementHandler exposes explicit administrator review and
// removal-plan protocols. None of the endpoints performs DDL or mutates the
// legacy source table.
type LegacySourceRetirementHandler struct {
	service      *service.LegacySourceRetirementService
	auditService *service.UserService
	logger       *zap.SugaredLogger
}

func NewLegacySourceRetirementHandler(retirementService *service.LegacySourceRetirementService, logger *zap.SugaredLogger) *LegacySourceRetirementHandler {
	return &LegacySourceRetirementHandler{service: retirementService, logger: logger}
}

func (h *LegacySourceRetirementHandler) SetAuditService(userService *service.UserService) {
	h.auditService = userService
}

func (h *LegacySourceRetirementHandler) Report(c *gin.Context) {
	report, err := h.service.Report(c.Param("source"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": report})
}

func (h *LegacySourceRetirementHandler) Review(c *gin.Context) {
	var request service.LegacySourceRetirementReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评审请求格式无效", "code": "legacy_retirement_invalid_request"})
		return
	}
	reviewerID, reviewerName := legacySourceReviewer(c)

	record, err := h.service.Review(c.Param("source"), request, reviewerID, reviewerName)
	if err != nil {
		h.writeError(c, err)
		return
	}
	if h.auditService != nil {
		h.auditService.Audit(
			reviewerID,
			reviewerName,
			"legacy_source_retirement."+record.Decision,
			"legacy_source_retirement",
			record.Source,
			fmt.Sprintf("generation=%d evidence=%s", record.Generation, record.EvidenceHash),
			c.ClientIP(),
		)
	}
	c.JSON(http.StatusCreated, gin.H{"data": record})
}

func (h *LegacySourceRetirementHandler) PrepareRemovalPlan(c *gin.Context) {
	var request service.LegacySourceRemovalPlanRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "删表计划请求格式无效", "code": "legacy_removal_plan_invalid_request"})
		return
	}
	reviewerID, reviewerName := legacySourceReviewer(c)

	plan, err := h.service.PrepareRemovalPlan(c.Param("source"), request, reviewerID, reviewerName)
	if err != nil {
		h.writeError(c, err)
		return
	}
	if h.auditService != nil {
		h.auditService.Audit(
			reviewerID,
			reviewerName,
			"legacy_source_retirement.prepare_removal_plan",
			"legacy_source_removal_plan",
			plan.ID,
			fmt.Sprintf(
				"source=%s generation=%d decision=%s evidence=%s schema=%s",
				plan.Source,
				plan.Generation,
				plan.RetirementDecisionID,
				plan.EvidenceHash,
				plan.SchemaHash,
			),
			c.ClientIP(),
		)
	}
	c.JSON(http.StatusCreated, gin.H{"data": plan})
}

func legacySourceReviewer(c *gin.Context) (string, string) {
	userID, _ := c.Get("user_id")
	reviewerID, _ := userID.(string)
	username, _ := c.Get("username")
	reviewerName, _ := username.(string)
	return reviewerID, reviewerName
}

func (h *LegacySourceRetirementHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrLegacySourceRetirementNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到旧源迁移状态", "code": "legacy_retirement_not_found"})
	case errors.Is(err, service.ErrLegacySourceRetirementEvidenceStale):
		c.JSON(http.StatusConflict, gin.H{"error": "评审证据已变化，请重新获取报告", "code": "legacy_retirement_evidence_stale"})
	case errors.Is(err, service.ErrLegacySourceRemovalPlanBlocked):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "legacy_removal_plan_blocked"})
	case errors.Is(err, service.ErrLegacySourceRetirementBlocked):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "legacy_retirement_blocked"})
	case errors.Is(err, service.ErrLegacySourceRetirementInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "legacy_retirement_invalid"})
	default:
		if h.logger != nil {
			h.logger.Errorf("旧源废弃评审失败: %v", err)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "旧源废弃评审失败", "code": "legacy_retirement_failed"})
	}
}
