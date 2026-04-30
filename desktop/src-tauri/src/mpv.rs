
//! mpv 播放器管理 —— 双模式
//!
//! - **外部进程模式**（默认）：系统 mpv 二进制 + 独立窗口
//!   - 优点：跨平台一致、打包不依赖动态库、易于调试
//!   - 适用：90% 场景
//!
//! - **libmpv 嵌入模式**（M4，启用 feature `embed-mpv`）：
//!   - 通过 FFI 调用 libmpv，支持传入窗口 handle 嵌入 Tauri 窗口内渲染
//!   - 适用：追求"一个窗口、前端控制"的无缝体验
//!   - 运行时需系统安装 libmpv 动态库

use crate::settings::Settings;
use anyhow::{anyhow, Context, Result};
use std::collections::HashMap;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

pub struct MpvManager {
    settings: Settings,
    /// 外部进程模式的活动 mpv 进程 —— key 为 session_id
    sessions: HashMap<String, Child>,

    /// libmpv 嵌入模式 —— key 为 session_id
    #[cfg(feature = "embed-mpv")]
    embedded: HashMap<String, embed::EmbeddedMpv>,
}

impl MpvManager {
    pub fn new(settings: Settings) -> Self {
        Self {
            settings,
            sessions: HashMap::new(),
            #[cfg(feature = "embed-mpv")]
            embedded: HashMap::new(),
        }
    }

    /// 是否系统可用 mpv（外部进程）
    pub fn is_available(&self) -> bool {
        self.resolve_mpv_path().is_ok()
    }

    /// libmpv 嵌入是否可用
    pub fn is_embed_available(&self) -> bool {
        #[cfg(feature = "embed-mpv")]
        {
            true
        }
        #[cfg(not(feature = "embed-mpv"))]
        {
            false
        }
    }

    /// 查找 mpv 二进制路径
    fn resolve_mpv_path(&self) -> Result<PathBuf> {
        // 用户自定义优先
        let custom = self.settings.player.mpv_path.trim();
        if !custom.is_empty() {
            let p = PathBuf::from(custom);
            if p.exists() {
                return Ok(p);
            }
            log::warn!("用户指定的 mpv 路径不存在: {}", custom);
        }

        // 系统 PATH 查找
        if let Ok(p) = which::which("mpv") {
            return Ok(p);
        }

        // Windows 常见位置兜底
        #[cfg(target_os = "windows")]
        {
            let candidates = [
                r"C:\Program Files\mpv\mpv.exe",
                r"C:\Program Files (x86)\mpv\mpv.exe",
                r"C:\mpv\mpv.exe",
            ];
            for c in &candidates {
                let p = PathBuf::from(c);
                if p.exists() {
                    return Ok(p);
                }
            }
        }

        // macOS 常见位置
        #[cfg(target_os = "macos")]
        {
            let candidates = [
                "/opt/homebrew/bin/mpv",
                "/usr/local/bin/mpv",
                "/Applications/mpv.app/Contents/MacOS/mpv",
            ];
            for c in &candidates {
                let p = PathBuf::from(c);
                if p.exists() {
                    return Ok(p);
                }
            }
        }

        Err(anyhow!(
            "未找到 mpv 可执行文件，请在设置中手动指定路径，或安装 mpv:\n\
            - Windows: winget install mpv\n\
            - macOS:   brew install mpv\n\
            - Linux:   apt install mpv"
        ))
    }

    /// 启动播放（外部进程）
    pub fn play(&mut self, session_id: &str, url: &str, options: PlayOptions) -> Result<()> {
        // 若同 session 已在播放，先停止
        if self.sessions.contains_key(session_id) {
            self.stop(session_id);
        }

        let mpv_path = self.resolve_mpv_path()?;
        log::info!("mpv: {}", mpv_path.display());
        log::info!("  url: {}", url);

        let mut cmd = Command::new(&mpv_path);

        // 默认参数
        for arg in &self.settings.player.mpv_args {
            cmd.arg(arg);
        }

        // 窗口标题
        if let Some(title) = &options.title {
            cmd.arg(format!("--title={}", title));
        }

        // 起始位置
        if let Some(start) = options.start_time {
            cmd.arg(format!("--start={}", start));
        }

        // 字幕
        for sub in &options.subtitles {
            cmd.arg(format!("--sub-file={}", sub));
        }

        // 音频语言首选
        if let Some(alang) = &options.audio_lang {
            cmd.arg(format!("--alang={}", alang));
        }

        // 字幕语言首选
        if let Some(slang) = &options.sub_lang {
            cmd.arg(format!("--slang={}", slang));
        }

        // 全屏
        if options.fullscreen {
            cmd.arg("--fullscreen");
        }

        // HTTP 请求头（认证 token 等）
        for (k, v) in &options.http_headers {
            cmd.arg(format!("--http-header-fields-append={}: {}", k, v));
        }

        // User-Agent
        if let Some(ua) = &options.user_agent {
            cmd.arg(format!("--user-agent={}", ua));
        }

        // 媒体 URL（必须放在最后）
        cmd.arg(url);

        cmd.stdout(Stdio::null())
            .stderr(Stdio::piped())
            .stdin(Stdio::null());

        // Windows 隐藏命令行窗口
        #[cfg(target_os = "windows")]
        {
            const CREATE_NO_WINDOW: u32 = 0x08000000;
            cmd.creation_flags(CREATE_NO_WINDOW);
        }

        let child = cmd.spawn().context("启动 mpv 失败")?;
        log::info!("mpv 已启动 session={} PID={}", session_id, child.id());
        self.sessions.insert(session_id.to_string(), child);
        Ok(())
    }

    /// 停止指定 session（外部进程）
    pub fn stop(&mut self, session_id: &str) {
        if let Some(mut child) = self.sessions.remove(session_id) {
            log::info!("停止 mpv session={} PID={}", session_id, child.id());
            let _ = child.kill();
            let _ = child.wait();
        }

        #[cfg(feature = "embed-mpv")]
        {
            if self.embedded.remove(session_id).is_some() {
                log::info!("已销毁嵌入 mpv session={}", session_id);
            }
        }
    }

    /// 停止所有
    pub fn stop_all(&mut self) {
        let ids: Vec<String> = self.sessions.keys().cloned().collect();
        for id in ids {
            self.stop(&id);
        }
        #[cfg(feature = "embed-mpv")]
        {
            self.embedded.clear();
        }
    }

    /// 清理已退出的 session
    pub fn cleanup_exited(&mut self) {
        let mut exited = Vec::new();
        for (id, child) in self.sessions.iter_mut() {
            if let Ok(Some(_)) = child.try_wait() {
                exited.push(id.clone());
            }
        }
        for id in exited {
            log::info!("mpv session={} 已退出，清理", id);
            self.sessions.remove(&id);
        }
    }

    /// 当前活跃 session 数
    pub fn active_count(&mut self) -> usize {
        self.cleanup_exited();
        self.sessions.len()
    }

    // ============ libmpv 嵌入接口（M4） ============

    /// 启动嵌入式播放（传入窗口 handle）
    #[cfg(feature = "embed-mpv")]
    pub fn play_embedded(
        &mut self,
        session_id: &str,
        url: &str,
        wid: i64,
        options: PlayOptions,
    ) -> Result<()> {
        if let Some(m) = self.embedded.remove(session_id) {
            drop(m);
        }
        let m = embed::EmbeddedMpv::new(wid, &options)?;
        m.loadfile(url)?;
        self.embedded.insert(session_id.to_string(), m);
        Ok(())
    }

    #[cfg(feature = "embed-mpv")]
    pub fn send_embed_command(&mut self, session_id: &str, cmd: &str, args: &[String]) -> Result<()> {
        let m = self
            .embedded
            .get_mut(session_id)
            .ok_or_else(|| anyhow!("嵌入会话不存在: {}", session_id))?;
        m.command(cmd, args)
    }

    #[cfg(feature = "embed-mpv")]
    pub fn set_embed_property(&mut self, session_id: &str, name: &str, value: &str) -> Result<()> {
        let m = self
            .embedded
            .get_mut(session_id)
            .ok_or_else(|| anyhow!("嵌入会话不存在: {}", session_id))?;
        m.set_property(name, value)
    }

    #[cfg(feature = "embed-mpv")]
    pub fn set_embed_anime4k(&mut self, session_id: &str, level: &str) -> Result<()> {
        let m = self
            .embedded
            .get_mut(session_id)
            .ok_or_else(|| anyhow!("嵌入会话不存在: {}", session_id))?;
        m.set_anime4k(level)
    }

    #[cfg(feature = "embed-mpv")]
    pub fn get_embed_video_info(&self, session_id: &str) -> Result<EmbedVideoInfo> {
        let m = self
            .embedded
            .get(session_id)
            .ok_or_else(|| anyhow!("嵌入会话不存在: {}", session_id))?;
        Ok(m.get_video_info())
    }

    // ==== 未启用 embed 特性时的占位（保证前端接口调用不会崩溃） ====

    #[cfg(not(feature = "embed-mpv"))]
    pub fn play_embedded(
        &mut self,
        _session_id: &str,
        _url: &str,
        _wid: i64,
        _options: PlayOptions,
    ) -> Result<()> {
        Err(anyhow!(
            "当前构建未启用 libmpv 嵌入模式，请用 `cargo build --features embed-mpv` 重新编译，\
             或继续使用外部进程模式 play()"
        ))
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn send_embed_command(
        &mut self,
        _session_id: &str,
        _cmd: &str,
        _args: &[String],
    ) -> Result<()> {
        Err(anyhow!("未启用 embed-mpv feature"))
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn set_embed_property(
        &mut self,
        _session_id: &str,
        _name: &str,
        _value: &str,
    ) -> Result<()> {
        Err(anyhow!("未启用 embed-mpv feature"))
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn set_embed_anime4k(&mut self, _session_id: &str, _level: &str) -> Result<()> {
        Err(anyhow!("未启用 embed-mpv feature"))
    }

    #[cfg(not(feature = "embed-mpv"))]
    pub fn get_embed_video_info(&self, _session_id: &str) -> Result<EmbedVideoInfo> {
        Err(anyhow!("未启用 embed-mpv feature"))
    }
}

/// 播放选项
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

/// 播放器实时信息快照（对外统一类型）
#[cfg(feature = "embed-mpv")]
pub type EmbedVideoInfo = embed::VideoInfo;

/// 未启用 embed-mpv 时的占位类型（保证 commands.rs 能编译）
#[cfg(not(feature = "embed-mpv"))]
#[derive(Debug, Clone, serde::Serialize, Default)]
pub struct EmbedVideoInfo {
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

// ================= libmpv 嵌入实现 =================

#[cfg(feature = "embed-mpv")]
mod embed {
    use super::PlayOptions;
    use anyhow::{anyhow, Result};
    use libmpv2::{Mpv, SetData};

    /// libmpv2::Error -> anyhow::Error 桥接（libmpv2::Error 未实现 StdError，不能用 .context()）
    fn map_err<T>(r: std::result::Result<T, libmpv2::Error>, msg: impl std::fmt::Display) -> Result<T> {
        r.map_err(|e| anyhow!("{}: {:?}", msg, e))
    }

    pub struct EmbeddedMpv {
        mpv: Mpv,
    }

    impl EmbeddedMpv {
        pub fn new(wid: i64, options: &PlayOptions) -> Result<Self> {
            let mpv = map_err(Mpv::new(), "创建 libmpv 实例失败")?;

            // 嵌入窗口（将 libmpv 渲染到指定原生窗口）
            map_err(mpv.set_property("wid", wid), "设置 wid 失败")?;

            // ========== 通用核心参数 ==========
            mpv.set_property("keep-open", "yes").ok();
            mpv.set_property("force-window", "yes").ok();
            mpv.set_property("input-default-bindings", "yes").ok();
            mpv.set_property("input-vo-keyboard", "yes").ok();
            mpv.set_property("osc", "yes").ok();
            // 关闭 idle 防止窗口加载失败闪退
            mpv.set_property("idle", "yes").ok();
            // 控制台级日志送到 stderr（可选）
            mpv.set_property("msg-level", "all=warn").ok();

            // ========== 视频渲染：对齐 Hills Lite 发烧级 ==========
            // Windows 首选 gpu-next + d3d11；其他平台保留 gpu 兜底
            #[cfg(target_os = "windows")]
            {
                mpv.set_property("vo", "gpu-next").ok();
                mpv.set_property("gpu-api", "d3d11").ok();
                mpv.set_property("gpu-context", "d3d11").ok();
                // 零拷贝硬解：比 auto-safe 更激进，失败由 libmpv 自动回退
                mpv.set_property("hwdec", "d3d11va-copy").ok();
                // D3D11 显示交换链：减少延迟 & 更平滑
                mpv.set_property("d3d11-flip", "yes").ok();
                mpv.set_property("d3d11-sync-interval", "1").ok();
            }
            #[cfg(not(target_os = "windows"))]
            {
                mpv.set_property("vo", "gpu-next").ok();
                mpv.set_property("hwdec", "auto-safe").ok();
            }

            // ========== HDR 色彩管理（Hills Lite 默认方案） ==========
            // 告诉 vo 发送目标色彩空间信息给 OS（Win11 HDR 显示器开启）
            mpv.set_property("target-colorspace-hint", "yes").ok();
            // HDR→SDR tone mapping（BT.2446 Method A 为目前推荐）
            mpv.set_property("tone-mapping", "bt.2446a").ok();
            mpv.set_property("tone-mapping-mode", "rgb").ok();
            mpv.set_property("hdr-compute-peak", "yes").ok();
            // ICC 自动（用户系统色彩配置）
            mpv.set_property("icc-profile-auto", "yes").ok();

            // ========== 缓冲与流畅度 ==========
            mpv.set_property("cache", "yes").ok();
            mpv.set_property("demuxer-max-bytes", "400MiB").ok();
            mpv.set_property("demuxer-max-back-bytes", "100MiB").ok();
            // 适度提前预读，减少原盘大码率卡顿
            mpv.set_property("cache-secs", "30").ok();

            // ========== 字幕渲染（ASS / PGS 特效级） ==========
            mpv.set_property("sub-auto", "fuzzy").ok();
            mpv.set_property("sub-font-provider", "auto").ok();
            mpv.set_property("blend-subtitles", "yes").ok();
            // 字体目录：resources/fonts
            if let Some(fonts) = crate::resources::font_dir() {
                if fonts.exists() {
                    let s = fonts.to_string_lossy().to_string();
                    mpv.set_property("sub-fonts-dir", s.clone()).ok();
                    mpv.set_property("osd-fonts-dir", s).ok();
                }
            }

            // ========== 音频（Atmos / DTS-HD passthrough 友好） ==========
            mpv.set_property("audio-channels", "auto-safe").ok();
            // 默认直通 AC3/DTS/TrueHD/EAC3，若系统不支持 libmpv 自动回退解码
            mpv.set_property("audio-spdif", "ac3,dts,eac3,truehd,dts-hd").ok();

            // ========== 用户选项覆盖 ==========
            if let Some(start) = options.start_time {
                mpv.set_property("start", format!("+{}", start)).ok();
            }
            if let Some(alang) = &options.audio_lang {
                mpv.set_property("alang", alang.clone()).ok();
            }
            if let Some(slang) = &options.sub_lang {
                mpv.set_property("slang", slang.clone()).ok();
            }
            if let Some(ua) = &options.user_agent {
                mpv.set_property("user-agent", ua.clone()).ok();
            }
            if !options.http_headers.is_empty() {
                let joined = options
                    .http_headers
                    .iter()
                    .map(|(k, v)| format!("{}: {}", k, v))
                    .collect::<Vec<_>>()
                    .join("\r\n");
                mpv.set_property("http-header-fields", joined).ok();
            }

            Ok(Self { mpv })
        }

        pub fn loadfile(&self, url: &str) -> Result<()> {
            map_err(
                self.mpv.command("loadfile", &[url, "replace"]),
                "loadfile 失败",
            )
        }

        pub fn command(&self, cmd: &str, args: &[String]) -> Result<()> {
            let arg_refs: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
            map_err(self.mpv.command(cmd, &arg_refs), format!("mpv {} 命令失败", cmd))
        }

        pub fn set_property(&self, name: &str, value: &str) -> Result<()> {
            map_err(
                self.mpv.set_property(name, value.to_string()),
                format!("设置 mpv 属性 {} 失败", name),
            )
        }

        // ================= M4: Anime4K & 视频信息 =================

        /// 应用 Anime4K 档位预设（off/low/medium/high）
        pub fn set_anime4k(&self, level: &str) -> Result<()> {
            // 档位 → 官方推荐 shader 链
            // 参考 Anime4K v4.0.1 Quality Presets:
            //   Mode A:  Restore + Upscale (CNN)
            //   Mode A+: Mode A + AutoDownscale + Extra Upscale
            let list: &[&str] = match level {
                "off" | "" => &[],
                "low" => &[
                    "Anime4K_Clamp_Highlights.glsl",
                    "Anime4K_Restore_CNN_M.glsl",
                    "Anime4K_Upscale_CNN_x2_M.glsl",
                ],
                "medium" => &[
                    "Anime4K_Clamp_Highlights.glsl",
                    "Anime4K_Restore_CNN_VL.glsl",
                    "Anime4K_Upscale_CNN_x2_VL.glsl",
                ],
                "high" => &[
                    "Anime4K_Clamp_Highlights.glsl",
                    "Anime4K_Restore_CNN_UL.glsl",
                    "Anime4K_Upscale_CNN_x2_UL.glsl",
                    "Anime4K_AutoDownscalePre_x2.glsl",
                    "Anime4K_AutoDownscalePre_x4.glsl",
                    "Anime4K_Upscale_CNN_x2_M.glsl",
                ],
                other => return Err(anyhow!("未知 Anime4K 档位: {}", other)),
            };

            if list.is_empty() {
                map_err(
                    self.mpv.set_property("glsl-shaders", String::new()),
                    "清空 glsl-shaders 失败",
                )
            } else {
                let joined = crate::resources::join_shader_paths(list)
                    .ok_or_else(|| anyhow!("未找到 shader 目录（resources/shaders）"))?;
                map_err(
                    self.mpv.set_property("glsl-shaders", joined),
                    format!("应用 Anime4K {} 档位失败", level),
                )
            }
        }

        /// 获取当前视频信息（HDR、分辨率、编码、时长等）
        pub fn get_video_info(&self) -> VideoInfo {
            let get_str = |prop: &str| -> String {
                self.mpv
                    .get_property::<String>(prop)
                    .unwrap_or_default()
            };
            let get_i64 = |prop: &str| -> i64 {
                self.mpv.get_property::<i64>(prop).unwrap_or(0)
            };
            let get_f64 = |prop: &str| -> f64 {
                self.mpv.get_property::<f64>(prop).unwrap_or(0.0)
            };

            let pixelformat = get_str("video-params/pixelformat");
            let colormatrix = get_str("video-params/colormatrix");
            let primaries = get_str("video-params/primaries");
            let gamma = get_str("video-params/gamma");

            // 简易 HDR 判定：primaries=bt.2020 且 gamma 包含 pq/hlg
            let hdr_kind = if gamma.contains("pq") && primaries.contains("bt.2020") {
                "HDR10"
            } else if gamma.contains("hlg") {
                "HLG"
            } else if gamma.contains("dolbyvision") || colormatrix.contains("dovi") {
                "DoVi"
            } else {
                "SDR"
            };

            VideoInfo {
                width: get_i64("video-params/w") as u32,
                height: get_i64("video-params/h") as u32,
                codec: get_str("video-codec-name"),
                container: get_str("file-format"),
                duration: get_f64("duration"),
                position: get_f64("time-pos"),
                pixel_format: pixelformat,
                primaries,
                gamma,
                hdr: hdr_kind.to_string(),
                paused: self.mpv.get_property::<bool>("pause").unwrap_or(false),
                volume: get_f64("volume"),
                mute: self.mpv.get_property::<bool>("mute").unwrap_or(false),
            }
        }
    }

    /// 播放器实时信息快照
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
        /// SDR / HDR10 / HLG / DoVi
        pub hdr: String,
        pub paused: bool,
        pub volume: f64,
        pub mute: bool,
    }

    // libmpv2::Mpv 内部为 Arc，可在线程间共享；这里显式标记以便放入 HashMap
    unsafe impl Send for EmbeddedMpv {}
    unsafe impl Sync for EmbeddedMpv {}

    // 保持类型引用避免 unused 警告
    #[allow(dead_code)]
    fn _ensure_traits<T: SetData>() {}
}
