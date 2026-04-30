
//! 应用设置持久化

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

/// 后端模式
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum ServerMode {
    /// 内嵌 Go sidecar
    Embedded,
    /// 连接远程 server
    Remote,
}

impl Default for ServerMode {
    fn default() -> Self {
        Self::Embedded
    }
}

/// 播放器引擎
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum PlayerEngine {
    /// 自动（根据文件特征决策）
    Auto,
    /// 总是使用 mpv
    Mpv,
    /// 总是使用 Web `<video>`
    Web,
}

impl Default for PlayerEngine {
    fn default() -> Self {
        Self::Auto
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ServerSettings {
    #[serde(default)]
    pub mode: ServerMode,
    #[serde(default)]
    pub remote_url: String,
    #[serde(default = "default_sidecar_port")]
    pub sidecar_port: u16,
}

fn default_sidecar_port() -> u16 {
    8080
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlayerSettings {
    #[serde(default)]
    pub engine: PlayerEngine,
    /// mpv 可执行路径（空表示自动查找）
    #[serde(default)]
    pub mpv_path: String,
    #[serde(default = "default_mpv_args")]
    pub mpv_args: Vec<String>,
    #[serde(default = "default_true")]
    pub hardware_accel: bool,
}

fn default_mpv_args() -> Vec<String> {
    // 与嵌入模式保持一致的 Hills Lite 级默认参数
    #[cfg(target_os = "windows")]
    {
        vec![
            "--vo=gpu-next".to_string(),
            "--gpu-api=d3d11".to_string(),
            "--hwdec=d3d11va-copy".to_string(),
            "--keep-open=yes".to_string(),
            "--force-window=yes".to_string(),
            "--target-colorspace-hint=yes".to_string(),
            "--tone-mapping=bt.2446a".to_string(),
            "--tone-mapping-mode=rgb".to_string(),
            "--hdr-compute-peak=yes".to_string(),
            "--icc-profile-auto=yes".to_string(),
            "--cache=yes".to_string(),
            "--demuxer-max-bytes=400MiB".to_string(),
            "--demuxer-max-back-bytes=100MiB".to_string(),
            "--cache-secs=30".to_string(),
            "--blend-subtitles=yes".to_string(),
            "--sub-auto=fuzzy".to_string(),
        ]
    }
    #[cfg(not(target_os = "windows"))]
    {
        vec![
            "--vo=gpu-next".to_string(),
            "--hwdec=auto-safe".to_string(),
            "--keep-open=yes".to_string(),
            "--force-window=yes".to_string(),
            "--target-colorspace-hint=yes".to_string(),
            "--tone-mapping=bt.2446a".to_string(),
            "--hdr-compute-peak=yes".to_string(),
            "--icc-profile-auto=yes".to_string(),
            "--cache=yes".to_string(),
            "--demuxer-max-bytes=400MiB".to_string(),
            "--cache-secs=30".to_string(),
            "--blend-subtitles=yes".to_string(),
            "--sub-auto=fuzzy".to_string(),
        ]
    }
}

fn default_true() -> bool {
    true
}

impl Default for PlayerSettings {
    fn default() -> Self {
        Self {
            engine: PlayerEngine::default(),
            mpv_path: String::new(),
            mpv_args: default_mpv_args(),
            hardware_accel: true,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WindowSettings {
    #[serde(default = "default_window_width")]
    pub width: u32,
    #[serde(default = "default_window_height")]
    pub height: u32,
    #[serde(default = "default_true")]
    pub remember_size: bool,
    /// 关闭主窗口时最小化到托盘而非退出
    #[serde(default)]
    pub minimize_to_tray: bool,
}

fn default_window_width() -> u32 {
    1400
}

fn default_window_height() -> u32 {
    900
}

impl Default for WindowSettings {
    fn default() -> Self {
        Self {
            width: default_window_width(),
            height: default_window_height(),
            remember_size: true,
            minimize_to_tray: false,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct Settings {
    #[serde(default)]
    pub server: ServerSettings,
    #[serde(default)]
    pub player: PlayerSettings,
    #[serde(default)]
    pub window: WindowSettings,
}

impl Settings {
    /// 配置文件路径
    pub fn config_path() -> Result<PathBuf> {
        let mut p = dirs::config_dir().context("无法获取系统配置目录")?;
        p.push("nowen-video");
        fs::create_dir_all(&p).context("无法创建配置目录")?;
        p.push("settings.json");
        Ok(p)
    }

    /// 加载配置
    pub fn load() -> Result<Self> {
        let path = Self::config_path()?;
        if !path.exists() {
            let default = Self::default();
            default.save()?;
            return Ok(default);
        }
        let content = fs::read_to_string(&path).context("读取设置文件失败")?;
        let s: Self = serde_json::from_str(&content).context("解析设置文件失败")?;
        Ok(s)
    }

    /// 保存配置
    pub fn save(&self) -> Result<()> {
        let path = Self::config_path()?;
        let content = serde_json::to_string_pretty(self).context("序列化设置失败")?;
        fs::write(&path, content).context("写入设置文件失败")?;
        Ok(())
    }
}
