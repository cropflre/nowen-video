//! Nowen Video Desktop 2.0 设置持久化。

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum ServerMode {
    Embedded,
    Remote,
}

impl Default for ServerMode {
    fn default() -> Self {
        Self::Embedded
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ServerSettings {
    #[serde(default)]
    pub mode: ServerMode,
    #[serde(default)]
    pub remote_url: String,
}

/// Desktop 2.0 不再暴露播放器引擎、外部 mpv 路径或 mpv CLI 参数。
/// 原生 Player Core 是桌面端唯一播放器，设置只保留用户真正可理解的产品能力。
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlayerSettings {
    #[serde(default = "default_true")]
    pub hardware_accel: bool,
}

impl Default for PlayerSettings {
    fn default() -> Self {
        Self {
            hardware_accel: true,
        }
    }
}

fn default_true() -> bool {
    true
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WindowSettings {
    #[serde(default = "default_window_width")]
    pub width: u32,
    #[serde(default = "default_window_height")]
    pub height: u32,
    #[serde(default = "default_true")]
    pub remember_size: bool,
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
    pub fn config_path() -> Result<PathBuf> {
        let mut path = dirs::config_dir().context("无法获取系统配置目录")?;
        path.push("nowen-video");
        fs::create_dir_all(&path).context("无法创建配置目录")?;
        path.push("settings.json");
        Ok(path)
    }

    pub fn load() -> Result<Self> {
        let path = Self::config_path()?;
        if !path.exists() {
            let settings = Self::default();
            settings.save()?;
            return Ok(settings);
        }

        let content = fs::read_to_string(&path).context("读取设置文件失败")?;
        serde_json::from_str(&content).context("解析设置文件失败")
    }

    pub fn save(&self) -> Result<()> {
        let path = Self::config_path()?;
        let content = serde_json::to_string_pretty(self).context("序列化设置失败")?;
        fs::write(path, content).context("写入设置文件失败")?;
        Ok(())
    }
}
