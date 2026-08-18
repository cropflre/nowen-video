use std::sync::{Arc, Mutex};

use crate::{mpv, settings, sidecar};

/// Desktop 2.0 全局运行时状态。
///
/// 这里只保存跨模块共享的长期对象；窗口、托盘、IPC 等短生命周期能力
/// 由各自模块管理，避免继续把所有职责堆进 main.rs。
pub struct AppState {
    /// Go Media Core sidecar 生命周期管理器。
    pub sidecar: Arc<Mutex<sidecar::SidecarManager>>,
    /// 桌面播放器运行时。
    ///
    /// 当前阶段仍承接现有 libmpv 实现，后续 Player Core 重构会把具体实现
    /// 收敛到 player/ 模块，AppState 的职责保持不变。
    pub mpv: Arc<Mutex<mpv::MpvManager>>,
    /// 桌面端持久化设置。
    pub settings: Arc<Mutex<settings::Settings>>,
}

impl AppState {
    pub fn new(settings: settings::Settings) -> Self {
        Self {
            sidecar: Arc::new(Mutex::new(sidecar::SidecarManager::new(settings.clone()))),
            mpv: Arc::new(Mutex::new(mpv::MpvManager::new(settings.clone()))),
            settings: Arc::new(Mutex::new(settings)),
        }
    }
}
