//! Player Core 按需媒体导航信息。
//!
//! 高频播放状态通过 `player-state` 事件推送；轨道和章节列表只在需要展示或刷新时读取，
//! 避免把大型列表塞进每一帧状态事件。

use crate::player::PlayerMediaInfo;
use crate::AppState;
use tauri::State;

#[tauri::command]
pub fn player_media_info(
    state: State<AppState>,
    session_id: String,
) -> Result<PlayerMediaInfo, String> {
    let player = state.player.lock().map_err(|error| error.to_string())?;
    player
        .media_info(&session_id)
        .map_err(|error| error.to_string())
}
