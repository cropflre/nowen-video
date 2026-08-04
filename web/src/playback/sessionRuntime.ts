export interface PlaybackSessionRuntimeState {
  sessionId: string
  generationId: number
  offsetSeconds: number
}

const activeSessions = new Map<string, PlaybackSessionRuntimeState>()

export function setPlaybackSessionRuntime(
  mediaId: string,
  state: PlaybackSessionRuntimeState,
): void {
  activeSessions.set(mediaId, {
    ...state,
    offsetSeconds: Math.max(0, state.offsetSeconds),
  })
}

export function clearPlaybackSessionRuntime(mediaId: string, sessionId?: string): void {
  const current = activeSessions.get(mediaId)
  if (!current) return
  if (sessionId && current.sessionId !== sessionId) return
  activeSessions.delete(mediaId)
}

export function getPlaybackSessionRuntime(mediaId: string): PlaybackSessionRuntimeState | null {
  return activeSessions.get(mediaId) || null
}

export function toAbsolutePlaybackPosition(mediaId: string, relativeSeconds: number): number {
  const runtime = activeSessions.get(mediaId)
  const relative = Number.isFinite(relativeSeconds) ? Math.max(0, relativeSeconds) : 0
  return runtime ? runtime.offsetSeconds + relative : relative
}
