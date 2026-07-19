import { defineStore } from 'pinia'
import Cookies from 'js-cookie'

export type UserRole = 'admin' | 'tenant' | 'agent'

interface AuthState {
  token: string
  role: UserRole | ''
  userId: number | null
  username: string
  tenantId: number | null
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    token: '',
    role: '',
    userId: null,
    username: '',
    tenantId: null
  }),
  getters: {
    isLoggedIn: (state) => !!state.token,
    homePath(): string {
      return `/${this.role}/dashboard`
    }
  },
  actions: {
    setAuth(payload: { token: string; role: UserRole; userId: number; username: string; tenantId?: number }) {
      this.token = payload.token
      this.role = payload.role
      this.userId = payload.userId
      this.username = payload.username
      this.tenantId = payload.tenantId ?? null
      // 同步写入 Cookie（供 SSR / nginx 鉴权使用，7 天过期）
      Cookies.set('keyauth_token', payload.token, { expires: 7, sameSite: 'lax' })
      Cookies.set('keyauth_role', payload.role, { expires: 7, sameSite: 'lax' })
    },
    logout() {
      this.$reset()
      Cookies.remove('keyauth_token')
      Cookies.remove('keyauth_role')
    }
  },
  persist: {
    key: 'keyauth-auth',
    storage: localStorage
  }
})
