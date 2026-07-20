import { defineStore } from 'pinia'
import Cookies from 'js-cookie'
import { refreshTokenApi, logoutApi, type UserRole } from '@/api/auth'

export type { UserRole }

interface AuthState {
  accessToken: string
  refreshToken: string
  role: UserRole | ''
  userId: number | null
  username: string
  tenantId: number | null
  expiresAt: number // access token 过期时间戳（秒）
  /** refresh token 自动续期定时器 */
  _refreshTimer: ReturnType<typeof setTimeout> | null
}

export const useAuthStore = defineStore('auth', {
  state: (): AuthState => ({
    accessToken: '',
    refreshToken: '',
    role: '',
    userId: null,
    username: '',
    tenantId: null,
    expiresAt: 0,
    _refreshTimer: null
  }),
  getters: {
    // 兼容旧代码：token 字段
    token: (state) => state.accessToken,
    isLoggedIn: (state) => !!state.accessToken,
    homePath(): string {
      return `/${this.role}/dashboard`
    },
    /** access token 是否已过期 */
    isAccessTokenExpired: (state) => {
      if (!state.accessToken) return true
      if (!state.expiresAt) return false
      return Date.now() / 1000 > state.expiresAt - 60 // 提前 60s 续期
    }
  },
  actions: {
    setAuth(payload: {
      access_token: string
      refresh_token: string
      role: UserRole
      userId: number
      username: string
      tenantId?: number
      expires_at?: number
    }) {
      this.accessToken = payload.access_token
      this.refreshToken = payload.refresh_token
      this.role = payload.role
      this.userId = payload.userId
      this.username = payload.username
      this.tenantId = payload.tenantId ?? null
      this.expiresAt = payload.expires_at || Math.floor(Date.now() / 1000) + 7200

      // 同步写入 Cookie（供 SSR / nginx 鉴权使用，7 天过期）
      Cookies.set('keyauth_token', payload.access_token, { expires: 7, sameSite: 'lax' })
      Cookies.set('keyauth_role', payload.role, { expires: 7, sameSite: 'lax' })

      this.scheduleRefresh()
    },

    /** 安排 access token 过期前自动续期 */
    scheduleRefresh() {
      if (this._refreshTimer) {
        clearTimeout(this._refreshTimer)
        this._refreshTimer = null
      }
      if (!this.refreshToken || !this.expiresAt) return

      // 提前 5 分钟续期
      const refreshAt = (this.expiresAt - 300) * 1000
      const delay = refreshAt - Date.now()
      if (delay <= 0) {
        // 已临近过期，立即续期
        this.doRefresh().catch(() => {})
        return
      }
      this._refreshTimer = setTimeout(() => {
        this.doRefresh().catch(() => {
          // 续期失败：清空登录态
          this.logout()
        })
      }, delay)
    },

    /** 执行 refresh token 续期 */
    async doRefresh() {
      if (!this.refreshToken) return
      try {
        const resp = await refreshTokenApi(this.refreshToken)
        this.accessToken = resp.access_token
        this.refreshToken = resp.refresh_token
        this.expiresAt = resp.expires_at
        Cookies.set('keyauth_token', resp.access_token, { expires: 7, sameSite: 'lax' })
        this.scheduleRefresh()
      } catch (e) {
        // refresh 失败：清空登录态
        this.logout()
        throw e
      }
    },

    /** 登出：调用后端黑名单 + 清空本地 */
    async logout() {
      if (this._refreshTimer) {
        clearTimeout(this._refreshTimer)
        this._refreshTimer = null
      }
      if (this.accessToken && this.role && this.refreshToken) {
        try {
          await logoutApi(this.role, this.refreshToken)
        } catch {
          // 静默失败：本地态仍要清空
        }
      }
      this.$reset()
      Cookies.remove('keyauth_token')
      Cookies.remove('keyauth_role')
    }
  },
  persist: {
    key: 'keyauth-auth',
    storage: localStorage,
    // 不持久化定时器
    paths: ['accessToken', 'refreshToken', 'role', 'userId', 'username', 'tenantId', 'expiresAt']
  }
})
