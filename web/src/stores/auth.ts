import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { clearPageCache } from '@/utils/pageCache'
import type { User } from '@/types'

interface AuthState {
  token: string | null
  user: User | null
  isAuthenticated: boolean

  setAuth: (token: string, user: User) => void
  logout: () => void
  updateUser: (user: User) => void
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      isAuthenticated: false,

      setAuth: (token, user) =>
        set((current) => {
          if (current.user?.id !== user.id || current.user?.role !== user.role || current.user?.child_mode !== user.child_mode) clearPageCache()
          return { token, user, isAuthenticated: true }
        }),

      logout: () => {
        clearPageCache()
        set({ token: null, user: null, isAuthenticated: false })
      },

      updateUser: (user) => set((current) => {
        if (current.user?.id !== user.id || current.user?.role !== user.role || current.user?.child_mode !== user.child_mode) clearPageCache()
        return { user }
      }),
    }),
    {
      name: 'nowen-auth', // localStorage key
      partialize: (state) => ({
        token: state.token,
        user: state.user,
        isAuthenticated: state.isAuthenticated,
      }),
    }
  )
)
