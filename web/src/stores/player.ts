import { create } from 'zustand'

interface PlayerState {
  // 当前播放状态
  isPlaying: boolean
  currentTime: number
  duration: number
  volume: number
  isMuted: boolean
  isFullscreen: boolean
  quality: string
  showControls: boolean

  // 动作
  setPlaying: (isPlaying: boolean) => void
  setCurrentTime: (time: number) => void
  setDuration: (duration: number) => void
  setVolume: (volume: number) => void
  setMuted: (isMuted: boolean) => void
  setFullscreen: (isFullscreen: boolean) => void
  setQuality: (quality: string) => void
  setShowControls: (show: boolean) => void
  reset: () => void
}

const initialState = {
  isPlaying: false,
  currentTime: 0,
  duration: 0,
  volume: 1,
  isMuted: false,
  isFullscreen: false,
  quality: 'auto',
  showControls: true,
}

export const usePlayerStore = create<PlayerState>()((set) => ({
  ...initialState,

  setPlaying: (isPlaying) => set({ isPlaying }),
  setCurrentTime: (currentTime) => set({ currentTime }),
  setDuration: (duration) => set({ duration }),
  setVolume: (volume) => set({ volume, isMuted: volume === 0 }),
  setMuted: (isMuted) => set({ isMuted }),
  setFullscreen: (isFullscreen) => set({ isFullscreen }),
  setQuality: (quality) => set({ quality }),
  setShowControls: (showControls) => set({ showControls }),
  reset: () => set(initialState),
}))
