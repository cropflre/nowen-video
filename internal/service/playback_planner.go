package service

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	PlaybackMethodDirect    = "direct"
	PlaybackMethodRemux     = "remux"
	PlaybackMethodTranscode = "transcode"
)

// PlaybackClientCapabilities describes the playback features that materially
// affect server-side planning. Unknown clients should use the conservative
// defaults returned by DefaultPlaybackClientCapabilities.
type PlaybackClientCapabilities struct {
	UserAgent          string `json:"user_agent,omitempty"`
	SupportsDirectPlay bool   `json:"supports_direct_play"`
	SupportsRemux      bool   `json:"supports_remux"`
	SupportsHEVC       bool   `json:"supports_hevc"`
	ForceTranscode     bool   `json:"force_transcode"`
	MaxBitrate         int    `json:"max_bitrate,omitempty"`
}

// PlaybackPlan is an additive contract. Existing media info fields and routes
// remain unchanged so Web, desktop and Android clients can migrate gradually.
type PlaybackPlan struct {
	MediaID           string                     `json:"media_id"`
	Method            string                     `json:"method"`
	URL               string                     `json:"url"`
	ReasonCode        string                     `json:"reason_code"`
	Reason            string                     `json:"reason"`
	RequiresTranscode bool                       `json:"requires_transcode"`
	FallbackMethod    string                     `json:"fallback_method,omitempty"`
	FallbackURL       string                     `json:"fallback_url,omitempty"`
	Capabilities      PlaybackClientCapabilities `json:"client_capabilities"`
}

func (s *StreamService) DefaultPlaybackClientCapabilities(userAgent string) PlaybackClientCapabilities {
	return PlaybackClientCapabilities{
		UserAgent:          userAgent,
		SupportsDirectPlay: true,
		SupportsRemux:      true,
		SupportsHEVC:       s.ClientSupportsHEVC(userAgent),
	}
}

// PlanPlayback applies the Lite priority order: direct play, zero-copy remux,
// then HLS transcoding. The method is read-only and does not start FFmpeg.
func (s *StreamService) PlanPlayback(mediaID string, caps PlaybackClientCapabilities) (*PlaybackPlan, error) {
	info, err := s.GetMediaPlayInfo(mediaID)
	if err != nil {
		return nil, err
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	directURL := fmt.Sprintf("/api/stream/%s/direct", mediaID)
	remuxURL := fmt.Sprintf("/api/stream/%s/remux", mediaID)
	hlsURL := info.HlsURL
	if hlsURL == "" {
		hlsURL = fmt.Sprintf("/api/stream/%s/master.m3u8", mediaID)
	}
	if caps.MaxBitrate > 0 && !info.IsPreprocessed {
		hlsURL = appendQuery(hlsURL, "maxBitrate", strconv.Itoa(caps.MaxBitrate))
	}

	plan := &PlaybackPlan{
		MediaID:      mediaID,
		Capabilities: caps,
	}

	// STRM already represents a remote playback source. The direct route is a
	// controlled proxy and therefore remains the safest first choice.
	if info.IsSTRM {
		plan.Method = PlaybackMethodDirect
		plan.URL = directURL
		plan.ReasonCode = "strm_proxy"
		plan.Reason = "远程 STRM 通过服务端代理直接播放"
		plan.RequiresTranscode = false
		return plan, nil
	}

	if caps.ForceTranscode {
		return chooseTranscode(plan, hlsURL, "client_forced_transcode", "客户端要求使用兼容转码"), nil
	}
	if !info.PreferDirectPlay {
		return chooseTranscode(plan, hlsURL, "system_prefers_transcode", "系统设置要求优先使用兼容转码"), nil
	}

	hevcSource := isHEVCCodec(media.VideoCodec)
	directAllowed := info.CanDirectPlay && caps.SupportsDirectPlay && (!hevcSource || caps.SupportsHEVC)
	if directAllowed {
		plan.Method = PlaybackMethodDirect
		plan.URL = directURL
		plan.ReasonCode = "native_direct_play"
		plan.Reason = "容器与音视频编码均受客户端原生支持"
		plan.RequiresTranscode = false
		plan.FallbackMethod = PlaybackMethodTranscode
		plan.FallbackURL = hlsURL
		return plan, nil
	}

	remuxAllowed := info.CanRemux && caps.SupportsRemux && (!hevcSource || caps.SupportsHEVC)
	if remuxAllowed {
		plan.Method = PlaybackMethodRemux
		plan.URL = remuxURL
		plan.ReasonCode = "container_remux"
		plan.Reason = "编码兼容，仅需转换容器，无需重新编码"
		plan.RequiresTranscode = false
		plan.FallbackMethod = PlaybackMethodTranscode
		plan.FallbackURL = hlsURL
		return plan, nil
	}

	reasonCode := "codec_or_container_unsupported"
	reason := "容器或音视频编码不受客户端稳定支持"
	if hevcSource && !caps.SupportsHEVC {
		reasonCode = "client_hevc_unsupported"
		reason = "客户端未声明 HEVC 解码能力"
	} else if info.CanDirectPlay && !caps.SupportsDirectPlay {
		reasonCode = "client_direct_play_disabled"
		reason = "客户端关闭了原始文件直放能力"
	} else if info.CanRemux && !caps.SupportsRemux {
		reasonCode = "client_remux_disabled"
		reason = "客户端不支持 fragmented MP4 Remux"
	}
	return chooseTranscode(plan, hlsURL, reasonCode, reason), nil
}

func chooseTranscode(plan *PlaybackPlan, hlsURL, reasonCode, reason string) *PlaybackPlan {
	plan.Method = PlaybackMethodTranscode
	plan.URL = hlsURL
	plan.ReasonCode = reasonCode
	plan.Reason = reason
	plan.RequiresTranscode = true
	return plan
}

func isHEVCCodec(codec string) bool {
	normalized := strings.ToLower(strings.TrimSpace(codec))
	return normalized == "h265" || normalized == "hevc" || strings.Contains(normalized, "h.265") || strings.Contains(normalized, "hevc")
}

func appendQuery(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		separator := "?"
		if strings.Contains(rawURL, "?") {
			separator = "&"
		}
		return rawURL + separator + url.QueryEscape(key) + "=" + url.QueryEscape(value)
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
