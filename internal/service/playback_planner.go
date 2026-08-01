package service

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
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

type PlaybackSourceTechnical struct {
	ProbeVersion string   `json:"probe_version"`
	VideoCodec   string   `json:"video_codec"`
	AudioCodecs  []string `json:"audio_codecs,omitempty"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	FrameRate    float64  `json:"frame_rate,omitempty"`
	PixelFormat  string   `json:"pixel_format,omitempty"`
	BitDepth     int      `json:"bit_depth,omitempty"`
	HDR          bool     `json:"hdr"`
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
	SourceTechnical   *PlaybackSourceTechnical   `json:"source_technical,omitempty"`
	StartupBridge     *StartupBridgeInfo         `json:"startup_bridge,omitempty"`
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

	// Playback planning is latency sensitive. Only a fresh cached Probe is read
	// here; this path never starts FFprobe. Runtime HLS performs cold probing in
	// its claimed Worker, while scan warm-up populates this cache ahead of
	// playback.
	effectiveInfo := *info
	var sourceTechnical *PlaybackSourceTechnical
	if s != nil && s.mediaRepo != nil && s.transcoder != nil {
		if media, err := s.mediaRepo.FindByID(mediaID); err == nil {
			if probe := s.transcoder.GetCachedMediaProbe(media); probe != nil {
				sourceTechnical = applyProbeToPlaybackInfo(mediaID, &effectiveInfo, probe)
			}
		}
	}
	info = &effectiveInfo

	directURL := fmt.Sprintf("/api/stream/%s/direct", mediaID)
	remuxURL := fmt.Sprintf("/api/stream/%s/remux", mediaID)
	hlsURL := info.HlsURL
	if hlsURL == "" {
		hlsURL = fmt.Sprintf("/api/stream/%s/master.m3u8", mediaID)
	}
	if caps.MaxBitrate > 0 && !info.IsPreprocessed {
		hlsURL = appendQuery(hlsURL, "maxBitrate", strconv.Itoa(caps.MaxBitrate))
	}

	var startupBridge *StartupBridgeInfo
	compatibleHLSURL := hlsURL
	if bridge, bridgeErr := s.GetStartupBridgeInfo(mediaID); bridgeErr == nil && bridge != nil && bridge.Available {
		startupBridge = bridge
		compatibleHLSURL = bridge.PlaylistURL
	} else if bridgeErr != nil && s.logger != nil {
		s.logger.Warnf("读取 Startup Bridge 失败 media=%s: %v", mediaID, bridgeErr)
	}

	plan := &PlaybackPlan{
		MediaID:         mediaID,
		Capabilities:    caps,
		SourceTechnical: sourceTechnical,
		StartupBridge:   startupBridge,
	}
	if info.IsSTRM {
		plan.Method = PlaybackMethodDirect
		plan.URL = directURL
		plan.ReasonCode = "strm_proxy"
		plan.Reason = "远程 STRM 通过服务端代理直接播放"
		return plan, nil
	}
	if caps.ForceTranscode {
		return chooseTranscode(plan, compatibleHLSURL, "client_forced_transcode", "客户端要求使用兼容转码"), nil
	}
	if !info.PreferDirectPlay {
		return chooseTranscode(plan, compatibleHLSURL, "system_prefers_transcode", "系统设置要求优先使用兼容转码"), nil
	}

	hevcSource := isHEVCCodec(info.VideoCodec)
	directAllowed := info.CanDirectPlay && caps.SupportsDirectPlay && (!hevcSource || caps.SupportsHEVC)
	if directAllowed {
		plan.Method = PlaybackMethodDirect
		plan.URL = directURL
		plan.ReasonCode = "native_direct_play"
		plan.Reason = "容器与音视频编码均受客户端原生支持"
		plan.FallbackMethod = PlaybackMethodTranscode
		plan.FallbackURL = compatibleHLSURL
		return plan, nil
	}

	remuxAllowed := info.CanRemux && caps.SupportsRemux && (!hevcSource || caps.SupportsHEVC)
	if remuxAllowed {
		plan.Method = PlaybackMethodRemux
		plan.URL = remuxURL
		plan.ReasonCode = "container_remux"
		plan.Reason = "编码兼容，仅转换容器，音视频均直接复制"
		plan.FallbackMethod = PlaybackMethodTranscode
		plan.FallbackURL = compatibleHLSURL
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
		plan.FallbackURL = compatibleHLSURL
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
	return chooseTranscode(plan, compatibleHLSURL, reasonCode, reason), nil
}

func applyProbeToPlaybackInfo(mediaID string, info *MediaPlayInfo, probe *model.MediaProbeRecord) *PlaybackSourceTechnical {
	technical, preferredAudio := playbackTechnicalFromProbe(probe)
	if info == nil || probe == nil {
		return technical
	}
	if probe.VideoCodec != "" {
		info.VideoCodec = probe.VideoCodec
	}
	if preferredAudio != "" {
		info.AudioCodec = preferredAudio
	}
	if probe.DurationMS > 0 {
		info.Duration = float64(probe.DurationMS) / 1000
	}

	videoCompatible := browserCompatibleVideoCodecs[strings.ToLower(strings.TrimSpace(info.VideoCodec))]
	audioCodec := strings.ToLower(strings.TrimSpace(info.AudioCodec))
	audioCompatible := audioCodec == "" || browserCompatibleAudioCodecs[audioCodec]
	info.CanDirectPlay = directPlayableExts[strings.ToLower(info.FileExt)] && videoCompatible && audioCompatible
	info.CanRemux = !info.CanDirectPlay && remuxableExts[strings.ToLower(info.FileExt)] && videoCompatible && audioCompatible
	if info.CanDirectPlay {
		info.DirectPlayURL = fmt.Sprintf("/api/stream/%s/direct", mediaID)
	} else {
		info.DirectPlayURL = ""
	}
	if info.CanRemux {
		info.RemuxURL = fmt.Sprintf("/api/stream/%s/remux", mediaID)
	} else {
		info.RemuxURL = ""
	}
	return technical
}

func playbackTechnicalFromProbe(probe *model.MediaProbeRecord) (*PlaybackSourceTechnical, string) {
	if probe == nil {
		return nil, ""
	}
	technical := &PlaybackSourceTechnical{
		ProbeVersion: probe.ProbeVersion,
		VideoCodec:   probe.VideoCodec,
		Width:        probe.Width,
		Height:       probe.Height,
		FrameRate:    probe.FrameRate(),
		PixelFormat:  probe.PixelFormat,
		BitDepth:     probe.BitDepth,
		HDR:          probe.HDR,
	}
	preferredAudio := ""
	defaultAudio := ""
	seen := make(map[string]struct{})
	for _, stream := range probe.AudioStreams() {
		codec := strings.ToLower(strings.TrimSpace(stream.Codec))
		if codec == "" {
			continue
		}
		if _, exists := seen[codec]; !exists {
			seen[codec] = struct{}{}
			technical.AudioCodecs = append(technical.AudioCodecs, codec)
		}
		if preferredAudio == "" {
			preferredAudio = codec
		}
		if stream.Default && defaultAudio == "" {
			defaultAudio = codec
		}
	}
	if defaultAudio != "" {
		preferredAudio = defaultAudio
	}
	return technical, preferredAudio
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