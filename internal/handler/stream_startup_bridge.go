package handler

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

func (h *ArtifactStreamHandler) StartupBridgePlaylist(c *fiber.Ctx) error {
	if h == nil || h.StreamHandler == nil || h.streamService == nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "stream service unavailable"})
	}
	playlist, err := h.streamService.GetStartupBridgePlaylist(c.Params("id"), c.Params("quality"))
	if err != nil {
		return startupBridgeError(c, err)
	}
	c.Set(fiber.HeaderContentType, "application/vnd.apple.mpegurl")
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendString(playlist)
}

func (h *ArtifactStreamHandler) StartupBridgeSegment(c *fiber.Ctx) error {
	if h == nil || h.StreamHandler == nil || h.streamService == nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "stream service unavailable"})
	}
	file, err := h.streamService.ResolveStartupBridgeSegment(
		c.Params("id"),
		c.Params("quality"),
		c.Params("segment"),
	)
	if err != nil {
		return startupBridgeError(c, err)
	}
	return sendStartupBridgeFile(c, file)
}

func (h *ArtifactStreamHandler) StartupContinuationSegment(c *fiber.Ctx) error {
	if h == nil || h.StreamHandler == nil || h.streamService == nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"error": "stream service unavailable"})
	}
	file, err := h.streamService.ResolveStartupContinuationSegment(
		c.Params("id"),
		c.Params("quality"),
		c.Params("segment"),
	)
	if err != nil {
		return startupBridgeError(c, err)
	}
	return sendStartupBridgeFile(c, file)
}

func sendStartupBridgeFile(c *fiber.Ctx, file *service.StartupBridgeFile) error {
	if file == nil || file.Path == "" {
		return c.SendStatus(http.StatusNotFound)
	}
	c.Set(fiber.HeaderContentType, "video/mp2t")
	c.Set(fiber.HeaderAcceptRanges, "bytes")
	if file.Immutable {
		c.Set(fiber.HeaderCacheControl, "private, max-age=31536000, immutable")
	} else {
		c.Set(fiber.HeaderCacheControl, "no-store")
	}
	return c.SendFile(file.Path)
}

func startupBridgeError(c *fiber.Ctx, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, service.ErrMediaNotFound) {
		return c.SendStatus(http.StatusNotFound)
	}
	return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}
