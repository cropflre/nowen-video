package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

func (h *LegacySourceRetirementHandler) IsolationState(c *gin.Context) {
	state, err := h.service.IsolationState(c.Param("source"))
	if err != nil {
		h.writeIsolationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": state})
}

func (h *LegacySourceRetirementHandler) Isolate(c *gin.Context) {
	var request service.LegacySourceIsolationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "隔离请求格式无效", "code": "legacy_source_isolation_invalid_request"})
		return
	}
	reviewerID, reviewerName := legacySourceReviewer(c)
	record, err := h.service.Isolate(c.Param("source"), request, reviewerID, reviewerName)
	if err != nil {
		h.writeIsolationError(c, err)
		return
	}
	if h.auditService != nil {
		h.auditService.Audit(
			reviewerID,
			reviewerName,
			"legacy_source_retirement.isolate",
			"legacy_source_isolation",
			record.ID,
			fmt.Sprintf(
				"source=%s generation=%d plan=%s evidence=%s schema=%s table=%s",
				record.Source,
				record.Generation,
				record.RemovalPlanID,
				record.EvidenceHash,
				record.SchemaHash,
				record.ArchiveTable,
			),
			c.ClientIP(),
		)
	}
	c.JSON(http.StatusCreated, gin.H{"data": record})
}

func (h *LegacySourceRetirementHandler) RollbackIsolation(c *gin.Context) {
	var request service.LegacySourceIsolationRollbackRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "隔离回滚请求格式无效", "code": "legacy_source_isolation_rollback_invalid_request"})
		return
	}
	reviewerID, reviewerName := legacySourceReviewer(c)
	record, err := h.service.RollbackIsolation(c.Param("source"), request, reviewerID, reviewerName)
	if err != nil {
		h.writeIsolationError(c, err)
		return
	}
	if h.auditService != nil {
		h.auditService.Audit(
			reviewerID,
			reviewerName,
			"legacy_source_retirement.rollback_isolation",
			"legacy_source_isolation_rollback",
			record.ID,
			fmt.Sprintf(
				"source=%s isolation=%s plan=%s schema=%s table=%s",
				record.Source,
				record.IsolationID,
				record.RemovalPlanID,
				record.SchemaHash,
				record.OriginalTable,
			),
			c.ClientIP(),
		)
	}
	c.JSON(http.StatusCreated, gin.H{"data": record})
}

func (h *LegacySourceRetirementHandler) writeIsolationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrLegacySourceIsolationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到旧源隔离作记录或删除计划", "code": "legacy_source_isolation_not_found"})
	case errors.Is(err, service.ErrLegacySourceIsolationBlocked):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "legacy_source_isolation_blocked"})
	case errors.Is(err, service.ErrLegacySourceIsolationConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "code": "legacy_source_isolation_conflict"})
	default:
		h.writeError(c, err)
	}
}
