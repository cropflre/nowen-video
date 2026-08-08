package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

type PlaybackSessionHandler struct {
	sessions   *service.PlaybackSessionService
	permission *service.PermissionService
	mediaRepo  *repository.MediaRepo
	logger     *zap.SugaredLogger
}

func NewPlaybackSessionHandler(
	sessions *service.PlaybackSessionService,
	permission *service.PermissionService,
	mediaRepo *repository.MediaRepo,
	logger *zap.SugaredLogger,
) *PlaybackSessionHandler {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &PlaybackSessionHandler{
		sessions:   sessions,
		permission: permission,
		mediaRepo:  mediaRepo,
		logger:     logger,
	}
}

func (h *PlaybackSessionHandler) Create(c *gin.Context) {
	var request service.PlaybackSessionCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "播放会话参数无效"})
		return
	}
	if err := h.checkMediaAccess(c, request.MediaID); err != nil {
		h.writeError(c, err)
		return
	}
	result, err := h.sessions.Create(c.Request.Context(), c.GetString("user_id"), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	status := http.StatusCreated
	if !result.FirstSegmentReady {
		status = http.StatusAccepted
	}
	c.JSON(status, gin.H{"data": result})
}

func (h *PlaybackSessionHandler) Status(c *gin.Context) {
	result, err := h.sessions.Status(c.GetString("user_id"), c.Param("sessionID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *PlaybackSessionHandler) Heartbeat(c *gin.Context) {
	var request service.PlaybackSessionHeartbeatRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "播放心跳参数无效"})
		return
	}
	result, err := h.sessions.Heartbeat(c.GetString("user_id"), c.Param("sessionID"), request)
	if err != nil {
		h.writeError(c, err)
		return
	}
	if err := h.sessions.ReconcilePlaybackThrottle(c.Param("sessionID")); err != nil {
		h.logger.Debugw("reconcile playback throttle after heartbeat failed",
			"session_id", c.Param("sessionID"),
			"error", err,
		)
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *PlaybackSessionHandler) Restart(c *gin.Context) {
	var request service.PlaybackSessionRestartRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "播放重启参数无效"})
		return
	}
	result, err := h.sessions.Restart(
		c.Request.Context(),
		c.GetString("user_id"),
		c.Param("sessionID"),
		request,
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	status := http.StatusOK
	if !result.FirstSegmentReady {
		status = http.StatusAccepted
	}
	c.JSON(status, gin.H{"data": result})
}

func (h *PlaybackSessionHandler) Close(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	if err := h.sessions.Close(
		ctx,
		c.GetString("user_id"),
		c.Param("sessionID"),
		c.Query("reason"),
	); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *PlaybackSessionHandler) Playlist(c *gin.Context) {
	generationID, err := parseGenerationID(c.Param("generationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	file, err := h.sessions.OpenPlaylist(c.GetString("user_id"), c.Param("sessionID"), generationID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	defer file.Release()
	h.serveSessionFile(c, file)
}

func (h *PlaybackSessionHandler) Segment(c *gin.Context) {
	generationID, err := parseGenerationID(c.Param("generationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	file, err := h.sessions.OpenSegment(
		c.GetString("user_id"),
		c.Param("sessionID"),
		generationID,
		c.Param("file"),
	)
	if err != nil {
		h.writeError(c, err)
		return
	}
	defer file.Release()
	h.serveSessionFile(c, file)
}

func (h *PlaybackSessionHandler) serveSessionFile(c *gin.Context, file *service.PlaybackSessionFile) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", file.ContentType)
	http.ServeFile(c.Writer, c.Request, file.Path)
}

func (h *PlaybackSessionHandler) checkMediaAccess(c *gin.Context, mediaID string) error {
	if mediaID == "" {
		return service.ErrMediaNotFound
	}
	if role, _ := c.Get("role"); role == "admin" {
		return nil
	}
	if h.permission == nil || h.mediaRepo == nil {
		return service.ErrForbidden
	}
	userID := c.GetString("user_id")
	return h.permission.CheckMediaAccess(userID, mediaID, func(id string) (string, error) {
		media, err := h.mediaRepo.FindByID(id)
		if err != nil {
			return "", err
		}
		return media.LibraryID, nil
	})
}

func (h *PlaybackSessionHandler) writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrMediaNotFound),
		errors.Is(err, playbacksession.ErrSessionNotFound),
		errors.Is(err, playbacksession.ErrGenerationNotFound),
		os.IsNotExist(err):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrForbidden),
		errors.Is(err, service.ErrContentRestricted),
		errors.Is(err, service.ErrTimeLimitExceeded):
		status = http.StatusForbidden
	case errors.Is(err, playbacksession.ErrInvalidRequest):
		status = http.StatusBadRequest
	case errors.Is(err, playbacksession.ErrSessionClosing),
		errors.Is(err, playbacksession.ErrGenerationNotReady),
		errors.Is(err, playbacksession.ErrGenerationNotActive):
		status = http.StatusConflict
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	case errors.Is(err, context.Canceled):
		status = 499
	}
	if status >= 500 {
		h.logger.Errorw("playback session request failed", "error", err)
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func parseGenerationID(raw string) (uint64, error) {
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("generation id must be a positive integer")
	}
	return value, nil
}
