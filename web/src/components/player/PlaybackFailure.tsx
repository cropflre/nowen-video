import { useAuthStore } from '@/stores/auth'

interface PlaybackFailureProps {
  detail: string
  onRetry?: () => void
  onBack?: () => void
}

export default function PlaybackFailure({ detail, onRetry, onBack }: PlaybackFailureProps) {
  const isAdmin = useAuthStore((state) => state.user?.role === 'admin')
  return (
    <div className="flex h-full w-full flex-col items-center justify-center gap-4 bg-[var(--nv-player-canvas)] p-6 text-center text-[var(--nv-player-text-primary)]" role="alert">
      <h2 className="text-lg font-semibold">这部影片暂时无法播放</h2>
      <p className="text-sm text-[var(--nv-player-text-secondary)]">可以再试一次，或返回选择其他影片。</p>
      <div className="flex flex-wrap justify-center gap-3">
        {onRetry && <button type="button" onClick={onRetry} className="nv-button" data-variant="primary">重试</button>}
        {onBack && <button type="button" onClick={onBack} className="nv-button" data-variant="secondary">返回</button>}
      </div>
      {isAdmin && <details className="max-w-xl break-words text-sm text-[var(--nv-player-text-secondary)]">
        <summary className="cursor-pointer py-3">查看故障详情</summary>
        <p>{detail}</p>
      </details>}
    </div>
  )
}
