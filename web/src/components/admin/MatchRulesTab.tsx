import { useState, useEffect, useCallback } from 'react'
import type { MatchRule, Library } from '@/types'
import { matchRuleApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Filter,
  Plus,
  Trash2,
  Edit3,
  Check,
  X,
  Loader2,
  Play,
  ToggleLeft,
  ToggleRight,
  Zap,
  FileText,
  FolderOpen,
  Code,
  Search,
  AlertCircle,
} from 'lucide-react'
import clsx from 'clsx'

// 规则类型选项
const RULE_TYPES = [
  { value: 'filename', label: '文件名匹配', icon: FileText, desc: '匹配文件名中的关键词' },
  { value: 'path', label: '路径匹配', icon: FolderOpen, desc: '匹配文件路径中的关键词' },
  { value: 'regex', label: '正则表达式', icon: Code, desc: '使用正则表达式精确匹配' },
  { value: 'keyword', label: '关键词匹配', icon: Search, desc: '多个关键词用逗号分隔' },
]

// 动作类型选项
const ACTION_TYPES = [
  { value: 'set_type', label: '设置类型', desc: '将匹配的媒体设为指定类型（movie/episode）' },
  { value: 'set_genre', label: '设置分类', desc: '自动添加指定分类标签' },
  { value: 'set_tag', label: '添加标签', desc: '自动添加指定标签' },
  { value: 'skip', label: '跳过扫描', desc: '扫描时忽略匹配的文件' },
  { value: 'set_library', label: '指定媒体库', desc: '将匹配的文件归入指定媒体库' },
]

export default function MatchRulesTab() {
  const toast = useToast()
  const [rules, setRules] = useState<MatchRule[]>([])
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)

  // 创建表单
  const [formName, setFormName] = useState('')
  const [formDesc, setFormDesc] = useState('')
  const [formType, setFormType] = useState('filename')
  const [formPattern, setFormPattern] = useState('')
  const [formAction, setFormAction] = useState('set_type')
  const [formActionValue, setFormActionValue] = useState('')
  const [formLibrary, setFormLibrary] = useState('')
  const [formPriority, setFormPriority] = useState(0)
  const [creating, setCreating] = useState(false)

  // 测试
  const [testInput, setTestInput] = useState('')
  const [testResult, setTestResult] = useState<boolean | null>(null)
  const [testing, setTesting] = useState(false)

  const fetchRules = useCallback(async () => {
    try {
      const res = await matchRuleApi.list()
      setRules(res.data.data || [])
    } catch {
      toast.error('加载匹配规则失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchRules()
    libraryApi.list().then(res => setLibraries(res.data.data || [])).catch(() => {})
  }, [fetchRules])

  const handleCreate = async () => {
    if (!formName.trim() || !formPattern.trim()) {
      toast.error('名称和匹配模式不能为空')
      return
    }
    setCreating(true)
    try {
      await matchRuleApi.create({
        name: formName.trim(),
        description: formDesc,
        rule_type: formType,
        pattern: formPattern,
        action: formAction,
        action_value: formActionValue,
        library_id: formLibrary,
        priority: formPriority,
      })
      toast.success('匹配规则已创建')
      setShowCreate(false)
      resetForm()
      fetchRules()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '创建失败')
    } finally {
      setCreating(false)
    }
  }

  const resetForm = () => {
    setFormName('')
    setFormDesc('')
    setFormType('filename')
    setFormPattern('')
    setFormAction('set_type')
    setFormActionValue('')
    setFormLibrary('')
    setFormPriority(0)
  }

  const handleToggle = async (rule: MatchRule) => {
    try {
      await matchRuleApi.update(rule.id, { enabled: !rule.enabled })
      setRules(prev => prev.map(r => r.id === rule.id ? { ...r, enabled: !r.enabled } : r))
      toast.success(rule.enabled ? '已禁用' : '已启用')
    } catch {
      toast.error('操作失败')
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除该匹配规则？')) return
    try {
      await matchRuleApi.delete(id)
      toast.success('规则已删除')
      fetchRules()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleTest = async () => {
    if (!formPattern.trim() || !testInput.trim()) {
      toast.error('请输入匹配模式和测试文本')
      return
    }
    setTesting(true)
    try {
      const res = await matchRuleApi.test({
        rule_type: formType,
        pattern: formPattern,
        test_input: testInput,
      })
      setTestResult(res.data.data.matched)
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '测试失败')
      setTestResult(null)
    } finally {
      setTesting(false)
    }
  }

  const getRuleTypeIcon = (type: string) => {
    const found = RULE_TYPES.find(t => t.value === type)
    return found ? found.icon : FileText
  }

  return (
    <section>
      <div className="mb-4 flex items-center justify-between">
        <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Filter size={20} className="text-neon/60" />
          自定义匹配规则
        </h2>
        <button
          onClick={() => setShowCreate(!showCreate)}
          className="btn-primary gap-1.5 px-3.5 py-2 text-xs"
        >
          <Plus size={14} />
          新建规则
        </button>
      </div>

      <p className="mb-4 text-xs text-surface-500">
        💡 匹配规则在扫描媒体库时自动应用。可以根据文件名、路径等条件自动设置媒体类型、分类或标签。优先级越高越先执行。
      </p>

      {/* 创建表单 */}
      {showCreate && (
        <div className="glass-panel mb-4 animate-slide-up rounded-xl p-5 space-y-4">
          <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>创建匹配规则</h3>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>规则名称</label>
              <input
                type="text"
                value={formName}
                onChange={(e) => setFormName(e.target.value)}
                className="input w-full"
                placeholder="如：动画片自动分类"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>描述（可选）</label>
              <input
                type="text"
                value={formDesc}
                onChange={(e) => setFormDesc(e.target.value)}
                className="input w-full"
                placeholder="规则用途说明"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>匹配类型</label>
              <select
                value={formType}
                onChange={(e) => setFormType(e.target.value)}
                className="input w-full"
              >
                {RULE_TYPES.map(t => (
                  <option key={t.value} value={t.value}>{t.label} — {t.desc}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>匹配模式</label>
              <input
                type="text"
                value={formPattern}
                onChange={(e) => setFormPattern(e.target.value)}
                className="input w-full font-mono"
                placeholder={formType === 'regex' ? '如：\\[BDRip\\].*\\.mkv$' : '如：动画,anime'}
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>执行动作</label>
              <select
                value={formAction}
                onChange={(e) => setFormAction(e.target.value)}
                className="input w-full"
              >
                {ACTION_TYPES.map(a => (
                  <option key={a.value} value={a.value}>{a.label} — {a.desc}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>动作参数值</label>
              <input
                type="text"
                value={formActionValue}
                onChange={(e) => setFormActionValue(e.target.value)}
                className="input w-full"
                placeholder={formAction === 'set_type' ? 'movie 或 episode' : '参数值'}
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>限定媒体库（可选）</label>
              <select
                value={formLibrary}
                onChange={(e) => setFormLibrary(e.target.value)}
                className="input w-full"
              >
                <option value="">全局（所有媒体库）</option>
                {libraries.map(lib => (
                  <option key={lib.id} value={lib.id}>{lib.name}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>优先级</label>
              <input
                type="number"
                value={formPriority}
                onChange={(e) => setFormPriority(parseInt(e.target.value) || 0)}
                className="input w-full"
                placeholder="数字越大越先执行"
              />
            </div>
          </div>

          {/* 测试区域 */}
          <div className="rounded-lg p-3" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
            <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>🧪 测试匹配</label>
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={testInput}
                onChange={(e) => { setTestInput(e.target.value); setTestResult(null) }}
                className="input flex-1 font-mono text-xs"
                placeholder="输入测试文件名或路径..."
              />
              <button
                onClick={handleTest}
                disabled={testing}
                className="btn-primary gap-1 px-3 py-1.5 text-xs"
              >
                {testing ? <Loader2 size={12} className="animate-spin" /> : <Play size={12} />}
                测试
              </button>
              {testResult !== null && (
                <span className={clsx('text-xs font-medium', testResult ? 'text-green-400' : 'text-red-400')}>
                  {testResult ? '✅ 匹配' : '❌ 不匹配'}
                </span>
              )}
            </div>
          </div>

          <div className="flex items-center justify-end gap-2 pt-2">
            <button onClick={() => { setShowCreate(false); resetForm() }} className="btn-ghost px-4 py-2 text-sm">取消</button>
            <button onClick={handleCreate} disabled={creating} className="btn-primary gap-1.5 px-4 py-2 text-sm">
              {creating ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
              创建
            </button>
          </div>
        </div>
      )}

      {/* 规则列表 */}
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 size={24} className="animate-spin text-neon/40" />
        </div>
      ) : rules.length > 0 ? (
        <div className="space-y-2">
          {rules.map(rule => {
            const TypeIcon = getRuleTypeIcon(rule.rule_type)
            return (
              <div
                key={rule.id}
                className={clsx(
                  'glass-panel-subtle group flex items-center gap-4 rounded-xl p-4 transition-all hover:border-neon-blue/20',
                  !rule.enabled && 'opacity-50'
                )}
              >
                {/* 图标 */}
                <div className={clsx(
                  'flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg',
                  rule.enabled ? 'bg-neon-blue/10 text-neon' : 'bg-surface-800 text-surface-500'
                )}>
                  <TypeIcon size={16} />
                </div>

                {/* 信息 */}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{rule.name}</span>
                    <span className="rounded-md px-1.5 py-0.5 text-[10px] font-medium" style={{
                      background: 'var(--nav-hover-bg)',
                      color: 'var(--text-tertiary)',
                    }}>
                      {RULE_TYPES.find(t => t.value === rule.rule_type)?.label || rule.rule_type}
                    </span>
                    <span className="rounded-md px-1.5 py-0.5 text-[10px] font-medium text-amber-400" style={{
                      background: 'rgba(245,158,11,0.1)',
                    }}>
                      {ACTION_TYPES.find(a => a.value === rule.action)?.label || rule.action}
                      {rule.action_value && `: ${rule.action_value}`}
                    </span>
                  </div>
                  <div className="mt-0.5 flex items-center gap-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                    <span className="font-mono">{rule.pattern}</span>
                    <span>优先级: {rule.priority}</span>
                    <span>命中: {rule.hit_count} 次</span>
                    {rule.description && <span className="text-surface-600">{rule.description}</span>}
                  </div>
                </div>

                {/* 操作 */}
                <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                  <button
                    onClick={() => handleToggle(rule)}
                    className="rounded-lg p-1.5 text-surface-400 hover:text-yellow-400 hover:bg-yellow-400/5"
                    title={rule.enabled ? '禁用' : '启用'}
                  >
                    {rule.enabled ? <ToggleRight size={14} className="text-green-400" /> : <ToggleLeft size={14} />}
                  </button>
                  <button
                    onClick={() => handleDelete(rule.id)}
                    className="rounded-lg p-1.5 text-surface-400 hover:text-red-400 hover:bg-red-400/5"
                    title="删除"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
          <div>
            <Filter size={32} className="mx-auto mb-2 text-surface-600" />
            <p className="text-sm text-surface-500">暂无匹配规则</p>
            <p className="mt-1 text-xs text-surface-600">点击「新建规则」添加自动分类、标签等规则</p>
          </div>
        </div>
      )}
    </section>
  )
}
