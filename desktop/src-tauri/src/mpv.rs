
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

// ================= libmpv 嵌入实现 =================

#[cfg(feature = "embed-mpv")]
mod embed {
    use super::PlayOptions;
    use anyhow::{Context, Result};
    use libmpv2::{Mpv, SetData};

    pub struct EmbeddedMpv {
        mpv: Mpv,
    }

    impl EmbeddedMpv {
        pub fn new(wid: i64, options: &PlayOptions) -> Result<Self> {
            let mpv = Mpv::new().context("创建 libmpv 实例失败")?;

            // 嵌入窗口（将 libmpv 渲染到指定原生窗口）
            mpv.set_property("wid", wid).context("设置 wid 失败")?;

            // 默认参数
            mpv.set_property("hwdec", "auto-safe").ok();
            mpv.set_property("keep-open", "yes").ok();
            mpv.set_property("force-window", "yes").ok();
            mpv.set_property("input-default-bindings", "yes").ok();
            mpv.set_property("input-vo-keyboard", "yes").ok();
            mpv.set_property("osc", "yes").ok();

            // 用户自定义
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
            self.mpv
                .command("loadfile", &[url, "replace"])
                .context("loadfile 失败")
        }

        pub fn command(&self, cmd: &str, args: &[String]) -> Result<()> {
            let arg_refs: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
            self.mpv
                .command(cmd, &arg_refs)
                .with_context(|| format!("mpv {} 命令失败", cmd))
        }

        pub fn set_property(&self, name: &str, value: &str) -> Result<()> {
            // libmpv2 的 set_property 对 &str 有实现
            self.mpv
                .set_property(name, value.to_string())
                .with_context(|| format!("设置 mpv 属性 {} 失败", name))
        }
    }

    // libmpv2::Mpv 内部为 Arc，可在线程间共享；这里显式标记以便放入 HashMap
    unsafe impl Send for EmbeddedMpv {}
    unsafe impl Sync for EmbeddedMpv {}

    // 保持类型引用避免 unused 警告
    #[allow(dead_code)]
    fn _ensure_traits<T: SetData>() {}
}
