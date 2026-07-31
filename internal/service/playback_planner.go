package service

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	PlaybackMethodDirect     = "direct"
	PlaybackMethodRemux      = "remux"
	PlaybackMethodSmartRemux = "smart_remux"
	PlaybackMethodTranscode  = "transcode"
)

type PlaybackClientCapabilities struct {
	UserAgent          string `json:"user_agent,omitempty"`
	SupportsDirectPlay bool   `json:"supports_direct_play"`
	SupportsRemux      bool   `json:"supports_remux"`
	SupportsHEVC       bool   `json:"supports_hevc"`
	ForceTranscode     bool   `json:"force_transcode"`
	MaxBitrate         int    `json:"max_bitrate,omitempty"`
}

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

func (s *StreamService) PlanPlayback(mediaID string, caps PlaybackClientCapabilities) (*PlaybackPlan, error) {
	info, err := s.GetMediaPlayInfo(mediaID)
	if err != nil {
		return nil, err
	}
	return s.PlanPlaybackWithInfo(mediaID, info, caps)
}

func (s *StreamService) PlanPlaybackWithInfo(mediaID string, info *MediaPlayInfo, caps PlaybackClientCapabilities) (*PlaybackPlan, error) {
	if info == nil {
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

	plan := &PlaybackPlan{MediaID: mediaID, Capabilities: caps}
	if info.IsSTRM {
		plan.Method = PlaybackMethodDirect
		plan.URL = directURL
		plan.ReasonCode = "strm_proxy"
		plan.Reason = "远程 STRM 通过服务端代理直接播放"
		return plan, nil
	}
	if caps.ForceTranscode {
		return chooseTranscode(plan, hlsURL, "client_forced_transcode", "客户端要求使用兼容转码"), nil
	}
	if !info.PreferDirectPlay {
		return chooseTranscode(plan, hlsURL, "system_prefers_transcode", "系统设置要求优先使用兼容转码"), nil
	}

	hevcSource := isHEVCCodec(info.VideoCodec)
	directAllowed := info.CanDirectPlay && caps.SupportsDirectPlay && (!hevcSource || caps.SupportsHEVC)
	if directAllowed {
		plan.Method = PlaybackMethodDirect
		plan.URL = directURL
		plan.ReasonCode = "native_direct_play"
		plan.Reason = "容器与音视频编码均受客户端原生支持"
		plan.FallbackMethod = PlaybackMethodTranscode
		plan.FallbackURL = hlsURL
		return plan, nil
	}

	remuxAllowed := info.CanRemux && caps.SupportsRemux && (!hevcSource || caps.SupportsHEVC)
	if remuxAllowed {
		plan.Method = PlaybackMethodRemux
		plan.URL = remuxURL
		plan.ReasonCode = "container_remux"
		plan.Reason = "编码兼容，仅转换容器，音视频均直接复制"
		plan.FallbackMethod = PlaybackMethodTranscode
		plan.FallbackURL = hlsURL
		return plan, nil
	}

	// Smart Remux keeps compatible video bit-for-bit and only converts an
	// incompatible audio track to AAC. This is dramatically cheaper than full
	// video transcoding and covers common H.264+DTS/TrueHD/FLAC libraries.
	if caps.SupportsRemux && canSmartRemuxInfo(info, caps) {
		plan.Method = PlaybackMethodSmartRemux
		plan.URL = remuxURL
		plan.ReasonCode = "audio_transcode_only"
		plan.Reason = "视频编码可直接复制，仅将不兼容音频转换为 AAC"
		plan.RequiresTranscode = true
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
	} else if (info.CanRemux || smartRemuxVideoCodec(info.VideoCodec)) && !caps.SupportsRemux {
		reasonCode = "client_remux_disabled"
		reason = "客户端不支持 fragmented MP4 Remux"
	}
	return chooseTranscode(plan, hlsURL, reasonCode, reason), nil
}

func canSmartRemuxInfo(info *MediaPlayInfo, caps PlaybackClientCapabilities) bool {
	if info == nil || info.IsSTRM || !smartRemuxVideoCodec(info.VideoCodec) {
		return false
	}
	if isHEVCCodec(info.VideoCodec) && !caps.SupportsHEVC {
		return false
	}
	audio := strings.ToLower(strings.TrimSpace(info.AudioCodec))
	return audio != "" && !mp4CopyAudioCodecs[audio]
}

func smartRemuxVideoCodec(codec string) bool {
	normalized := strings.ToLower(strings.TrimSpace(codec))
	return managedRemuxVideoCodecs[normalized]
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
