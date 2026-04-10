import { useState, useCallback, useRef, useEffect } from 'react'
import type { FolderNode } from '@/types'
import {
  ChevronRight,
  Folder,
  FolderOpen,
  FolderPlus,
  Loader2,
  FolderTree as FolderTreeIcon,
  Home,
  Pencil,
  Trash2,
  RefreshCw,
  Copy,
} from 'lucide-react'
import clsx from 'clsx'
import ContextMenu from './ContextMenu'
import type { ContextMenuItem } from './ContextMenu'

interface FolderTreeProps {
  tree: FolderNode[]
  loading: boolean
  selectedPath: string
  onSelectFolder: (path: string) => void
  onClearFolder: () => void
  onCreateFolder?: (parentPath: string) => void
  onRenameFolder?: (folderPath: string) => void
  onDeleteFolder?: (folderPath: string) => void
  onRefreshFolder?: () => void
  onCopyPath?: (path: string) => void
}

// 单个树节点
function TreeNode({
  node,
  selectedPath,
  onSelect,
  onContextMenu,
  depth = 0,
  scrollIntoView,
}: {
  node: FolderNode
  selectedPath: string
  onSelect: (path: string) => void
  onContextMenu: (e: React.MouseEvent, node: FolderNode) => void
  depth?: number
  scrollIntoView?: boolean
}) {
  const [expanded, setExpanded] = useState(depth < 1) // 默认展开第一层
  const isSelected = selectedPath === node.path
  const hasChildren = node.children && node.children.length > 0
  const nodeRef = useRef<HTMLDivElement>(null)

  // 选中节点自动滚动到可视区域
  useEffect(() => {
    if (isSelected && scrollIntoView && nodeRef.current) {
      nodeRef.current.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  }, [isSelected, scrollIntoView])

  // 当选中路径是当前节点的子路径时，自动展开
  useEffect(() => {
    if (selectedPath && hasChildren && selectedPath.startsWith(node.path + '/')) {
      setExpanded(true)
    }
  }, [selectedPath, node.path, hasChildren])

  const handleToggle = useCallback((e: React.MouseEvent) => {
    e.stopPropagation()
    setExpanded(!expanded)
  }, [expanded])

  const handleSelect = useCallback(() => {
    onSelect(node.path)
    if (hasChildren && !expanded) {
      setExpanded(true)
    }
  }, [node.path, onSelect, hasChildren, expanded])

  const handleContextMenu = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    onSelect(node.path)
    onContextMenu(e, node)
  }, [node, onSelect, onContextMenu])

  return (
    <div>
      <div
        ref={nodeRef}
        onClick={handleSelect}
        onContextMenu={handleContextMenu}
        className={clsx(
          'flex items-center gap-1.5 px-2 py-1.5 rounded-lg cursor-pointer text-sm transition-all group',
          isSelected
            ? 'bg-neon-blue/10 text-neon'
            : 'hover:bg-white/[0.04] text-surface-300 hover:text-surface-100'
        )}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        title={node.path}
      >
        {/* 展开/收起箭头 */}
        {hasChildren ? (
          <button
            onClick={handleToggle}
            className="flex-shrink-0 p-0.5 rounded hover:bg-white/10 transition-transform"
          >
            <ChevronRight
              size={14}
              className={clsx(
                'text-surface-400 transition-transform duration-200',
                expanded && 'rotate-90'
              )}
            />
          </button>
        ) : (
          <span className="w-[18px] flex-shrink-0" />
        )}

        {/* 文件夹图标 */}
        {isSelected || expanded ? (
          <FolderOpen size={16} className={clsx('flex-shrink-0', isSelected ? 'text-neon' : 'text-amber-400/70')} />
        ) : (
          <Folder size={16} className="flex-shrink-0 text-amber-400/50 group-hover:text-amber-400/70" />
        )}

        {/* 文件夹名称 */}
        <span className="truncate flex-1 min-w-0">{node.name}</span>

        {/* 文件数量 */}
        {node.file_count > 0 && (
          <span className={clsx(
            'flex-shrink-0 text-[10px] px-1.5 py-0.5 rounded-full',
            isSelected ? 'bg-neon-blue/20 text-neon' : 'bg-surface-700/50 text-surface-400'
          )}>
            {node.file_count}
          </span>
        )}
      </div>

      {/* 子节点 - 带展开/收起动画 */}
      {expanded && hasChildren && (
        <div className="animate-slide-down">
          {node.children.map((child) => (
            <TreeNode
              key={child.path}
              node={child}
              selectedPath={selectedPath}
              onSelect={onSelect}
              onContextMenu={onContextMenu}
              depth={depth + 1}
              scrollIntoView={scrollIntoView}
            />
          ))}
        </div>
      )}
    </div>
  )
}

export default function FolderTree({
  tree,
  loading,
  selectedPath,
  onSelectFolder,
  onClearFolder,
  onCreateFolder,
  onRenameFolder,
  onDeleteFolder,
  onRefreshFolder,
  onCopyPath,
}: FolderTreeProps) {
  // 右键菜单状态
  const [contextMenu, setContextMenu] = useState<{
    visible: boolean; x: number; y: number; node: FolderNode | null
  }>({ visible: false, x: 0, y: 0, node: null })

  const handleContextMenu = useCallback((e: React.MouseEvent, node: FolderNode) => {
    setContextMenu({ visible: true, x: e.clientX, y: e.clientY, node })
  }, [])

  const closeContextMenu = useCallback(() => {
    setContextMenu(prev => ({ ...prev, visible: false }))
  }, [])

  // 构建右键菜单项
  const getContextMenuItems = useCallback((): ContextMenuItem[] => {
    const node = contextMenu.node
    if (!node) return []
    return [
      {
        key: 'create',
        label: '新建子文件夹',
        icon: <FolderPlus size={14} />,
        disabled: !onCreateFolder,
        onClick: () => onCreateFolder?.(node.path),
      },
      {
        key: 'rename',
        label: '重命名',
        icon: <Pencil size={14} />,
        disabled: !onRenameFolder,
        onClick: () => onRenameFolder?.(node.path),
      },
      {
        key: 'refresh',
        label: '刷新',
        icon: <RefreshCw size={14} />,
        divider: true,
        disabled: !onRefreshFolder,
        onClick: () => onRefreshFolder?.(),
      },
      {
        key: 'copy-path',
        label: '复制路径',
        icon: <Copy size={14} />,
        onClick: () => onCopyPath?.(node.path),
      },
      {
        key: 'delete',
        label: '删除文件夹',
        icon: <Trash2 size={14} />,
        danger: true,
        divider: true,
        disabled: !onDeleteFolder,
        onClick: () => onDeleteFolder?.(node.path),
      },
    ]
  }, [contextMenu.node, onCreateFolder, onRenameFolder, onDeleteFolder, onRefreshFolder, onCopyPath])

  // 滚动容器引用，用于自动滚动到选中节点
  const scrollContainerRef = useRef<HTMLDivElement>(null)

  return (
    <div className="glass-panel rounded-xl flex flex-col h-full overflow-hidden">
      {/* 标题栏 */}
      <div className="flex-shrink-0 flex items-center gap-2 px-3 py-2.5 border-b" style={{ borderColor: 'var(--border-default)' }}>
        <FolderTreeIcon size={16} className="text-neon flex-shrink-0" />
        <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>文件夹导航</span>
      </div>

      {/* 全部文件按钮 */}
      <div className="flex-shrink-0 px-2 pt-2">
        <button
          onClick={onClearFolder}
          className={clsx(
            'flex items-center gap-2 w-full px-2 py-1.5 rounded-lg text-sm transition-all',
            !selectedPath
              ? 'bg-neon-blue/10 text-neon'
              : 'hover:bg-white/[0.04] text-surface-300 hover:text-surface-100'
          )}
        >
          <Home size={16} />
          <span>全部文件</span>
        </button>
      </div>

      {/* 树形列表 - 可滚动区域 */}
      <div
        ref={scrollContainerRef}
        className="flex-1 overflow-y-auto px-2 py-1 min-h-0"
        style={{
          scrollBehavior: 'smooth',
          scrollbarWidth: 'thin',
          scrollbarColor: 'rgba(255,255,255,0.15) transparent',
        }}
      >
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 size={20} className="animate-spin text-surface-400" />
          </div>
        ) : tree.length === 0 ? (
          <div className="text-center py-8">
            <Folder size={32} className="mx-auto mb-2 text-surface-600" />
            <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>暂无文件夹</p>
          </div>
        ) : (
          tree.map((node) => (
            <TreeNode
              key={node.path}
              node={node}
              selectedPath={selectedPath}
              onSelect={onSelectFolder}
              onContextMenu={handleContextMenu}
              scrollIntoView
            />
          ))
        )}
      </div>

      {/* 右键菜单 */}
      <ContextMenu
        visible={contextMenu.visible}
        x={contextMenu.x}
        y={contextMenu.y}
        items={getContextMenuItems()}
        onClose={closeContextMenu}
      />
    </div>
  )
}
