//! Nowen Video Desktop 2.0 Player Core。
//!
//! Desktop 正式运行时只有一个原生播放内核：libmpv。Web 播放能力属于浏览器平台，
//! 不进入 Rust Player Core，也不存在运行时引擎决策。

pub mod surface;

use crate::settings::Settings;
use anyhow::{anyhow, Result};
use std::collections::HashMap;

pub struct PlayerManager {
    #[cfg(feature = "embed-mpv")]
    sessions: HashMap<String, native::NativePlayer>,
}

impl PlayerManager {
    pub fn new(_settings: Settings) -> Self {
        Self {
            #[cfg(feature = "embed-mpv")]
            sessions: HashMap::new(),
        }
    }

    pub fn is_available(&self) -> bool {
        cfg!(feature = "embed-mpv")
    }

    pub fn stop(&mut self, session_id: &str) {
        #[cfg(feature = "embed-mpv")]
        if self.sessions.remove(session_id).is_some() {
            log::info!("已释放 Player Core 会话: {}", session_id);
        }
    }

    pub fn stop_all(&mut self) {
        #[cfg(feature = "embed-mpv")]
        {
            if !self.sessions.is_empty() {
                log::info!("释放全部 Player Core 会话: {}", self.sessions.len());
            }
            self.sessions.clear();
        }
    }

    #[cfg(feature = "embed-mpv")]
    pub fn start(
        &mut self,
        session_id: &str,
        url: &str,
        native_window_id: i64,
        options: PlayOptions,
    ) -> Result<()> {
        self.stop(session_id);
        let player = native::NativePlayer::new(native_window_id, &options)?;
        player.load(url)?;
        self.sessions.insert(session_id.to_string(), player);
        Ok(())
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn start(
        &mut self,
        _session_id: &str,
        _url: &str,
        _native_window_id: i64,
        _options: PlayOptions,
    ) -> Result<()> {
        Err(anyhow!("当前构建未启用 Desktop Player Core"))
    }

    #[cfg(feature = "embed-mpv")]
    pub fn command(&self, session_id: &str, command: &str, args: &[String]) -> Result<()> {
        self.session(session_id)?.command(command, args)
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn command(&self, _session_id: &str, _command: &str, _args: &[String]) -> Result<()> {
        Err(anyhow!("当前构建未启用 Desktop Player Core"))
    }

    #[cfg(feature = "embed-mpv")]
    pub fn set_property(&self, session_id: &str, name: &str, value: &str) -> Result<()> {
        self.session(session_id)?.set_property(name, value)
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn set_property(&self, _session_id: &str, _name: &str, _value: &str) -> Result<()> {
        Err(anyhow!("当前构建未启用 Desktop Player Core"))
    }

    #[cfg(feature = "embed-mpv")]
    pub fn video_info(&self, session_id: &str) -> Result<PlayerVideoInfo> {
        Ok(self.session(session_id)?.video_info())
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn video_info(&self, _session_id: &str) -> Result<PlayerVideoInfo> {
        Err(anyhow!("当前构建未启用 Desktop Player Core"))
    }

    #[cfg(feature = "embed-mpv")]
    fn session(&self, session_id: &str) -> Result<&native::NativePlayer> {
        self.sessions
            .get(session_id)
            .ok_or_else(|| anyhow!("Player Core 会话不存在: {}", session_id))
    }
}

#[derive(Debug, Default, Clone, serde::Deserialize)]
pub struct PlayOptions {
    #[serde(default)]
    pub title: Option<String>,
    #[serde(default)]
    pub start_time: Option<f64>,
    #[serde(default)]
    pub subtitles: Vec<String>,
    #[serde(default)]
    pub audio_lang: Option<String>,
    #[serde(default)]
    pub sub_lang: Option<String>,
    #[serde(default)]
    pub fullscreen: bool,
    #[serde(default)]
    pub http_headers: HashMap<String, String>,
    #[serde(default)]
    pub user_agent: Option<String>,
}

#[cfg(feature = "embed-mpv")]
pub type PlayerVideoInfo = native::VideoInfo;

#[cfg(not(feature = "embed-mpv"))]
#[derive(Debug, Clone, serde::Serialize, Default)]
pub struct PlayerVideoInfo {
    pub width: u32,
    pub height: u32,
    pub codec: String,
    pub container: String,
    pub duration: f64,
    pub position: f64,
    pub pixel_format: String,
    pub primaries: String,
    pub gamma: String,
    pub hdr: String,
    pub paused: bool,
    pub volume: f64,
    pub mute: bool,
}

#[cfg(feature = "embed-mpv")]
mod native {
    use super::PlayOptions;
    use anyhow::{anyhow, Result};
    use libmpv2::Mpv;

    fn map_err<T>(
        result: std::result::Result<T, libmpv2::Error>,
        message: impl std::fmt::Display,
    ) -> Result<T> {
        result.map_err(|error| anyhow!("{}: {:?}", message, error))
    }

    pub struct NativePlayer {
        mpv: Mpv,
    }

    impl NativePlayer {
        pub fn new(native_window_id: i64, options: &PlayOptions) -> Result<Self> {
            let mpv = map_err(Mpv::new(), "创建 libmpv 实例失败")?;
            map_err(
                mpv.set_property("wid", native_window_id),
                "绑定 Player Surface 失败",
            )?;

            mpv.set_property("keep-open", "yes").ok();
            mpv.set_property("force-window", "yes").ok();
            mpv.set_property("input-default-bindings", "yes").ok();
            mpv.set_property("input-vo-keyboard", "yes").ok();
            mpv.set_property("osc", "no").ok();
            mpv.set_property("idle", "yes").ok();
            mpv.set_property("msg-level", "all=warn").ok();

            #[cfg(target_os = "windows")]
            {
                mpv.set_property("vo", "gpu-next").ok();
                mpv.set_property("gpu-api", "d3d11").ok();
                mpv.set_property("gpu-context", "d3d11").ok();
                mpv.set_property("hwdec", "d3d11va-copy").ok();
                mpv.set_property("d3d11-flip", "yes").ok();
                mpv.set_property("d3d11-sync-interval", "1").ok();
            }

            #[cfg(not(target_os = "windows"))]
            {
                mpv.set_property("vo", "gpu-next").ok();
                mpv.set_property("hwdec", "auto-safe").ok();
            }

            mpv.set_property("target-colorspace-hint", "yes").ok();
            mpv.set_property("tone-mapping", "bt.2446a").ok();
            mpv.set_property("tone-mapping-mode", "rgb").ok();
            mpv.set_property("hdr-compute-peak", "yes").ok();
            mpv.set_property("icc-profile-auto", "yes").ok();

            mpv.set_property("cache", "yes").ok();
            mpv.set_property("demuxer-max-bytes", "400MiB").ok();
            mpv.set_property("demuxer-max-back-bytes", "100MiB").ok();
            mpv.set_property("cache-secs", "30").ok();

            mpv.set_property("sub-auto", "fuzzy").ok();
            mpv.set_property("sub-font-provider", "auto").ok();
            mpv.set_property("blend-subtitles", "yes").ok();
            if let Some(fonts) = crate::resources::font_dir() {
                if fonts.exists() {
                    let path = fonts.to_string_lossy().to_string();
                    mpv.set_property("sub-fonts-dir", path.clone()).ok();
                    mpv.set_property("osd-fonts-dir", path).ok();
                }
            }

            mpv.set_property("audio-channels", "auto-safe").ok();
            mpv.set_property("audio-spdif", "ac3,dts,eac3,truehd,dts-hd").ok();

            if let Some(start) = options.start_time {
                mpv.set_property("start", format!("+{}", start)).ok();
            }
            if let Some(language) = &options.audio_lang {
                mpv.set_property("alang", language.clone()).ok();
            }
            if let Some(language) = &options.sub_lang {
                mpv.set_property("slang", language.clone()).ok();
            }
            if let Some(user_agent) = &options.user_agent {
                mpv.set_property("user-agent", user_agent.clone()).ok();
            }
            if !options.http_headers.is_empty() {
                let headers = options
                    .http_headers
                    .iter()
                    .map(|(key, value)| format!("{}: {}", key, value))
                    .collect::<Vec<_>>()
                    .join("\r\n");
                mpv.set_property("http-header-fields", headers).ok();
            }
            for subtitle in &options.subtitles {
                let _ = mpv.command("sub-add", &[subtitle, "auto"]);
            }

            Ok(Self { mpv })
        }

        pub fn load(&self, url: &str) -> Result<()> {
            map_err(self.mpv.command("loadfile", &[url, "replace"]), "加载媒体失败")
        }

        pub fn command(&self, command: &str, args: &[String]) -> Result<()> {
            let refs: Vec<&str> = args.iter().map(String::as_str).collect();
            map_err(
                self.mpv.command(command, &refs),
                format!("执行播放器命令 {} 失败", command),
            )
        }

        pub fn set_property(&self, name: &str, value: &str) -> Result<()> {
            map_err(
                self.mpv.set_property(name, value.to_string()),
                format!("设置播放器属性 {} 失败", name),
            )
        }

        pub fn video_info(&self) -> VideoInfo {
            let get_string = |property: &str| {
                self.mpv.get_property::<String>(property).unwrap_or_default()
            };
            let get_i64 = |property: &str| self.mpv.get_property::<i64>(property).unwrap_or(0);
            let get_f64 = |property: &str| self.mpv.get_property::<f64>(property).unwrap_or(0.0);

            let pixel_format = get_string("video-params/pixelformat");
            let color_matrix = get_string("video-params/colormatrix");
            let primaries = get_string("video-params/primaries");
            let gamma = get_string("video-params/gamma");

            let hdr = if gamma.contains("pq") && primaries.contains("bt.2020") {
                "HDR10"
            } else if gamma.contains("hlg") {
                "HLG"
            } else if gamma.contains("dolbyvision") || color_matrix.contains("dovi") {
                "DoVi"
            } else {
                "SDR"
            };

            VideoInfo {
                width: get_i64("video-params/w") as u32,
                height: get_i64("video-params/h") as u32,
                codec: get_string("video-codec-name"),
                container: get_string("file-format"),
                duration: get_f64("duration"),
                position: get_f64("time-pos"),
                pixel_format,
                primaries,
                gamma,
                hdr: hdr.to_string(),
                paused: self.mpv.get_property::<bool>("pause").unwrap_or(false),
                volume: get_f64("volume"),
                mute: self.mpv.get_property::<bool>("mute").unwrap_or(false),
            }
        }
    }

    #[derive(Debug, Clone, serde::Serialize)]
    pub struct VideoInfo {
        pub width: u32,
        pub height: u32,
        pub codec: String,
        pub container: String,
        pub duration: f64,
        pub position: f64,
        pub pixel_format: String,
        pub primaries: String,
        pub gamma: String,
        pub hdr: String,
        pub paused: bool,
        pub volume: f64,
        pub mute: bool,
    }

    unsafe impl Send for NativePlayer {}
    unsafe impl Sync for NativePlayer {}
}
