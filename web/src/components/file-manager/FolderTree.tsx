import { useCallback, useEffect, useRef, useState } from 'react'
import type { FolderNode } from '@/types'
import {
  ChevronRight,
  Copy,
  Folder,
  FolderOpen,
  FolderPlus,
  FolderTree as FolderTreeIcon,
  Home,
  Loader2,
  Pencil,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import clsx from 'clsx'
import { Surface } from '@/components/design-system'
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

function normalizeTreePath(path: string) {
  let normalized = path.replace(/\\/g, '/')
  while (normalized.length > 1 && normalized.endsWith('/') && !/^[A-Za-z]:\/$/.test(normalized)) {
    normalized = normalized.slice(0, -1)
  }
  return normalized
}

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
  onContextMenu: (event: React.MouseEvent, node: FolderNode) => void
  depth?: number
  scrollIntoView?: boolean
}) {
  const [expanded, setExpanded] = useState(depth < 1)
  const normalizedNodePath = normalizeTreePath(node.path)
  const normalizedSelectedPath = normalizeTreePath(selectedPath)
  const isSelected = normalizedSelectedPath === normalizedNodePath
  const hasChildren = Boolean(node.children?.length)
  const nodeRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (isSelected && scrollIntoView && nodeRef.current) {
      nodeRef.current.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  }, [isSelected, scrollIntoView])

  useEffect(() => {
    if (
      normalizedSelectedPath
      && hasChildren
      && normalizedSelectedPath.startsWith(`${normalizedNodePath}/`)
    ) {
      setExpanded(true)
    }
  }, [normalizedSelectedPath, normalizedNodePath, hasChildren])

  const handleToggle = useCallback((event: React.MouseEvent) => {
    event.stopPropagation()
    setExpanded((value) => !value)
  }, [])

  const handleSelect = useCallback(() => {
    onSelect(node.path)
    if (hasChildren && !expanded) setExpanded(true)
  }, [node.path, onSelect, hasChildren, expanded])

  const handleContextMenu = useCallback((event: React.MouseEvent) => {
    event.preventDefault()
    event.stopPropagation()
    onSelect(node.path)
    onContextMenu(event, node)
  }, [node, onSelect, onContextMenu])

  const handleKeyDown = useCallback((event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      handleSelect()
    }
    if (event.key === 'ArrowRight' && hasChildren) {
      event.preventDefault()
      setExpanded(true)
    }
    if (event.key === 'ArrowLeft' && hasChildren) {
      event.preventDefault()
      setExpanded(false)
    }
  }, [handleSelect, hasChildren])

  return (
    <div role="none">
      <div
        ref={nodeRef}
        role="treeitem"
        tabIndex={0}
        aria-selected={isSelected}
        aria-expanded={hasChildren ? expanded : undefined}
        onClick={handleSelect}
        onKeyDown={handleKeyDown}
        onContextMenu={handleContextMenu}
        className={clsx(
          'group flex cursor-pointer items-center gap-1.5 rounded-[var(--nv-radius-control)] py-1.5 pr-2 text-sm outline-none transition-[background-color,color,box-shadow] duration-200 focus-visible:shadow-[var(--nv-shadow-focus)]',
          isSelected
            ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
            : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
        )}
        style={{ paddingLeft: `${depth * 16 + 8}px` }}
        title={node.path}
      >
        {hasChildren ? (
          <button
            type="button"
            onClick={handleToggle}
            className="flex h-[18px] w-[18px] shrink-0 items-center justify-center rounded-[var(--nv-radius-sm)] text-[var(--nv-text-tertiary)] outline-none transition-[background-color,color] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)]"
            aria-label={expanded ? `收起 ${node.name}` : `展开 ${node.name}`}
            aria-expanded={expanded}
          >
            <ChevronRight
              size={14}
              className={clsx('transition-transform duration-200 motion-reduce:transition-none', expanded && 'rotate-90')}
              aria-hidden="true"
            />
          </button>
        ) : (
          <span className="h-[18px] w-[18px] shrink-0" aria-hidden="true" />
        )}

        {isSelected || expanded ? (
          <FolderOpen
            size={16}
            className={clsx('shrink-0', isSelected ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-tertiary)]')}
            aria-hidden="true"
          />
        ) : (
          <Folder
            size={16}
            className="shrink-0 text-[var(--nv-text-tertiary)] transition-colors group-hover:text-[var(--nv-text-secondary)]"
            aria-hidden="true"
          />
        )}

        <span className="min-w-0 flex-1 truncate">{node.name}</span>

        {node.file_count > 0 && (
          <span
            className={clsx(
              'shrink-0 rounded-[var(--nv-radius-pill)] border px-1.5 py-0.5 text-[10px] leading-none',
              isSelected
                ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-control)] text-[var(--nv-action-primary)]'
                : 'border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-tertiary)]',
            )}
          >
            {node.file_count}
          </span>
        )}
      </div>

      {expanded && hasChildren && (
        <div role="group" className="animate-slide-down motion-reduce:animate-none">
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
  const [contextMenu, setContextMenu] = useState<{
    visible: boolean
    x: number
    y: number
    node: FolderNode | null
  }>({ visible: false, x: 0, y: 0, node: null })

  const handleContextMenu = useCallback((event: React.MouseEvent, node: FolderNode) => {
    setContextMenu({ visible: true, x: event.clientX, y: event.clientY, node })
  }, [])

  const closeContextMenu = useCallback(() => {
    setContextMenu((value) => ({ ...value, visible: false }))
  }, [])

  const getContextMenuItems = useCallback((): ContextMenuItem[] => {
    const node = contextMenu.node
    if (!node) return []

    return [
      {
        key: 'create',
        label: '新建子文件夹',
        icon: <FolderPlus size={14} aria-hidden="true" />,
        disabled: !onCreateFolder,
        onClick: () => onCreateFolder?.(node.path),
      },
      {
        key: 'rename',
        label: '重命名',
        icon: <Pencil size={14} aria-hidden="true" />,
        disabled: !onRenameFolder,
        onClick: () => onRenameFolder?.(node.path),
      },
      {
        key: 'refresh',
        label: '刷新',
        icon: <RefreshCw size={14} aria-hidden="true" />,
        divider: true,
        disabled: !onRefreshFolder,
        onClick: () => onRefreshFolder?.(),
      },
      {
        key: 'copy-path',
        label: '复制路径',
        icon: <Copy size={14} aria-hidden="true" />,
        disabled: !onCopyPath,
        onClick: () => onCopyPath?.(node.path),
      },
      {
        key: 'delete',
        label: '删除文件夹',
        icon: <Trash2 size={14} aria-hidden="true" />,
        danger: true,
        divider: true,
        disabled: !onDeleteFolder,
        onClick: () => onDeleteFolder?.(node.path),
      },
    ]
  }, [contextMenu.node, onCreateFolder, onRenameFolder, onDeleteFolder, onRefreshFolder, onCopyPath])

  return (
    <Surface className="flex h-full min-h-0 flex-col overflow-hidden">
      <div className="flex shrink-0 items-center gap-2 border-b border-[var(--nv-border-subtle)] px-3 py-2.5">
        <FolderTreeIcon size={16} className="shrink-0 text-[var(--nv-action-primary)]" aria-hidden="true" />
        <span className="text-sm font-semibold text-[var(--nv-text-primary)]">文件夹导航</span>
      </div>

      <div className="shrink-0 px-2 pt-2">
        <button
          type="button"
          onClick={onClearFolder}
          aria-current={!selectedPath ? 'page' : undefined}
          className={clsx(
            'flex w-full items-center gap-2 rounded-[var(--nv-radius-control)] px-2 py-1.5 text-sm outline-none transition-[background-color,color,box-shadow] duration-200 focus-visible:shadow-[var(--nv-shadow-focus)]',
            !selectedPath
              ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
              : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
          )}
        >
          <Home size={16} aria-hidden="true" />
          <span>全部文件</span>
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-2 py-1" role="tree" aria-label="文件夹目录">
        {loading ? (
          <div className="flex items-center justify-center py-8 text-[var(--nv-text-tertiary)]">
            <Loader2 size={20} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
            <span className="sr-only">正在加载文件夹</span>
          </div>
        ) : tree.length === 0 ? (
          <div className="px-4 py-8 text-center text-[var(--nv-text-tertiary)]">
            <div className="mx-auto mb-2 flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
              <Folder size={20} aria-hidden="true" />
            </div>
            <p className="text-xs">暂无文件夹</p>
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

      <ContextMenu
        visible={contextMenu.visible}
        x={contextMenu.x}
        y={contextMenu.y}
        items={getContextMenuItems()}
        onClose={closeContextMenu}
      />
    </Surface>
  )
}
