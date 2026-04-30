/**
 * Anime4KPanel
 *
 * Anime4K 超分档位选择：off / low / medium / high
 *
 * 背后对应不同的 GLSL shader 链，在父组件 MpvEmbedPlayer 里
 * 通过 mpv 属性 `glsl-shaders` 应用到 libmpv 渲染管线。
 *
 * 档位选择依据（基于 Anime4K v4.0.1 官方推荐）：
 *  - off    → 原图，不做任何处理
 *  - low    → CNN_M 系，GPU 开销 ~10%，1080p->4K 约 5ms/帧
 *  - medium → CNN_VL 系，GPU 开销 ~25%，质量明显提升
 *  - high   → CNN_UL + AutoDownscale，GPU 开销 ~50%，最高质量
 */

import { CheckCircle2, Zap } from 'lucide-react'
import type { Anime4KLevel } from './bridge'

export type { Anime4KLevel }

interface Props {
  value: Anime4KLevel
  onChange: (level: Anime4KLevel) => void
}

interface LevelMeta {
  key: Anime4KLevel
  label: string
  desc: string
  badge?: string
}

const LEVELS: LevelMeta[] = [
  { key: 'off', label: '关闭', desc: '原始画质，零开销' },
  { key: 'low', label: '低', desc: 'CNN-M · 约 +10% GPU', badge: '节能' },
  { key: 'medium', label: '中', desc: 'CNN-VL · 约 +25% GPU', badge: '推荐' },
  { key: 'high', label: '高', desc: 'CNN-UL · 约 +50% GPU', badge: '极致' },
]

export default function Anime4KPanel({ value, onChange }: Props) {
  return (
    <div
      className="rounded-2xl p-3 min-w-[280px] shadow-2xl backdrop-blur-xl"
      style={{
        background: 'rgba(20, 20, 28, 0.88)',
        border: '1px solid rgba(255,255,255,0.08)',
      }}
    >
      <div className="flex items-center gap-2 mb-2 px-1">
        <Zap className="w-4 h-4 text-violet-400" />
        <span className="text-sm font-semibold text-white/90">Anime4K 超分</span>
        <span className="ml-auto text-[10px] text-white/40 uppercase tracking-wider">
          GPU Shader
        </span>
      </div>

      <div className="space-y-1">
        {LEVELS.map((lv) => {
          const active = value === lv.key
          return (
            <button
              key={lv.key}
              onClick={() => onChange(lv.key)}
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left transition
                ${active
                  ? 'bg-gradient-to-r from-violet-500/40 to-fuchsia-500/30 border border-violet-400/40'
                  : 'bg-white/[0.03] hover:bg-white/10 border border-transparent'}`}
            >
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className={`text-sm ${active ? 'text-white' : 'text-white/85'} font-medium`}>
                    {lv.label}
                  </span>
                  {lv.badge && (
                    <span
                      className={`text-[10px] px-1.5 py-0.5 rounded
                        ${lv.key === 'medium'
                          ? 'bg-emerald-500/20 text-emerald-300'
                          : lv.key === 'high'
                            ? 'bg-red-500/20 text-red-300'
                            : 'bg-blue-500/20 text-blue-300'}`}
                    >
                      {lv.badge}
                    </span>
                  )}
                </div>
                <div className="text-[11px] text-white/50 mt-0.5 truncate">{lv.desc}</div>
              </div>
              {active && <CheckCircle2 className="w-4 h-4 text-violet-300 shrink-0" />}
            </button>
          )
        })}
      </div>

      <div className="mt-2 px-1 text-[10px] text-white/40 leading-relaxed">
        适合动漫和 2D 插画。3D 真人影片建议"关闭"，实时超分对人脸会产生伪影。
      </div>
    </div>
  )
}
