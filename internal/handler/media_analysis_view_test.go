package handler

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestHighlightViewDoesNotTriggerServerPreviewForClientResult(t *testing.T) {
	highlight := model.VideoHighlight{
		ID:        "highlight-client",
		MediaID:   "media-1",
		Source:    "client_android",
		Thumbnail: "/tmp/client.webp",
	}

	view := highlightView("media-1", highlight)
	if view.ThumbnailURL == "" {
		t.Fatal("客户端结果应继续暴露静态缩略图")
	}
	if view.PreviewURL != "" {
		t.Fatalf("客户端未上传动态预览时不应触发服务端 FFmpeg，得到 %q", view.PreviewURL)
	}
}

func TestHighlightViewKeepsLazyPreviewForServerResult(t *testing.T) {
	highlight := model.VideoHighlight{
		ID:        "highlight-server",
		MediaID:   "media-1",
		Source:    "ffmpeg",
		Thumbnail: "/tmp/server.webp",
	}

	view := highlightView("media-1", highlight)
	if view.PreviewURL == "" {
		t.Fatal("服务端 Sparse V2 结果应继续支持首次悬停懒生成动态预览")
	}
}

func TestHighlightViewExposesAlreadyStoredClientPreview(t *testing.T) {
	highlight := model.VideoHighlight{
		ID:          "highlight-client-preview",
		MediaID:     "media-1",
		Source:      "client_desktop",
		Thumbnail:   "/tmp/client.webp",
		PreviewPath: "/tmp/client-preview.webp",
	}

	view := highlightView("media-1", highlight)
	if view.PreviewURL == "" {
		t.Fatal("客户端已经上传并持久化预览时应暴露 preview_url")
	}
}
