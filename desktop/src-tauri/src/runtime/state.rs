use std::sync::{Arc, Mutex};

use crate::{player, settings, sidecar};

/// Desktop 2.0 全局运行时状态。
///
/// 这里只保存跨模块共享的长期对象；窗口、托盘、IPC 等短生命周期能力
/// 由各自模块管理，避免继续把所有职责堆进 main.rs。
pub struct AppState {
    /// Go Media Core sidecar 生命周期管理器。
    pub sidecar: Arc<Mutex<sidecar::SidecarManager>>,
    /// Desktop 2.0 唯一原生 Player Core。
    pub player: Arc<Mutex<player::PlayerManager>>,
    /// 桌面端持久化设置。
    pub settings: Arc<Mutex<settings::Settings>>,
}

impl AppState {
    pub fn new(settings: settings::Settings) -> Self {
        Self {
            sidecar: Arc::new(Mutex::new(sidecar::SidecarManager::new(settings.clone()))),
            player: Arc::new(Mutex::new(player::PlayerManager::new(settings.clone()))),
            settings: Arc::new(Mutex::new(settings)),
        }
    }
}
