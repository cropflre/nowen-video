import { useState, useRef, useEffect, useCallback } from 'react'
import {
  MessageSquare,
  Send,
  X,
  Loader2,
  Sparkles,
  Bot,
  User,
  ChevronDown,
  ChevronUp,
  Play,
  Undo2,
  AlertTriangle,
  History,
  Trash2,
  Minimize2,
  Maximize2,
  ArrowRight,
} from 'lucide-react'
import clsx from 'clsx'
import { aiAssistantApi } from '@/api'
import type { ChatMsg, SuggestedAction, OperationPreview, AssistantOperation } from '@/types'
import { useToast } from '@/components/Toast'
import { Button, Surface, Tag, Textarea } from '@/components/design-system'

interface AIAssistantProps {
  /** 当前选中的文件ID列表 */
  selectedMediaIds: string[]
  /** 当前媒体库ID */
  libraryId?: string
  /** 操作执行后的回调（用于刷新列表） */
  onOperationComplete?: () => void
  /** 是否展开面板（由父组件控制） */
  isOpen: boolean
  /** 切换面板展开/关闭 */
  onToggle: () => void
}

/** AI助手触发按钮（嵌入到工具栏中） */
export function AIAssistantButton({ isOpen, onToggle, selectedCount }: {
  isOpen: boolean
  onToggle: () => void
  selectedCount: number
}) {
  return (
    <Button
      type="button"
      variant={isOpen ? 'primary' : 'secondary'}
      size="sm"
      onClick={onToggle}
      aria-pressed={isOpen}
      title={isOpen ? '关闭AI助手' : '打开AI助手'}
    >
      <Bot size={16} aria-hidden={true} />
      <span>AI 助手</span>
      {selectedCount > 0 && (
        <Tag tone={isOpen ? 'neutral' : 'brand'} className="ml-0.5">
          {selectedCount}
        </Tag>
      )}
    </Button>
  )
}

export default function AIAssistant({ selectedMediaIds, libraryId, onOperationComplete, isOpen, onToggle }: AIAssistantProps) {
  const toast = useToast()
  const [isMinimized, setIsMinimized] = useState(false)
  const [sessionId, setSessionId] = useState<string>('')
  const [messages, setMessages] = useState<ChatMsg[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [executing, setExecuting] = useState<string | null>(null)
  const [showHistory, setShowHistory] = useState(false)
  const [opHistory, setOpHistory] = useState<AssistantOperation[]>([])
  const [expandedPreviews, setExpandedPreviews] = useState<Set<number>>(new Set())
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, scrollToBottom])

  useEffect(() => {
    if (isOpen && !isMinimized) {
      setTimeout(() => inputRef.current?.focus(), 100)
    }
  }, [isOpen, isMinimized])

  const handleSend = async () => {
    if (!input.trim() || loading) return

    const userMessage = input.trim()
    setInput('')

    const userMsg: ChatMsg = {
      role: 'user',
      content: userMessage,
      timestamp: new Date().toISOString(),
    }
    setMessages(prev => [...prev, userMsg])
    setLoading(true)

    try {
      const res = await aiAssistantApi.chat({
        session_id: sessionId || undefined,
        message: userMessage,
        media_ids: selectedMediaIds.length > 0 ? selectedMediaIds : undefined,
        library_id: libraryId,
      })

      const data = res.data.data
      setSessionId(data.session_id)
      setMessages(prev => [...prev, data.message])
    } catch (err: any) {
      const errorMsg: ChatMsg = {
        role: 'assistant',
        content: `❌ ${err?.response?.data?.error || '请求失败，请稍后重试'}`,
        timestamp: new Date().toISOString(),
      }
      setMessages(prev => [...prev, errorMsg])
    } finally {
      setLoading(false)
    }
  }

  const handleExecuteAction = async (action: SuggestedAction) => {
    if (!sessionId || executing) return

    setExecuting(action.id)
    try {
      const res = await aiAssistantApi.executeAction({
        session_id: sessionId,
        action_id: action.id,
      })

      const data = res.data.data
      const resultMsg: ChatMsg = {
        role: 'assistant',
        content: data.success
          ? `✅ ${data.message}${data.errors?.length ? `\n\n⚠️ 部分错误:\n${data.errors.join('\n')}` : ''}`
          : `❌ ${data.message}`,
        timestamp: new Date().toISOString(),
        previews: data.results,
      }
      setMessages(prev => [...prev, resultMsg])

      if (data.success) {
        toast.success(data.message)
        onOperationComplete?.()
      }
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '操作执行失败')
    } finally {
      setExecuting(null)
    }
  }

  const handleUndo = async (opId: string) => {
    try {
      const res = await aiAssistantApi.undoOperation(opId)
      const data = res.data.data
      toast.success(data.message)
      onOperationComplete?.()
      loadHistory()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '撤销失败')
    }
  }

  const loadHistory = async () => {
    try {
      const res = await aiAssistantApi.getOperationHistory(20)
      setOpHistory(res.data.data || [])
    } catch {
      // 忽略
    }
  }

  const handleNewSession = () => {
    setSessionId('')
    setMessages([])
    setExpandedPreviews(new Set())
  }

  const togglePreview = (index: number) => {
    setExpandedPreviews(prev => {
      const next = new Set(prev)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  const quickCommands = [
    { label: '📊 分析文件库', cmd: '分析当前文件库的整体状态，给出优化建议' },
    { label: '🔍 批量刮削', cmd: '为选中的文件批量获取元数据' },
    { label: '🏷️ 自动分类', cmd: '分析选中文件的内容，自动添加合适的标签分类' },
    { label: '🔧 检测问题', cmd: '检查选中文件是否有命名不规范、信息缺失等问题' },
    { label: '🎬 误分类检测', cmd: '分析文件库中被误标记为电影的剧集文件，提供重分类建议' },
  ]

  const renderContent = (content: string) => {
    const lines = content.split('\n')
    return lines.map((line, i) => {
      if (line.startsWith('## ')) {
        return <h3 key={i} className="mb-1 mt-2 text-sm font-bold text-[var(--nv-text-primary)]">{line.slice(3)}</h3>
      }
      if (line.startsWith('### ')) {
        return <h4 key={i} className="mb-1 mt-2 text-xs font-bold text-[var(--nv-text-primary)]">{line.slice(4)}</h4>
      }
      if (line.startsWith('- ')) {
        return <div key={i} className="py-0.5 pl-2 text-xs text-[var(--nv-text-secondary)]">{line}</div>
      }
      if (line.startsWith('| ')) {
        return <div key={i} className="py-0.5 font-mono text-xs text-[var(--nv-text-secondary)]">{line}</div>
      }
      if (line.startsWith('**') && line.endsWith('**')) {
        return <div key={i} className="text-xs font-bold text-[var(--nv-text-primary)]">{line.slice(2, -2)}</div>
      }
      if (line.trim() === '') return <div key={i} className="h-1" />
      return <div key={i} className="text-xs text-[var(--nv-text-secondary)]">{line}</div>
    })
  }

  const renderPreviews = (previews: OperationPreview[], msgIndex: number) => {
    if (!previews || previews.length === 0) return null
    const isExpanded = expandedPreviews.has(msgIndex)

    return (
      <div className="mt-2">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => togglePreview(msgIndex)}
          className="h-auto px-1.5 py-1 text-xs text-[var(--nv-text-tertiary)]"
        >
          {isExpanded ? <ChevronUp size={12} aria-hidden={true} /> : <ChevronDown size={12} aria-hidden={true} />}
          预览变更 ({previews.length} 项)
        </Button>
        {isExpanded && (
          <div className="mt-1.5 max-h-48 space-y-1 overflow-y-auto [scrollbar-width:thin]">
            {previews.map((preview, index) => (
              <div
                key={index}
                className="flex items-start gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-2 text-xs"
              >
                <span className="w-4 shrink-0 text-right font-mono text-[var(--nv-text-tertiary)]">{index + 1}</span>
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[var(--nv-status-danger)] line-through opacity-80">{preview.old_value}</div>
                  <div className="flex items-center gap-1 text-[var(--nv-status-success)]">
                    <ArrowRight size={10} className="shrink-0" aria-hidden={true} />
                    <span className="truncate">{preview.new_value}</span>
                  </div>
                </div>
                <Tag tone="neutral" className="shrink-0">{preview.change_type}</Tag>
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }

  const renderActions = (actions: SuggestedAction[]) => {
    if (!actions || actions.length === 0) return null

    return (
      <div className="mt-2 flex flex-wrap gap-1.5">
        {actions.map(action => (
          <Button
            key={action.id}
            type="button"
            variant={action.dangerous ? 'danger' : 'secondary'}
            size="sm"
            onClick={() => handleExecuteAction(action)}
            disabled={!!executing}
            loading={executing === action.id}
            title={action.description}
            className="h-8 text-xs"
          >
            {executing !== action.id && (
              action.dangerous
                ? <AlertTriangle size={12} aria-hidden={true} />
                : <Play size={12} className="text-[var(--nv-action-primary)]" aria-hidden={true} />
            )}
            {action.label}
          </Button>
        ))}
      </div>
    )
  }

  if (!isOpen) return null

  return (
    <div className={clsx('flex h-full flex-col', isMinimized ? 'w-72' : 'w-full')}>
      <Surface className="flex h-full flex-col overflow-hidden border border-[var(--nv-border-default)] shadow-[var(--nv-shadow-card)]">
        <div className="flex items-center justify-between border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
          <div className="flex min-w-0 items-center gap-2.5">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]">
              <Bot size={16} aria-hidden={true} />
            </div>
            <div className="min-w-0">
              <h3 className="truncate text-sm font-bold text-[var(--nv-text-primary)]">AI 文件助手</h3>
              <p className="truncate text-[10px] text-[var(--nv-text-tertiary)]">
                {selectedMediaIds.length > 0 ? `已选 ${selectedMediaIds.length} 个文件` : '自然语言管理文件'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              iconOnly
              onClick={() => {
                setShowHistory(!showHistory)
                if (!showHistory) loadHistory()
              }}
              aria-label="操作历史"
              title="操作历史"
            >
              <History size={14} aria-hidden={true} />
            </Button>
            <Button type="button" variant="ghost" size="sm" iconOnly onClick={handleNewSession} aria-label="新建会话" title="新建会话">
              <Trash2 size={14} aria-hidden={true} />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              iconOnly
              onClick={() => setIsMinimized(!isMinimized)}
              aria-label={isMinimized ? '展开助手' : '最小化助手'}
              title={isMinimized ? '展开助手' : '最小化助手'}
            >
              {isMinimized ? <Maximize2 size={14} aria-hidden={true} /> : <Minimize2 size={14} aria-hidden={true} />}
            </Button>
            <Button type="button" variant="ghost" size="sm" iconOnly onClick={onToggle} aria-label="关闭AI助手" title="关闭AI助手">
              <X size={14} aria-hidden={true} />
            </Button>
          </div>
        </div>

        {!isMinimized && (
          <>
            {showHistory && (
              <div className="max-h-48 overflow-y-auto border-b border-[var(--nv-border-subtle)] px-4 py-3 [scrollbar-width:thin]">
                <div className="mb-2 flex items-center justify-between gap-2">
                  <h4 className="text-xs font-bold text-[var(--nv-text-secondary)]">操作历史</h4>
                  <Tag tone="neutral">{opHistory.length}</Tag>
                </div>
                {opHistory.length === 0 ? (
                  <p className="text-xs text-[var(--nv-text-tertiary)]">暂无操作记录</p>
                ) : (
                  <div className="space-y-1.5">
                    {opHistory.map(op => (
                      <div
                        key={op.id}
                        className="flex items-center justify-between gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-2 text-xs"
                      >
                        <div className="min-w-0 flex-1">
                          <span className={clsx('font-medium text-[var(--nv-text-primary)]', op.undone && 'line-through opacity-50')}>
                            {op.action}
                          </span>
                          <span className="ml-2 text-[var(--nv-text-tertiary)]">{op.previews?.length || 0} 项</span>
                        </div>
                        {!op.undone && op.previews?.length > 0 && (
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => handleUndo(op.id)}
                            className="h-7 px-2 text-xs text-[var(--nv-status-warning)]"
                          >
                            <Undo2 size={11} aria-hidden={true} />
                            撤销
                          </Button>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            <div
              className="flex-1 space-y-3 overflow-y-auto px-4 py-3 [scrollbar-width:thin]"
              style={{ minHeight: '200px', maxHeight: '400px', scrollBehavior: 'smooth' }}
            >
              {messages.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-8 text-center">
                  <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
                    <Sparkles size={24} aria-hidden={true} />
                  </div>
                  <p className="mb-1 text-sm font-medium text-[var(--nv-text-secondary)]">你好！我是AI文件助手</p>
                  <p className="mb-4 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                    用自然语言告诉我你想做什么<br />
                    比如重命名、刮削、分类整理等
                  </p>
                  <div className="w-full space-y-1.5">
                    {quickCommands.map((cmd) => (
                      <Button
                        key={cmd.label}
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setInput(cmd.cmd)
                          setTimeout(() => inputRef.current?.focus(), 50)
                        }}
                        className="h-auto w-full justify-start px-3 py-2 text-left text-xs text-[var(--nv-text-secondary)]"
                      >
                        {cmd.label}
                      </Button>
                    ))}
                  </div>
                </div>
              ) : (
                messages.map((msg, index) => (
                  <div key={index} className={clsx('flex gap-2', msg.role === 'user' && 'flex-row-reverse')}>
                    <div
                      className={clsx(
                        'flex h-7 w-7 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border',
                        msg.role === 'user'
                          ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
                          : 'border-[var(--nv-action-primary)] bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]',
                      )}
                    >
                      {msg.role === 'user'
                        ? <User size={13} aria-hidden={true} />
                        : <Bot size={13} aria-hidden={true} />}
                    </div>
                    <div
                      className={clsx(
                        'min-w-0 flex-1 rounded-[var(--nv-radius-container)] border px-3 py-2',
                        msg.role === 'user'
                          ? 'ml-8 border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)]'
                          : 'mr-8 border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]',
                      )}
                    >
                      {renderContent(msg.content)}
                      {renderPreviews(msg.previews || [], index)}
                      {renderActions(msg.actions || [])}
                    </div>
                  </div>
                ))
              )}

              {loading && (
                <div className="flex gap-2">
                  <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]">
                    <Bot size={13} aria-hidden={true} />
                  </div>
                  <div className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2">
                    <div className="flex items-center gap-2 text-[var(--nv-text-tertiary)]">
                      <Loader2 size={12} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden={true} />
                      <span className="text-xs">思考中...</span>
                    </div>
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>

            <div className="border-t border-[var(--nv-border-subtle)] px-4 py-3">
              {selectedMediaIds.length > 0 && (
                <div className="mb-2 flex items-center gap-1.5 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-2 py-1 text-[10px] text-[var(--nv-text-tertiary)]">
                  <MessageSquare size={10} aria-hidden={true} />
                  已关联 {selectedMediaIds.length} 个选中文件
                </div>
              )}
              <div className="flex items-end gap-2">
                <Textarea
                  ref={inputRef}
                  value={input}
                  onChange={event => setInput(event.target.value)}
                  onKeyDown={event => {
                    if (event.key === 'Enter' && !event.shiftKey) {
                      event.preventDefault()
                      handleSend()
                    }
                  }}
                  placeholder="输入指令，如：帮我把这些文件按电影格式重命名..."
                  className="max-h-28 min-h-[64px] resize-none text-xs leading-5"
                  rows={2}
                />
                <Button
                  type="button"
                  variant="primary"
                  size="md"
                  iconOnly
                  onClick={handleSend}
                  disabled={!input.trim() || loading}
                  aria-label="发送指令"
                  title="发送指令"
                  className="shrink-0"
                >
                  <Send size={15} aria-hidden={true} />
                </Button>
              </div>
            </div>
          </>
        )}
      </Surface>
    </div>
  )
}

/** AI助手侧边面板包装器（带动画过渡） */
export function AIAssistantPanel({ isOpen, children }: { isOpen: boolean; children: React.ReactNode }) {
  return (
    <div
      className={clsx(
        'flex-shrink-0 overflow-hidden transition-[width,opacity,max-height] duration-300 ease-out',
        isOpen ? 'w-[380px] opacity-100' : 'w-0 opacity-0',
      )}
      style={{ maxHeight: isOpen ? 'calc(100vh - 280px)' : 0 }}
    >
      <div className="h-full w-[380px]">{children}</div>
    </div>
  )
}
