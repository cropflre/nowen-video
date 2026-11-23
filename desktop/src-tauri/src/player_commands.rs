//! Player Core 按需媒体导航信息。
//!
//! 高频播放状态通过 `player-state` 事件推送；轨道和章节列表只在需要展示或刷新时读取，
//! 避免把大型列表塞进每一帧状态事件。

use crate::player::PlayerMediaInfo;
use crate::AppState;
use tauri::{AppHandle, Manager};

#[tauri::command]
pub async fn player_media_info(
    app: AppHandle,
    session_id: String,
) -> Result<PlayerMediaInfo, String> {
    tokio::task::spawn_blocking(move || {
        let state = app.state::<AppState>();
        let player = state.player.lock().map_err(|error| error.to_string())?;
        player
            .media_info(&session_id)
            .map_err(|error| error.to_string())
    })
    .await
    .map_err(|error| error.to_string())?
}
