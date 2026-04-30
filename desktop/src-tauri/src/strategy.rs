
//! 播放策略决策引擎
//!
//! 根据文件特征（编码、容器、字幕、HDR）+ 用户设置，
//! 决定使用哪个播放内核。

use crate::settings::{PlayerEngine, Settings};
use serde::{Deserialize, Serialize};

/// 前端传来的媒体特征描述
#[derive(Debug, Clone, Deserialize)]
pub struct MediaProfile {
    /// 容器格式（mkv/mp4/ts/avi/flv/webm/iso/m2ts...）
    #[serde(default)]
    pub container: String,
    /// 视频编码（h264/hevc/av1/vp9/vc1/mpeg2...）
    #[serde(default)]
    pub video_codec: String,
    /// 音频编码（aac/mp3/ac3/eac3/dts/truehd/flac/opus...）
    #[serde(default)]
    pub audio_codec: String,
    /// 视频位深（8 / 10 / 12）
    #[serde(default)]
    pub bit_depth: u32,
    /// HDR 类型（none / hdr10 / hdr10plus / dolby_vision）
    #[serde(default)]
    pub hdr: String,
    /// 是否含 ASS / PGS 字幕
    #[serde(default)]
    pub has_complex_subtitle: bool,
    /// 分辨率高度
    #[serde(default)]
    pub height: u32,
    /// 是否为原盘（ISO / BDMV）
    #[serde(default)]
    pub is_bluray: bool,
}

/// 决策结果
#[derive(Debug, Clone, Serialize)]
pub struct EngineDecision {
    pub engine: String,     // "mpv" | "web"
    pub reason: String,     // 选择理由（展示给用户）
    pub confidence: String, // "strict" | "recommend" | "fallback"
}

/// 决策函数
pub fn decide(profile: &MediaProfile, settings: &Settings) -> EngineDecision {
    // 1. 用户强制指定
    match settings.player.engine {
        PlayerEngine::Mpv => {
            return EngineDecision {
                engine: "mpv".into(),
                reason: "用户设置：总是使用 mpv 内核".into(),
                confidence: "strict".into(),
            };
        }
        PlayerEngine::Web => {
            return EngineDecision {
                engine: "web".into(),
                reason: "用户设置：总是使用 Web 内核".into(),
                confidence: "strict".into(),
            };
        }
        PlayerEngine::Auto => {} // 走自动判定
    }

    // 2. 强烈推荐 mpv 的场景
    let container = profile.container.to_lowercase();
    let vcodec = profile.video_codec.to_lowercase();
    let acodec = profile.audio_codec.to_lowercase();
    let hdr = profile.hdr.to_lowercase();

    // 原盘必走 mpv
    if profile.is_bluray || container == "iso" || container == "bdmv" {
        return EngineDecision {
            engine: "mpv".into(),
            reason: "蓝光原盘，Web 无法播放".into(),
            confidence: "strict".into(),
        };
    }

    // 浏览器不支持的容器
    let bad_containers = ["mkv", "avi", "flv", "rmvb", "rm", "wmv", "asf", "ts", "m2ts", "mts"];
    if bad_containers.contains(&container.as_str()) {
        return EngineDecision {
            engine: "mpv".into(),
            reason: format!("容器 {} 浏览器不支持", container),
            confidence: "strict".into(),
        };
    }

    // 浏览器不兼容的视频编码
    let bad_vcodecs = ["hevc", "h265", "vc1", "mpeg2", "wmv3"];
    if bad_vcodecs.contains(&vcodec.as_str()) {
        return EngineDecision {
            engine: "mpv".into(),
            reason: format!("{} 编码浏览器兼容性差", vcodec.to_uppercase()),
            confidence: "recommend".into(),
        };
    }

    // AV1 浏览器支持不稳定，走 mpv 更稳
    if vcodec == "av1" {
        return EngineDecision {
            engine: "mpv".into(),
            reason: "AV1 编码，mpv 硬解更流畅".into(),
            confidence: "recommend".into(),
        };
    }

    // 浏览器不支持的音频
    let bad_acodecs = ["ac3", "eac3", "dts", "dtshd", "truehd", "atmos", "dolby"];
    if bad_acodecs.iter().any(|a| acodec.contains(a)) {
        return EngineDecision {
            engine: "mpv".into(),
            reason: format!("{} 音频浏览器不支持", acodec.to_uppercase()),
            confidence: "strict".into(),
        };
    }

    // HDR / 杜比视界
    if !hdr.is_empty() && hdr != "none" && hdr != "sdr" {
        return EngineDecision {
            engine: "mpv".into(),
            reason: format!("{} 内容，mpv 色彩管理更专业", hdr.to_uppercase()),
            confidence: "recommend".into(),
        };
    }

    // 10bit+
    if profile.bit_depth >= 10 {
        return EngineDecision {
            engine: "mpv".into(),
            reason: format!("{} bit 深度，浏览器支持有限", profile.bit_depth),
            confidence: "recommend".into(),
        };
    }

    // ASS / PGS
    if profile.has_complex_subtitle {
        return EngineDecision {
            engine: "mpv".into(),
            reason: "含 ASS/PGS 字幕，mpv 渲染完美".into(),
            confidence: "recommend".into(),
        };
    }

    // 4K+ 建议走 mpv（硬解稳）
    if profile.height >= 2160 {
        return EngineDecision {
            engine: "mpv".into(),
            reason: "4K+ 分辨率，mpv 硬解更稳".into(),
            confidence: "recommend".into(),
        };
    }

    // 默认 Web
    EngineDecision {
        engine: "web".into(),
        reason: "轻量内容，Web 播放启动快".into(),
        confidence: "fallback".into(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mkv_always_mpv() {
        let p = MediaProfile {
            container: "mkv".into(),
            video_codec: "h264".into(),
            audio_codec: "aac".into(),
            ..Default::default()
        };
        let s = Settings::default();
        let d = decide(&p, &s);
        assert_eq!(d.engine, "mpv");
    }

    #[test]
    fn test_mp4_h264_web() {
        let p = MediaProfile {
            container: "mp4".into(),
            video_codec: "h264".into(),
            audio_codec: "aac".into(),
            height: 1080,
            ..Default::default()
        };
        let s = Settings::default();
        let d = decide(&p, &s);
        assert_eq!(d.engine, "web");
    }

    #[test]
    fn test_hdr_goes_mpv() {
        let p = MediaProfile {
            container: "mp4".into(),
            video_codec: "hevc".into(),
            audio_codec: "aac".into(),
            hdr: "hdr10".into(),
            ..Default::default()
        };
        let s = Settings::default();
        let d = decide(&p, &s);
        assert_eq!(d.engine, "mpv");
    }

    #[test]
    fn test_user_force_web() {
        let p = MediaProfile {
            container: "mkv".into(),
            ..Default::default()
        };
        let mut s = Settings::default();
        s.player.engine = PlayerEngine::Web;
        let d = decide(&p, &s);
        assert_eq!(d.engine, "web");
    }
}

impl Default for MediaProfile {
    fn default() -> Self {
        Self {
            container: String::new(),
            video_codec: String::new(),
            audio_codec: String::new(),
            bit_depth: 8,
            hdr: String::new(),
            has_complex_subtitle: false,
            height: 0,
            is_bluray: false,
        }
    }
}
