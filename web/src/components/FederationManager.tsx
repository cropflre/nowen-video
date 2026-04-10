import { useState, useEffect } from 'react'
import { federationApi } from '@/api'
import type { ServerNode, FederationStats, SyncTask } from '@/types'
import {
  Network, Plus, Trash2, RefreshCw, Server, Wifi, WifiOff,
  HardDrive, Loader2, Activity, Globe, Clock, X,
} from 'lucide-react'
import clsx from 'clsx'

export default function FederationManager() {
  const [nodes, setNodes] = useState<ServerNode[]>([])
  const [stats, setStats] = useState<FederationStats | null>(null)
  const [syncTasks, setSyncTasks] = useState<SyncTask[]>([])
  const [loading, setLoading] = useState(true)
  const [showAddNode, setShowAddNode] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const loadData = async () => {
    try {
      const [nodesRes, statsRes, tasksRes] = await Promise.all([
        federationApi.listNodes(),
        federationApi.getStats(),
        federationApi.getSyncTasks(),
      ])
      setNodes(nodesRes.data.data || [])
      setStats(statsRes.data.data)
      setSyncTasks(tasksRes.data.data || [])
    } catch { /* ignore */ } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadData() }, [])

  // 定时刷新
  useEffect(() => {
    const timer = setInterval(loadData, 10000)
    return () => clearInterval(timer)
  }, [])

  const handleRemoveNode = async (id: string) => {
    if (!confirm('确定要移除此节点吗？')) return
    try {
      await federationApi.removeNode(id)
      setMessage({ type: 'success', text: '节点已移除' })
      loadData()
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || '移除失败' })
    }
    setTimeout(() => setMessage(null), 3000)
  }

  const handleSync = async (nodeId: string) => {
    try {
      await federationApi.syncNode(nodeId)
      setMessage({ type: 'success', text: '同步已开始' })
      loadData()
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || '同步失败' })
    }
    setTimeout(() => setMessage(null), 3000)
  }

  const formatSize = (bytes: number) => {
    if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(0)} MB`
    return `${(bytes / 1073741824).toFixed(1)} GB`
  }

  if (loading) {
    return (
      <div className="space-y-6 animate-fade-in">
        {/* 标题区骨架 */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="skeleton h-5 w-5 rounded" />
            <div className="skeleton h-6 w-28 rounded-lg" />
          </div>
          <div className="skeleton h-9 w-28 rounded-xl" />
        </div>
        {/* 统计概览骨架 */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="rounded-xl p-4" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
              <div className="flex items-center gap-2 mb-2">
                <div className="skeleton h-4 w-4 rounded" />
                <div className="skeleton h-3 w-14 rounded" />
              </div>
              <div className="skeleton h-6 w-16 rounded-lg" />
            </div>
          ))}
        </div>
        {/* 节点列表骨架 */}
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="rounded-2xl p-5" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
              <div className="flex items-start gap-3 mb-3">
                <div className="skeleton h-10 w-10 rounded-xl" />
                <div className="flex-1 space-y-2">
                  <div className="skeleton h-4 w-1/4 rounded" />
                  <div className="skeleton h-3 w-1/3 rounded" />
                </div>
                <div className="skeleton h-5 w-14 rounded-full" />
              </div>
              <div className="grid grid-cols-4 gap-3 mb-3">
                {Array.from({ length: 4 }).map((_, j) => (
                  <div key={j} className="text-center space-y-1">
                    <div className="skeleton mx-auto h-3 w-10 rounded" />
                    <div className="skeleton mx-auto h-4 w-8 rounded" />
                  </div>
                ))}
              </div>
              <div className="skeleton h-1.5 w-full rounded-full mb-3" />
              <div className="flex items-center gap-2 pt-3" style={{ borderTop: '1px solid var(--border-subtle)' }}>
                <div className="skeleton h-8 flex-1 rounded-lg" />
                <div className="skeleton h-3 w-32 rounded" />
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {message && (
        <div className={clsx(
          'rounded-xl px-4 py-3 text-sm font-medium',
          message.type === 'success' ? 'bg-green-500/10 text-green-400 border border-green-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'
        )}>{message.text}</div>
      )}

      {/* 标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Network className="h-5 w-5 text-neon-blue" />
          <h2 className="font-display text-xl font-semibold text-white">联邦网络</h2>
        </div>
        <button onClick={() => setShowAddNode(true)}
          className="btn-neon rounded-xl px-4 py-2 text-sm font-medium flex items-center gap-2">
          <Plus className="h-4 w-4" /> 添加节点
        </button>
      </div>

      {/* 统计概览 */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {[
            { label: '在线节点', value: `${stats.online_nodes}/${stats.total_nodes}`, icon: Server, color: 'text-green-400' },
            { label: '共享媒体', value: stats.shared_media, icon: Globe, color: 'text-neon-blue' },
            { label: '总媒体数', value: stats.total_media, icon: Activity, color: 'text-neon-purple' },
            { label: '存储使用', value: `${formatSize(stats.used_storage)} / ${formatSize(stats.total_storage)}`, icon: HardDrive, color: 'text-yellow-400' },
          ].map(({ label, value, icon: Icon, color }) => (
            <div key={label} className="card-glass rounded-xl p-4">
              <div className="flex items-center gap-2 mb-2">
                <Icon className={clsx('h-4 w-4', color)} />
                <span className="text-xs text-surface-400">{label}</span>
              </div>
              <p className={clsx('font-display text-lg font-bold', color)}>{value}</p>
            </div>
          ))}
        </div>
      )}

      {/* 节点列表 */}
      <div className="space-y-3">
        {nodes.map(node => (
          <div key={node.id} className={clsx(
            'card-glass rounded-2xl p-5 transition-all',
            node.status === 'online' ? 'border-green-500/10' : 'border-red-500/10 opacity-70'
          )}>
            <div className="flex items-start justify-between mb-3">
              <div className="flex items-center gap-3">
                <div className={clsx(
                  'w-10 h-10 rounded-xl flex items-center justify-center',
                  node.status === 'online' ? 'bg-green-500/20' : 'bg-red-500/20'
                )}>
                  {node.status === 'online' ? <Wifi className="h-5 w-5 text-green-400" /> : <WifiOff className="h-5 w-5 text-red-400" />}
                </div>
                <div>
                  <h3 className="font-display text-sm font-semibold text-white">{node.name}</h3>
                  <p className="text-xs text-surface-400">{node.url}</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <span className={clsx(
                  'text-xs rounded-full px-2 py-0.5',
                  node.role === 'primary' ? 'bg-neon-blue/20 text-neon-blue' :
                  node.role === 'mirror' ? 'bg-yellow-500/20 text-yellow-400' :
                  'bg-surface-800 text-surface-400'
                )}>
                  {node.role === 'primary' ? '主节点' : node.role === 'mirror' ? '镜像' : '对等'}
                </span>
                {node.is_local && (
                  <span className="text-xs rounded-full bg-green-500/20 text-green-400 px-2 py-0.5">本机</span>
                )}
              </div>
            </div>

            {/* 节点状态 */}
            <div className="grid grid-cols-4 gap-3 mb-3">
              <div className="text-center">
                <p className="text-xs text-surface-400">媒体数</p>
                <p className="text-sm font-medium text-white">{node.media_count}</p>
              </div>
              <div className="text-center">
                <p className="text-xs text-surface-400">延迟</p>
                <p className={clsx('text-sm font-medium', node.latency < 100 ? 'text-green-400' : node.latency < 500 ? 'text-yellow-400' : 'text-red-400')}>
                  {node.latency}ms
                </p>
              </div>
              <div className="text-center">
                <p className="text-xs text-surface-400">CPU</p>
                <p className="text-sm font-medium text-white">{node.cpu_usage.toFixed(0)}%</p>
              </div>
              <div className="text-center">
                <p className="text-xs text-surface-400">内存</p>
                <p className="text-sm font-medium text-white">{node.mem_usage.toFixed(0)}%</p>
              </div>
            </div>

            {/* 存储条 */}
            {node.storage_total > 0 && (
              <div className="mb-3">
                <div className="h-1.5 rounded-full bg-surface-700 overflow-hidden">
                  <div className="h-full rounded-full bg-gradient-to-r from-neon-blue to-neon-purple"
                    style={{ width: `${(node.storage_used / node.storage_total) * 100}%` }} />
                </div>
                <p className="text-xs text-surface-400 mt-1">
                  {formatSize(node.storage_used)} / {formatSize(node.storage_total)}
                </p>
              </div>
            )}

            {/* 操作 */}
            <div className="flex items-center gap-2 pt-3 border-t border-white/5">
              <button onClick={() => handleSync(node.id)}
                className="btn-ghost flex-1 rounded-lg py-2 text-xs flex items-center justify-center gap-1">
                <RefreshCw className="h-3 w-3" /> 同步
              </button>
              {node.last_sync && (
                <span className="text-xs text-surface-500 flex items-center gap-1">
                  <Clock className="h-3 w-3" />
                  {new Date(node.last_sync).toLocaleString()}
                </span>
              )}
              {!node.is_local && (
                <button onClick={() => handleRemoveNode(node.id)}
                  className="text-red-400 hover:text-red-300 rounded-lg px-3 py-2 text-xs">
                  <Trash2 className="h-3 w-3" />
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {nodes.length === 0 && (
        <div className="text-center py-16 text-surface-500">
          <Network className="h-16 w-16 mx-auto mb-4 opacity-20" />
          <p className="text-lg">暂无联邦节点</p>
          <p className="text-sm mt-1">添加其他 Nowen Video 服务器以共享媒体资源</p>
        </div>
      )}

      {/* 同步任务 */}
      {syncTasks.length > 0 && (
        <div>
          <h3 className="font-display text-base font-semibold text-white mb-3">同步任务</h3>
          <div className="space-y-2">
            {syncTasks.map(task => (
              <div key={task.id} className="card-glass rounded-xl p-3 flex items-center gap-3">
                {task.status === 'running' ? (
                  <Loader2 className="h-4 w-4 animate-spin text-neon-blue shrink-0" />
                ) : task.status === 'completed' ? (
                  <div className="h-4 w-4 rounded-full bg-green-400 shrink-0" />
                ) : (
                  <div className="h-4 w-4 rounded-full bg-surface-600 shrink-0" />
                )}
                <div className="flex-1 min-w-0">
                  <p className="text-xs text-white">{task.type === 'full' ? '全量同步' : task.type === 'incremental' ? '增量同步' : '元数据同步'}</p>
                  {task.status === 'running' && (
                    <div className="h-1 rounded-full bg-surface-700 mt-1 overflow-hidden">
                      <div className="h-full rounded-full bg-neon-blue" style={{ width: `${task.progress}%` }} />
                    </div>
                  )}
                </div>
                <span className="text-xs text-surface-400">{task.synced}/{task.total}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 添加节点弹窗 */}
      {showAddNode && (
        <AddNodeModal
          onClose={() => setShowAddNode(false)}
          onAdded={() => { setShowAddNode(false); loadData() }}
        />
      )}
    </div>
  )
}

function AddNodeModal({ onClose, onAdded }: { onClose: () => void; onAdded: () => void }) {
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [role, setRole] = useState('peer')
  const [saving, setSaving] = useState(false)

  const handleAdd = async () => {
    if (!name || !url || !apiKey) return
    setSaving(true)
    try {
      await federationApi.registerNode({ name, url, api_key: apiKey, role })
      onAdded()
    } catch { /* ignore */ } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-2xl p-6" style={{
        background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-15)', backdropFilter: 'blur(20px)',
      }}>
        <div className="flex items-center justify-between mb-6">
          <h3 className="font-display text-lg font-semibold text-white">添加联邦节点</h3>
          <button onClick={onClose} className="text-surface-400 hover:text-white"><X className="h-5 w-5" /></button>
        </div>

        <div className="space-y-4">
          <div>
            <label className="text-xs text-surface-400">节点名称</label>
            <input type="text" value={name} onChange={e => setName(e.target.value)}
              placeholder="我的 NAS" className="input-glass mt-1 w-full rounded-lg px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="text-xs text-surface-400">服务器地址</label>
            <input type="url" value={url} onChange={e => setUrl(e.target.value)}
              placeholder="http://192.168.1.100:8080" className="input-glass mt-1 w-full rounded-lg px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="text-xs text-surface-400">API Key</label>
            <input type="password" value={apiKey} onChange={e => setApiKey(e.target.value)}
              className="input-glass mt-1 w-full rounded-lg px-3 py-2 text-sm" />
          </div>
          <div>
            <label className="text-xs text-surface-400">角色</label>
            <select value={role} onChange={e => setRole(e.target.value)}
              className="input-glass mt-1 w-full rounded-lg px-3 py-2 text-sm">
              <option value="peer">对等节点</option>
              <option value="mirror">镜像节点</option>
            </select>
          </div>

          <div className="flex gap-3 pt-2">
            <button onClick={onClose} className="btn-ghost flex-1 rounded-xl py-2.5 text-sm">取消</button>
            <button onClick={handleAdd} disabled={saving || !name || !url || !apiKey}
              className="btn-neon flex-1 rounded-xl py-2.5 text-sm font-medium flex items-center justify-center gap-2">
              {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
              添加
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
