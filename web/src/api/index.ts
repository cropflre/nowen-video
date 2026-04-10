/**
 * API 统一导出入口
 *
 * 所有 API 模块已按领域拆分为独立文件，此文件仅负责 re-export。
 * 这样做的好处：
 * 1. Tree-shaking 更有效 — 未使用的 API 不会打包
 * 2. 每个模块独立维护 — 减少合并冲突
 * 3. IDE 导航更快 — 代码补全更精准
 *
 * 使用方式不变：
 *   import { mediaApi, libraryApi } from '@/api'
 */

// 核心模块
export { authApi } from './auth'
export { libraryApi } from './library'
export { mediaApi, personApi } from './media'
export { streamApi } from './stream'
export { subtitleApi, subtitleSearchApi } from './subtitle'
export { subtitlePreprocessApi } from './subtitlePreprocess'
export { userApi } from './user'
export { playlistApi } from './playlist'
export { seriesApi } from './series'

// 管理模块
export { adminApi } from './admin'
export { scrapeApi, fileManagerApi } from './scrape'
export { notificationApi, batchMetadataApi, importExportApi } from './backup'
export { embyCompatApi } from './emby'

// AI 模块
export { aiApi, aiAssistantApi } from './ai'

// 社交与互动
export { recommendApi } from './recommend'
export { castApi, bookmarkApi, commentApi, statsApi } from './social'

// V3 扩展
export { aiSceneApi } from './v3'

// V2 扩展
export { userProfileApi, offlineDownloadApi, pluginApi, musicApi, photoApi, federationApi, abrApi } from './v2'

// V5: Pulse 数据中心
export { pulseApi } from './pulse'

// V6: P1~P3 新增功能
export { batchMoveApi } from './v4'
