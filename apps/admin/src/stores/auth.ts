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
  /**
   * v0.6.7 P0 修复：是否正在执行 doRefresh
   * 防止 scheduleRefresh 在 delay <= 0 路径下与 doRefresh 形成无限异步递归
   * （管理员登录后死机 bug 的根因）
   */
  _refreshing: boolean
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
    _refreshTimer: null,
    _refreshing: false
  }),
  getters: {
    // 兼容旧代码：token 字段
    token: (state) => state.accessToken,
    isLoggedIn: (state) => !!state.accessToken,
    homePath(): string {
      // v0.6.5 修复：role 为空时兜底回登录页，避免跳转到 '//dashboard' → 404
      // 触发场景：localStorage 持久化数据损坏 / 字段缺失 / 用户手动篡改
      if (!this.role) return '/login'
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
      // P1-03: 生产环境（HTTPS）下补 secure，避免中间人嗅探
      Cookies.set('keyauth_token', payload.access_token, {
        expires: 7,
        sameSite: 'lax',
        secure: import.meta.env.PROD
      })
      Cookies.set('keyauth_role', payload.role, {
        expires: 7,
        sameSite: 'lax',
        secure: import.meta.env.PROD
      })

      this.scheduleRefresh()
    },

    /**
     * 安排 access token 过期前自动续期
     * v0.6.7 P0 修复：避免在 delay <= 0 路径下与 doRefresh 形成无限异步递归
     * - 增加 _refreshing 并发锁：正在刷新则不重复触发
     * - 最小延迟保护：兜底 30s，避免 sys_config 配置异常导致死循环
     * - expires_at 合法性校验：后端返回异常值时不更新 / 不重排
     */
    scheduleRefresh() {
      if (this._refreshTimer) {
        clearTimeout(this._refreshTimer)
        this._refreshTimer = null
      }
      if (!this.refreshToken || !this.expiresAt) return

      // 提前 5 分钟续期
      const refreshAt = (this.expiresAt - 300) * 1000
      let delay = refreshAt - Date.now()
      if (delay <= 0) {
        // 已临近过期，立即续期 —— 但必须检查 _refreshing 防止无限递归
        if (this._refreshing) return // 正在刷新中，由 doRefresh 完成后会重新调度
        this.doRefresh().catch(() => {})
        return
      }
      // v0.6.7 P0 修复：最小延迟保护（兜底 30s）
      // 触发场景：sys_config 中 jwt.access_ttl_seconds 被设为异常小值（如 60s）
      // 此时 expiresAt - 300 会变成负数导致 delay 极大，但若 ttl 接近 300 临界值
      // 也应避免极短 delay 触发高频刷新
      if (delay < 30_000) delay = 30_000
      this._refreshTimer = setTimeout(() => {
        this.doRefresh().catch(() => {
          // 续期失败：清空登录态
          this.logout()
        })
      }, delay)
    },

    /**
     * 执行 refresh token 续期
     * v0.6.7 P0 修复：
     * - _refreshing 并发锁：避免与 scheduleRefresh 的 delay <= 0 路径形成递归
     * - 新 expires_at 合法性校验：必须 > now + 60s，否则不更新、不重排
     *   防御后端异常返回 0 / 旧时间戳导致死循环
     * - try/finally 保证 _refreshing 一定释放
     */
    async doRefresh() {
      if (!this.refreshToken) return
      // 并发锁：正在刷新则直接返回（http 拦截器、scheduleRefresh 等并发调用安全）
      if (this._refreshing) return
      this._refreshing = true
      try {
        const resp = await refreshTokenApi(this.refreshToken)
        // v0.6.7 P0 修复：校验新 expires_at 合法性（绝对 Unix 秒，必须 > now + 60s）
        const nowSec = Math.floor(Date.now() / 1000)
        const newExpiresAt = resp.expires_at || 0
        if (!newExpiresAt || newExpiresAt <= nowSec + 60) {
          // 后端返回异常 expires_at：不更新本地态，不重排定时器
          // 下一次 401 拦截器或定时器触发时由 _refreshing 锁自然阻塞
          console.warn('[auth] refresh 返回异常 expires_at，跳过更新', newExpiresAt)
          return
        }
        this.accessToken = resp.access_token
        this.refreshToken = resp.refresh_token
        this.expiresAt = newExpiresAt
        // P1-03: 生产环境（HTTPS）下补 secure，避免中间人嗅探
        Cookies.set('keyauth_token', resp.access_token, {
          expires: 7,
          sameSite: 'lax',
          secure: import.meta.env.PROD
        })
        this.scheduleRefresh()
      } catch (e) {
        // refresh 失败：清空登录态
        this.logout()
        throw e
      } finally {
        this._refreshing = false
      }
    },

    /** 登出：调用后端黑名单 + 清空本地 */
    async logout() {
      if (this._refreshTimer) {
        clearTimeout(this._refreshTimer)
        this._refreshTimer = null
      }
      // v0.6.7 P0 修复：登出时重置并发锁，避免下次登录复用 stale 状态
      this._refreshing = false
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
