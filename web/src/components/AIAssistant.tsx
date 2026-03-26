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

interface AIAssistantProps {
  /** 当前选中的文件ID列表 */
  selectedMediaIds: string[]
  /** 当前媒体库ID */
  libraryId?: string
  /** 操作执行后的回调（用于刷新列表） */
  onOperationComplete?: () => void
}

export default function AIAssistant({ selectedMediaIds, libraryId, onOperationComplete }: AIAssistantProps) {
  const toast = useToast()
  const [isOpen, setIsOpen] = useState(false)
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

  // 自动滚动到底部
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  useEffect(() => {
    scrollToBottom()
  }, [messages, scrollToBottom])

  // 打开时聚焦输入框
  useEffect(() => {
    if (isOpen && !isMinimized) {
      setTimeout(() => inputRef.current?.focus(), 100)
    }
  }, [isOpen, isMinimized])

  // 发送消息
  const handleSend = async () => {
    if (!input.trim() || loading) return

    const userMessage = input.trim()
    setInput('')

    // 添加用户消息到列表
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

  // 执行操作
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

  // 撤销操作
  const handleUndo = async (opId: string) => {
    try {
      const res = await aiAssistantApi.undoOperation(opId)
      const data = res.data.data
      toast.success(data.message)
      onOperationComplete?.()
      // 刷新历史
      loadHistory()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '撤销失败')
    }
  }

  // 加载操作历史
  const loadHistory = async () => {
    try {
      const res = await aiAssistantApi.getOperationHistory(20)
      setOpHistory(res.data.data || [])
    } catch {
      // 忽略
    }
  }

  // 新建会话
  const handleNewSession = () => {
    setSessionId('')
    setMessages([])
    setExpandedPreviews(new Set())
  }

  // 切换预览展开
  const togglePreview = (index: number) => {
    setExpandedPreviews(prev => {
      const next = new Set(prev)
      if (next.has(index)) {
        next.delete(index)
      } else {
        next.add(index)
      }
      return next
    })
  }

  // 快捷指令
  const quickCommands = [
    { label: '📊 分析文件库', cmd: '分析当前文件库的整体状态，给出优化建议' },
    { label: '✨ 智能重命名', cmd: '帮我把选中的文件按照规范格式重命名' },
    { label: '🔍 批量刮削', cmd: '为选中的文件批量获取元数据' },
    { label: '🏷️ 自动分类', cmd: '分析选中文件的内容，自动添加合适的标签分类' },
    { label: '🔧 检测问题', cmd: '检查选中文件是否有命名不规范、信息缺失等问题' },
    { label: '🎬 误分类检测', cmd: '分析文件库中被误标记为电影的剧集文件，提供重分类建议' },
  ]

  // 渲染消息内容（支持简单Markdown）
  const renderContent = (content: string) => {
    // 简单的Markdown渲染
    const lines = content.split('\n')
    return lines.map((line, i) => {
      if (line.startsWith('## ')) {
        return <h3 key={i} className="text-sm font-bold mt-2 mb-1" style={{ color: 'var(--text-primary)' }}>{line.slice(3)}</h3>
      }
      if (line.startsWith('### ')) {
        return <h4 key={i} className="text-xs font-bold mt-2 mb-1" style={{ color: 'var(--text-primary)' }}>{line.slice(4)}</h4>
      }
      if (line.startsWith('- ')) {
        return <div key={i} className="text-xs pl-2 py-0.5" style={{ color: 'var(--text-secondary)' }}>{line}</div>
      }
      if (line.startsWith('| ')) {
        return <div key={i} className="text-xs font-mono py-0.5" style={{ color: 'var(--text-secondary)' }}>{line}</div>
      }
      if (line.startsWith('**') && line.endsWith('**')) {
        return <div key={i} className="text-xs font-bold" style={{ color: 'var(--text-primary)' }}>{line.slice(2, -2)}</div>
      }
      if (line.trim() === '') return <div key={i} className="h-1" />
      return <div key={i} className="text-xs" style={{ color: 'var(--text-secondary)' }}>{line}</div>
    })
  }

  // 渲染预览列表
  const renderPreviews = (previews: OperationPreview[], msgIndex: number) => {
    if (!previews || previews.length === 0) return null
    const isExpanded = expandedPreviews.has(msgIndex)

    return (
      <div className="mt-2">
        <button
          onClick={() => togglePreview(msgIndex)}
          className="flex items-center gap-1 text-xs hover:text-neon transition-colors"
          style={{ color: 'var(--text-tertiary)' }}
        >
          {isExpanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          预览变更 ({previews.length} 项)
        </button>
        {isExpanded && (
          <div className="mt-1.5 space-y-1 max-h-48 overflow-y-auto" style={{ scrollbarWidth: 'thin' }}>
            {previews.map((p, i) => (
              <div key={i} className="flex items-start gap-2 p-1.5 rounded text-xs" style={{ background: 'var(--bg-tertiary)' }}>
                <span className="text-surface-500 font-mono flex-shrink-0 w-4 text-right">{i + 1}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-red-400/80 line-through truncate">{p.old_value}</div>
                  <div className="flex items-center gap-1">
                    <ArrowRight size={10} className="text-surface-500 flex-shrink-0" />
                    <span className="text-green-400 truncate">{p.new_value}</span>
                  </div>
                </div>
                <span className="text-[10px] px-1 py-0.5 rounded flex-shrink-0"
                  style={{ background: 'var(--bg-secondary)', color: 'var(--text-tertiary)' }}>
                  {p.change_type}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }

  // 渲染操作按钮
  const renderActions = (actions: SuggestedAction[]) => {
    if (!actions || actions.length === 0) return null

    return (
      <div className="mt-2 flex flex-wrap gap-1.5">
        {actions.map(action => (
          <button
            key={action.id}
            onClick={() => handleExecuteAction(action)}
            disabled={!!executing}
            className={clsx(
              'flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all',
              action.dangerous
                ? 'border border-red-500/30 text-red-400 hover:bg-red-500/10'
                : 'border border-neon/30 text-neon hover:bg-neon-blue/10',
              executing === action.id && 'opacity-50'
            )}
            title={action.description}
          >
            {executing === action.id ? (
              <Loader2 size={12} className="animate-spin" />
            ) : action.dangerous ? (
              <AlertTriangle size={12} />
            ) : (
              <Play size={12} />
            )}
            {action.label}
          </button>
        ))}
      </div>
    )
  }

  if (!isOpen) {
    // 浮动按钮
    return (
      <button
        onClick={() => setIsOpen(true)}
        className="fixed bottom-6 right-6 z-50 flex items-center gap-2 px-4 py-3 rounded-2xl shadow-2xl transition-all hover:scale-105 active:scale-95"
        style={{
          background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
          boxShadow: '0 4px 24px rgba(0, 170, 255, 0.3)',
          color: 'var(--text-on-neon)',
        }}
      >
        <Bot size={20} />
        <span className="text-sm font-medium">AI 助手</span>
        {selectedMediaIds.length > 0 && (
          <span className="ml-1 px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-white/20">
            {selectedMediaIds.length}
          </span>
        )}
      </button>
    )
  }

  return (
    <div
      className={clsx(
        'fixed z-50 transition-all duration-300 ease-out',
        isMinimized
          ? 'bottom-6 right-6 w-72'
          : 'bottom-6 right-6 w-[440px] max-h-[calc(100vh-48px)]'
      )}
    >
      <div className="glass-panel-strong rounded-2xl shadow-2xl flex flex-col overflow-hidden"
        style={{
          maxHeight: isMinimized ? 'auto' : 'calc(100vh - 48px)',
          border: '1px solid rgba(0, 170, 255, 0.15)',
        }}
      >
        {/* 标题栏 */}
        <div className="flex items-center justify-between px-4 py-3 border-b"
          style={{ borderColor: 'var(--border-default)', background: 'var(--bg-secondary)' }}>
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg flex items-center justify-center"
              style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' }}>
              <Bot size={16} style={{ color: 'var(--text-on-neon)' }} />
            </div>
            <div>
              <h3 className="text-sm font-bold" style={{ color: 'var(--text-primary)' }}>AI 文件助手</h3>
              <p className="text-[10px]" style={{ color: 'var(--text-tertiary)' }}>
                {selectedMediaIds.length > 0 ? `已选 ${selectedMediaIds.length} 个文件` : '自然语言管理文件'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-1">
            <button onClick={() => { setShowHistory(!showHistory) ; if (!showHistory) loadHistory() }}
              className="p-1.5 rounded-lg transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-tertiary)' }} title="操作历史">
              <History size={14} />
            </button>
            <button onClick={handleNewSession}
              className="p-1.5 rounded-lg transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-tertiary)' }} title="新建会话">
              <Trash2 size={14} />
            </button>
            <button onClick={() => setIsMinimized(!isMinimized)}
              className="p-1.5 rounded-lg transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-tertiary)' }}>
              {isMinimized ? <Maximize2 size={14} /> : <Minimize2 size={14} />}
            </button>
            <button onClick={() => setIsOpen(false)}
              className="p-1.5 rounded-lg transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-tertiary)' }}>
              <X size={14} />
            </button>
          </div>
        </div>

        {!isMinimized && (
          <>
            {/* 操作历史面板 */}
            {showHistory && (
              <div className="border-b px-4 py-3 max-h-48 overflow-y-auto" style={{ borderColor: 'var(--border-default)', scrollbarWidth: 'thin' }}>
                <h4 className="text-xs font-bold mb-2" style={{ color: 'var(--text-secondary)' }}>操作历史</h4>
                {opHistory.length === 0 ? (
                  <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>暂无操作记录</p>
                ) : (
                  <div className="space-y-1.5">
                    {opHistory.map(op => (
                      <div key={op.id} className="flex items-center justify-between p-2 rounded-lg text-xs"
                        style={{ background: 'var(--bg-secondary)' }}>
                        <div className="flex-1 min-w-0">
                          <span className={clsx('font-medium', op.undone ? 'line-through text-surface-500' : '')}
                            style={{ color: op.undone ? undefined : 'var(--text-primary)' }}>
                            {op.action}
                          </span>
                          <span className="ml-2" style={{ color: 'var(--text-tertiary)' }}>
                            {op.previews?.length || 0} 项
                          </span>
                        </div>
                        {!op.undone && op.previews?.length > 0 && (
                          <button onClick={() => handleUndo(op.id)}
                            className="flex items-center gap-1 px-2 py-1 rounded text-xs text-amber-400 hover:bg-amber-400/10 transition-colors">
                            <Undo2 size={10} /> 撤销
                          </button>
                        )}
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* 消息列表 */}
            <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3"
              style={{
                minHeight: '200px',
                maxHeight: '400px',
                scrollBehavior: 'smooth',
                scrollbarWidth: 'thin',
                scrollbarColor: 'rgba(255,255,255,0.1) transparent',
              }}>
              {messages.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-8">
                  <div className="w-12 h-12 rounded-2xl flex items-center justify-center mb-3"
                    style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))', opacity: 0.3 }}>
                    <Sparkles size={24} style={{ color: 'var(--text-on-neon)' }} />
                  </div>
                  <p className="text-sm font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
                    你好！我是AI文件助手
                  </p>
                  <p className="text-xs text-center mb-4" style={{ color: 'var(--text-tertiary)' }}>
                    用自然语言告诉我你想做什么<br />
                    比如重命名、刮削、分类整理等
                  </p>
                  {/* 快捷指令 */}
                  <div className="w-full space-y-1.5">
                    {quickCommands.map((cmd, i) => (
                      <button
                        key={i}
                        onClick={() => { setInput(cmd.cmd); setTimeout(() => inputRef.current?.focus(), 50) }}
                        className="w-full text-left px-3 py-2 rounded-lg text-xs transition-all hover:ring-1 hover:ring-neon/20"
                        style={{ background: 'var(--bg-secondary)', color: 'var(--text-secondary)' }}
                      >
                        {cmd.label}
                      </button>
                    ))}
                  </div>
                </div>
              ) : (
                messages.map((msg, i) => (
                  <div key={i} className={clsx('flex gap-2', msg.role === 'user' ? 'flex-row-reverse' : '')}>
                    {/* 头像 */}
                    <div className={clsx(
                      'w-6 h-6 rounded-lg flex items-center justify-center flex-shrink-0',
                      msg.role === 'user' ? 'bg-neon-blue/20' : ''
                    )}
                      style={msg.role === 'assistant' ? { background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' } : undefined}>
                      {msg.role === 'user'
                        ? <User size={12} className="text-neon" />
                        : <Bot size={12} style={{ color: 'var(--text-on-neon)' }} />
                      }
                    </div>
                    {/* 消息内容 */}
                    <div className={clsx(
                      'flex-1 min-w-0 rounded-xl px-3 py-2',
                      msg.role === 'user' ? 'bg-neon-blue/10 ml-8' : 'mr-8'
                    )}
                      style={msg.role === 'assistant' ? { background: 'var(--bg-secondary)' } : undefined}>
                      {renderContent(msg.content)}
                      {renderPreviews(msg.previews || [], i)}
                      {renderActions(msg.actions || [])}
                    </div>
                  </div>
                ))
              )}
              {loading && (
                <div className="flex gap-2">
                  <div className="w-6 h-6 rounded-lg flex items-center justify-center flex-shrink-0"
                    style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' }}>
                    <Bot size={12} style={{ color: 'var(--text-on-neon)' }} />
                  </div>
                  <div className="rounded-xl px-3 py-2" style={{ background: 'var(--bg-secondary)' }}>
                    <div className="flex items-center gap-2">
                      <Loader2 size={12} className="animate-spin text-neon" />
                      <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>思考中...</span>
                    </div>
                  </div>
                </div>
              )}
              <div ref={messagesEndRef} />
            </div>

            {/* 输入区域 */}
            <div className="border-t px-4 py-3" style={{ borderColor: 'var(--border-default)' }}>
              {/* 选中文件提示 */}
              {selectedMediaIds.length > 0 && (
                <div className="flex items-center gap-1.5 mb-2 px-2 py-1 rounded-lg text-[10px]"
                  style={{ background: 'var(--bg-secondary)', color: 'var(--text-tertiary)' }}>
                  <MessageSquare size={10} />
                  已关联 {selectedMediaIds.length} 个选中文件
                </div>
              )}
              <div className="flex items-end gap-2">
                <textarea
                  ref={inputRef}
                  value={input}
                  onChange={e => setInput(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault()
                      handleSend()
                    }
                  }}
                  placeholder="输入指令，如：帮我把这些文件按电影格式重命名..."
                  className="flex-1 resize-none rounded-xl px-3 py-2 text-xs outline-none transition-all focus:ring-1 focus:ring-neon/30"
                  style={{
                    background: 'var(--bg-secondary)',
                    color: 'var(--text-primary)',
                    minHeight: '36px',
                    maxHeight: '80px',
                  }}
                  rows={1}
                />
                <button
                  onClick={handleSend}
                  disabled={!input.trim() || loading}
                  className={clsx(
                    'flex items-center justify-center w-9 h-9 rounded-xl transition-all',
                    input.trim() && !loading
                      ? 'hover:scale-105 active:scale-95'
                      : 'opacity-30 cursor-not-allowed'
                  )}
                  style={{
                    background: input.trim() && !loading
                      ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))'
                      : 'var(--bg-secondary)',
                    color: input.trim() && !loading ? 'var(--text-on-neon)' : 'var(--text-tertiary)',
                  }}
                >
                  <Send size={14} />
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
