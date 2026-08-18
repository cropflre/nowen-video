//! Nowen Video Desktop 2.0 Player Core。
//!
//! Desktop 正式运行时只有一个原生播放内核：libmpv。Web 播放能力属于浏览器平台，
//! 不进入 Rust Player Core，也不存在运行时引擎决策。
//!
//! Player Core 的控制 handle 与状态事件 handle 相互独立：控制命令继续通过主 Mpv
//! handle 执行；事件泵使用 `mpv_create_client` 创建独立 client，专门消费
//! `observe_property` / `mpv_wait_event`，避免事件队列与同步控制互相干扰。
//!
//! 轨道和章节列表不参与高频状态事件。它们通过 mpv 的标量子属性按需读取，
//! 只在文件加载、轨道变化、章节变化以及用户主动切换时刷新。

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
        app: &tauri::AppHandle,
        session_id: &str,
        url: &str,
        surface: surface::PlayerSurface,
        options: PlayOptions,
    ) -> Result<()> {
        self.stop(session_id);
        let player = native::NativePlayer::new(
            app.clone(),
            session_id,
            surface.native_window_id(),
            &options,
        )?;
        player.load(url)?;
        self.sessions.insert(session_id.to_string(), player);
        Ok(())
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn start(
        &mut self,
        _app: &tauri::AppHandle,
        _session_id: &str,
        _url: &str,
        _surface: surface::PlayerSurface,
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
    pub fn media_info(&self, session_id: &str) -> Result<PlayerMediaInfo> {
        Ok(self.session(session_id)?.media_info())
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn media_info(&self, _session_id: &str) -> Result<PlayerMediaInfo> {
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

#[derive(Debug, Clone, serde::Serialize, Default)]
pub struct PlayerTrack {
    pub id: i64,
    pub kind: String,
    pub title: String,
    pub language: String,
    pub codec: String,
    pub codec_desc: String,
    pub selected: bool,
    pub is_default: bool,
    pub forced: bool,
    pub external: bool,
}

#[derive(Debug, Clone, serde::Serialize, Default)]
pub struct PlayerChapter {
    pub index: i64,
    pub title: String,
    pub time: f64,
}

#[derive(Debug, Clone, serde::Serialize, Default)]
pub struct PlayerMediaInfo {
    pub tracks: Vec<PlayerTrack>,
    pub chapters: Vec<PlayerChapter>,
    pub current_chapter: i64,
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
    use super::{PlayOptions, PlayerChapter, PlayerMediaInfo, PlayerTrack};
    use anyhow::{anyhow, Result};
    use libmpv2::Mpv;
    use libmpv2_sys as sys;
    use std::ffi::{CStr, CString};
    use std::os::raw::{c_char, c_void};
    use std::sync::{
        atomic::{AtomicBool, Ordering},
        Arc, RwLock,
    };
    use std::thread::{self, JoinHandle};
    use std::time::{Duration, Instant};
    use tauri::{AppHandle, Emitter};

    const OBS_POSITION: u64 = 1;
    const OBS_DURATION: u64 = 2;
    const OBS_PAUSE: u64 = 3;
    const OBS_VOLUME: u64 = 4;
    const OBS_MUTE: u64 = 5;
    const OBS_WIDTH: u64 = 6;
    const OBS_HEIGHT: u64 = 7;
    const OBS_CODEC: u64 = 8;
    const OBS_CONTAINER: u64 = 9;
    const OBS_PIXEL_FORMAT: u64 = 10;
    const OBS_PRIMARIES: u64 = 11;
    const OBS_GAMMA: u64 = 12;
    const OBS_COLOR_MATRIX: u64 = 13;
    const OBS_AUDIO_SELECTION: u64 = 14;
    const OBS_SUB_SELECTION: u64 = 15;
    const OBS_CHAPTER_SELECTION: u64 = 16;
    const OBS_TRACK_LIST: u64 = 17;
    const OBS_CHAPTER_LIST: u64 = 18;

    fn map_err<T>(
        result: std::result::Result<T, libmpv2::Error>,
        message: impl std::fmt::Display,
    ) -> Result<T> {
        result.map_err(|error| anyhow!("{}: {:?}", message, error))
    }

    pub struct NativePlayer {
        mpv: Mpv,
        state: Arc<RwLock<NativeState>>,
        stop_events: Arc<AtomicBool>,
        event_thread: Option<JoinHandle<()>>,
    }

    impl NativePlayer {
        pub fn new(
            app: AppHandle,
            session_id: &str,
            native_window_id: i64,
            options: &PlayOptions,
        ) -> Result<Self> {
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

            let state = Arc::new(RwLock::new(NativeState::default()));
            let stop_events = Arc::new(AtomicBool::new(false));
            let event_thread = spawn_event_pump(
                &mpv,
                app,
                session_id.to_string(),
                Arc::clone(&state),
                Arc::clone(&stop_events),
            )?;

            Ok(Self {
                mpv,
                state,
                stop_events,
                event_thread: Some(event_thread),
            })
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
            self.state
                .read()
                .map(|state| state.info.clone())
                .unwrap_or_default()
        }

        pub fn media_info(&self) -> PlayerMediaInfo {
            let track_count = self
                .mpv
                .get_property::<i64>("track-list/count")
                .unwrap_or(0)
                .clamp(0, 256);
            let mut tracks = Vec::with_capacity(track_count as usize);

            for index in 0..track_count {
                let prefix = format!("track-list/{index}");
                let kind = self
                    .mpv
                    .get_property::<String>(&format!("{prefix}/type"))
                    .unwrap_or_default();
                if kind.is_empty() {
                    continue;
                }

                tracks.push(PlayerTrack {
                    id: self
                        .mpv
                        .get_property::<i64>(&format!("{prefix}/id"))
                        .unwrap_or(0),
                    kind,
                    title: self
                        .mpv
                        .get_property::<String>(&format!("{prefix}/title"))
                        .unwrap_or_default(),
                    language: self
                        .mpv
                        .get_property::<String>(&format!("{prefix}/lang"))
                        .unwrap_or_default(),
                    codec: self
                        .mpv
                        .get_property::<String>(&format!("{prefix}/codec"))
                        .unwrap_or_default(),
                    codec_desc: self
                        .mpv
                        .get_property::<String>(&format!("{prefix}/codec-desc"))
                        .unwrap_or_default(),
                    selected: self
                        .mpv
                        .get_property::<bool>(&format!("{prefix}/selected"))
                        .unwrap_or(false),
                    is_default: self
                        .mpv
                        .get_property::<bool>(&format!("{prefix}/default"))
                        .unwrap_or(false),
                    forced: self
                        .mpv
                        .get_property::<bool>(&format!("{prefix}/forced"))
                        .unwrap_or(false),
                    external: self
                        .mpv
                        .get_property::<bool>(&format!("{prefix}/external"))
                        .unwrap_or(false),
                });
            }

            let chapter_count = self
                .mpv
                .get_property::<i64>("chapter-list/count")
                .unwrap_or(0)
                .clamp(0, 4096);
            let mut chapters = Vec::with_capacity(chapter_count as usize);
            for index in 0..chapter_count {
                let prefix = format!("chapter-list/{index}");
                let title = self
                    .mpv
                    .get_property::<String>(&format!("{prefix}/title"))
                    .unwrap_or_else(|_| format!("章节 {}", index + 1));
                chapters.push(PlayerChapter {
                    index,
                    title: if title.trim().is_empty() {
                        format!("章节 {}", index + 1)
                    } else {
                        title
                    },
                    time: self
                        .mpv
                        .get_property::<f64>(&format!("{prefix}/time"))
                        .unwrap_or(0.0),
                });
            }

            PlayerMediaInfo {
                tracks,
                chapters,
                current_chapter: self.mpv.get_property::<i64>("chapter").unwrap_or(-1),
            }
        }
    }

    impl Drop for NativePlayer {
        fn drop(&mut self) {
            self.stop_events.store(true, Ordering::Release);
            if let Some(thread) = self.event_thread.take() {
                let _ = thread.join();
            }
        }
    }

    #[derive(Debug, Clone, serde::Serialize, Default)]
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

    #[derive(Debug, Default)]
    struct NativeState {
        info: VideoInfo,
        color_matrix: String,
    }

    #[derive(Debug, Clone, serde::Serialize)]
    struct PlayerStatePayload {
        session_id: String,
        event: String,
        state: VideoInfo,
    }

    fn spawn_event_pump(
        mpv: &Mpv,
        app: AppHandle,
        session_id: String,
        state: Arc<RwLock<NativeState>>,
        stop: Arc<AtomicBool>,
    ) -> Result<JoinHandle<()>> {
        let client_name = CString::new("nowen_player_state").expect("固定 client 名称无 NUL");
        let client = unsafe { sys::mpv_create_client(mpv.ctx.as_ptr(), client_name.as_ptr()) };
        if client.is_null() {
            return Err(anyhow!("创建 Player Core 事件 client 失败"));
        }

        // raw pointer 本身不跨线程解引用；转成地址值后由唯一事件线程恢复并负责 destroy。
        let client_addr = client as usize;
        match thread::Builder::new()
            .name(format!("player-events-{}", session_id))
            .spawn(move || {
                let client = client_addr as *mut sys::mpv_handle;
                run_event_pump(client, app, session_id, state, stop);
                unsafe { sys::mpv_destroy(client) };
            })
        {
            Ok(handle) => Ok(handle),
            Err(error) => {
                unsafe { sys::mpv_destroy(client) };
                Err(anyhow!("创建 Player Core 事件线程失败: {}", error))
            }
        }
    }

    fn run_event_pump(
        client: *mut sys::mpv_handle,
        app: AppHandle,
        session_id: String,
        state: Arc<RwLock<NativeState>>,
        stop: Arc<AtomicBool>,
    ) {
        register_observations(client);

        let emit_interval = Duration::from_millis(33);
        let mut last_emit = Instant::now() - emit_interval;
        let mut dirty = true;
        let mut event_kind = "state";

        while !stop.load(Ordering::Acquire) {
            let event_ptr = unsafe { sys::mpv_wait_event(client, 0.05) };
            if event_ptr.is_null() {
                continue;
            }
            let event = unsafe { &*event_ptr };

            match event.event_id {
                sys::mpv_event_id_MPV_EVENT_NONE => {}
                sys::mpv_event_id_MPV_EVENT_PROPERTY_CHANGE => {
                    if is_media_info_observation(event.reply_userdata) {
                        dirty = true;
                        event_kind = "media-info-change";
                    } else if event.error >= 0
                        && apply_property_change(event.reply_userdata, event.data, &state)
                    {
                        dirty = true;
                        event_kind = "state";
                    }
                }
                sys::mpv_event_id_MPV_EVENT_FILE_LOADED => {
                    dirty = true;
                    event_kind = "file-loaded";
                }
                sys::mpv_event_id_MPV_EVENT_PLAYBACK_RESTART => {
                    dirty = true;
                    event_kind = "playback-restart";
                }
                sys::mpv_event_id_MPV_EVENT_END_FILE => {
                    if let Ok(mut current) = state.write() {
                        current.info.paused = true;
                    }
                    dirty = true;
                    event_kind = "end-file";
                }
                sys::mpv_event_id_MPV_EVENT_QUEUE_OVERFLOW => {
                    log::warn!("Player Core 事件队列溢出: {}", session_id);
                    dirty = true;
                    event_kind = "queue-overflow";
                }
                sys::mpv_event_id_MPV_EVENT_SHUTDOWN => break,
                _ => {}
            }

            let force_emit = event_kind != "state";
            if dirty
                && (force_emit
                    || last_emit.elapsed() >= emit_interval
                    || event.event_id == sys::mpv_event_id_MPV_EVENT_NONE)
            {
                emit_state(&app, &session_id, event_kind, &state);
                dirty = false;
                event_kind = "state";
                last_emit = Instant::now();
            }
        }

        if dirty {
            emit_state(&app, &session_id, event_kind, &state);
        }
    }

    fn register_observations(client: *mut sys::mpv_handle) {
        let observations = [
            (OBS_POSITION, "time-pos", sys::mpv_format_MPV_FORMAT_DOUBLE),
            (OBS_DURATION, "duration", sys::mpv_format_MPV_FORMAT_DOUBLE),
            (OBS_PAUSE, "pause", sys::mpv_format_MPV_FORMAT_FLAG),
            (OBS_VOLUME, "volume", sys::mpv_format_MPV_FORMAT_DOUBLE),
            (OBS_MUTE, "mute", sys::mpv_format_MPV_FORMAT_FLAG),
            (OBS_WIDTH, "video-params/w", sys::mpv_format_MPV_FORMAT_INT64),
            (OBS_HEIGHT, "video-params/h", sys::mpv_format_MPV_FORMAT_INT64),
            (OBS_CODEC, "video-codec-name", sys::mpv_format_MPV_FORMAT_STRING),
            (OBS_CONTAINER, "file-format", sys::mpv_format_MPV_FORMAT_STRING),
            (
                OBS_PIXEL_FORMAT,
                "video-params/pixelformat",
                sys::mpv_format_MPV_FORMAT_STRING,
            ),
            (
                OBS_PRIMARIES,
                "video-params/primaries",
                sys::mpv_format_MPV_FORMAT_STRING,
            ),
            (OBS_GAMMA, "video-params/gamma", sys::mpv_format_MPV_FORMAT_STRING),
            (
                OBS_COLOR_MATRIX,
                "video-params/colormatrix",
                sys::mpv_format_MPV_FORMAT_STRING,
            ),
            (OBS_AUDIO_SELECTION, "aid", sys::mpv_format_MPV_FORMAT_NONE),
            (OBS_SUB_SELECTION, "sid", sys::mpv_format_MPV_FORMAT_NONE),
            (
                OBS_CHAPTER_SELECTION,
                "chapter",
                sys::mpv_format_MPV_FORMAT_NONE,
            ),
            (OBS_TRACK_LIST, "track-list", sys::mpv_format_MPV_FORMAT_NONE),
            (
                OBS_CHAPTER_LIST,
                "chapter-list",
                sys::mpv_format_MPV_FORMAT_NONE,
            ),
        ];

        for (id, name, format) in observations {
            let Ok(name_c) = CString::new(name) else {
                continue;
            };
            let result = unsafe { sys::mpv_observe_property(client, id, name_c.as_ptr(), format) };
            if result < 0 {
                log::warn!("订阅 Player Core 属性失败: {} ({})", name, result);
            }
        }
    }

    fn is_media_info_observation(id: u64) -> bool {
        matches!(
            id,
            OBS_AUDIO_SELECTION
                | OBS_SUB_SELECTION
                | OBS_CHAPTER_SELECTION
                | OBS_TRACK_LIST
                | OBS_CHAPTER_LIST
        )
    }

    fn apply_property_change(
        id: u64,
        data: *mut c_void,
        state: &Arc<RwLock<NativeState>>,
    ) -> bool {
        if data.is_null() {
            return false;
        }
        let property = unsafe { &*(data as *const sys::mpv_event_property) };
        if property.data.is_null() || property.format == sys::mpv_format_MPV_FORMAT_NONE {
            return false;
        }

        let Ok(mut current) = state.write() else {
            return false;
        };

        match id {
            OBS_POSITION => {
                current.info.position = unsafe { read_double(property.data) }.unwrap_or(0.0)
            }
            OBS_DURATION => {
                current.info.duration = unsafe { read_double(property.data) }.unwrap_or(0.0)
            }
            OBS_PAUSE => {
                current.info.paused = unsafe { read_flag(property.data) }.unwrap_or(false)
            }
            OBS_VOLUME => {
                current.info.volume = unsafe { read_double(property.data) }.unwrap_or(0.0)
            }
            OBS_MUTE => current.info.mute = unsafe { read_flag(property.data) }.unwrap_or(false),
            OBS_WIDTH => {
                current.info.width = unsafe { read_i64(property.data) }.unwrap_or(0).max(0) as u32
            }
            OBS_HEIGHT => {
                current.info.height = unsafe { read_i64(property.data) }.unwrap_or(0).max(0) as u32
            }
            OBS_CODEC => {
                current.info.codec = unsafe { read_string(property.data) }.unwrap_or_default()
            }
            OBS_CONTAINER => {
                current.info.container = unsafe { read_string(property.data) }.unwrap_or_default()
            }
            OBS_PIXEL_FORMAT => {
                current.info.pixel_format =
                    unsafe { read_string(property.data) }.unwrap_or_default()
            }
            OBS_PRIMARIES => {
                current.info.primaries = unsafe { read_string(property.data) }.unwrap_or_default()
            }
            OBS_GAMMA => {
                current.info.gamma = unsafe { read_string(property.data) }.unwrap_or_default()
            }
            OBS_COLOR_MATRIX => {
                current.color_matrix = unsafe { read_string(property.data) }.unwrap_or_default()
            }
            _ => return false,
        }

        refresh_hdr(&mut current);
        true
    }

    fn refresh_hdr(state: &mut NativeState) {
        let gamma = state.info.gamma.to_ascii_lowercase();
        let primaries = state.info.primaries.to_ascii_lowercase();
        let matrix = state.color_matrix.to_ascii_lowercase();
        state.info.hdr = if gamma.contains("dolbyvision") || matrix.contains("dovi") {
            "DoVi"
        } else if gamma.contains("pq") && primaries.contains("bt.2020") {
            "HDR10"
        } else if gamma.contains("hlg") {
            "HLG"
        } else {
            "SDR"
        }
        .to_string();
    }

    fn emit_state(
        app: &AppHandle,
        session_id: &str,
        event: &str,
        state: &Arc<RwLock<NativeState>>,
    ) {
        let Ok(current) = state.read() else {
            return;
        };
        let payload = PlayerStatePayload {
            session_id: session_id.to_string(),
            event: event.to_string(),
            state: current.info.clone(),
        };
        if let Err(error) = app.emit("player-state", payload) {
            log::debug!("发送 Player Core 状态事件失败: {}", error);
        }
    }

    unsafe fn read_double(data: *mut c_void) -> Option<f64> {
        (!data.is_null()).then(|| *(data as *const f64))
    }

    unsafe fn read_i64(data: *mut c_void) -> Option<i64> {
        (!data.is_null()).then(|| *(data as *const i64))
    }

    unsafe fn read_flag(data: *mut c_void) -> Option<bool> {
        (!data.is_null()).then(|| *(data as *const i32) != 0)
    }

    unsafe fn read_string(data: *mut c_void) -> Option<String> {
        if data.is_null() {
            return None;
        }
        let value = *(data as *const *const c_char);
        if value.is_null() {
            return None;
        }
        Some(CStr::from_ptr(value).to_string_lossy().into_owned())
    }
}
