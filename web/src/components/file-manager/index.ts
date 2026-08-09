/**
 * 文件管理器子组件统一导出
 *
 * FileManagerPage 已拆分为以下子组件：
 * - FileStatsBar: 统计卡片栏
 * - FileToolbar: 搜索/筛选/批量操作工具栏
 * - FileListView: 文件列表（表格+网格视图+分页）
 * - ImportFileModal: 独立语义化导入弹窗
 * - ScanDirectoryModal: 独立语义化目录扫描弹窗
 * - EditFileModal: 独立语义化文件信息编辑弹窗
 * - FileModals: 其余历史弹窗（详情/重命名/日志），按阶段继续迁移
 * - constants: 共享常量、类型、工具函数
 */

export { default as FileStatsBar } from './FileStatsBar'
export { default as FileToolbar } from './FileToolbar'
export { default as FileListView } from './FileListView'
export { default as FolderTree } from './FolderTree'
export { default as Breadcrumb } from './Breadcrumb'
export { default as ContextMenu } from './ContextMenu'
export type { ContextMenuItem } from './ContextMenu'
export { default as ImportFileModal } from './ImportFileModal'
export { default as ScanDirectoryModal } from './ScanDirectoryModal'
export { default as EditFileModal } from './EditFileModal'
export {
  FileDetailModal,
  RenameModal,
  OperationLogsModal,
} from './FileModals'
export * from './constants'
