export default function PageLoader() {
  return (
    <div className="nv-page-loader flex min-h-[min(60dvh,24rem)] items-center justify-center px-4" role="status">
      <div className="nv-page-loader-indicator flex flex-col items-center gap-3">
        <span
          className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--nv-border-strong)] border-t-[var(--nv-action-primary)] motion-reduce:animate-none"
          aria-hidden="true"
        />
        <span className="text-sm font-medium text-[var(--nv-text-secondary)]">页面加载中…</span>
      </div>
    </div>
  )
}
