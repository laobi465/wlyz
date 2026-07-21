import axios, { type AxiosInstance, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useEndUserStore } from '@/stores/enduser'
import { endUserRefreshApi } from './enduser'

const apiBase = import.meta.env.VITE_API_BASE || '/api/v1'

const http: AxiosInstance = axios.create({
  baseURL: apiBase,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' }
})

// H5 终端用户请求路径前缀（与三角色鉴权隔离）
const isH5Request = (url?: string): boolean => {
  if (!url) return false
  return url.startsWith('/h5/') || url.startsWith('/public/enduser/')
}

// 请求拦截：注入 Bearer token（H5 端用 enduser store，其余用三角色 auth store）
http.interceptors.request.use(
  (config) => {
    if (isH5Request(config.url)) {
      const endUserStore = useEndUserStore()
      endUserStore.restore()
      if (endUserStore.accessToken) {
        config.headers.Authorization = `Bearer ${endUserStore.accessToken}`
      }
    } else {
      const auth = useAuthStore()
      if (auth.accessToken) {
        config.headers.Authorization = `Bearer ${auth.accessToken}`
      }
    }
    return config
  },
  (err) => Promise.reject(err)
)

// 是否正在刷新 token（避免并发刷新）—— 三角色（admin/tenant/agent）专用
let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

const subscribeTokenRefresh = (cb: (token: string) => void) => {
  refreshSubscribers.push(cb)
}

const onTokenRefreshed = (token: string) => {
  refreshSubscribers.forEach((cb) => cb(token))
  refreshSubscribers = []
}

// P1-07: H5 终端用户独立的刷新队列（与三角色 token 隔离，避免并发 refresh 误登出）
let isH5Refreshing = false
let h5RefreshSubscribers: Array<(token: string) => void> = []

const subscribeH5TokenRefresh = (cb: (token: string) => void) => {
  h5RefreshSubscribers.push(cb)
}

const onH5TokenRefreshed = (token: string) => {
  h5RefreshSubscribers.forEach((cb) => cb(token))
  h5RefreshSubscribers = []
}

// 响应拦截
http.interceptors.response.use(
  (resp) => {
    const data = resp.data
    if (data && typeof data === 'object' && 'code' in data) {
      if (data.code === 0 || data.code === 200) {
        return data.data ?? data
      }
      // 业务错误：构造带 code 的错误对象，便于调用方按 code 分支处理
      // （例如登录时 1007 = 需要 2FA，前端据此显示 TOTP 输入框）
      const bizErr: any = new Error(data.message || 'biz error')
      bizErr.code = data.code
      bizErr.message = data.message
      // 2FA 相关错误码不弹全局错误提示（由调用方处理）
      if (data.code !== 1007) {
        ElMessage.error(data.message || `请求失败：${data.code}`)
      }
      return Promise.reject(bizErr)
    }
    return data
  },
  async (err) => {
    const originalRequest = err.config as InternalAxiosRequestConfig & { _retry?: boolean }
    const status = err.response?.status

    // 登录/注册接口的 401/403：直接把后端 code+message 抛给调用方，不触发 refresh / 登出
    // （例如 1007 = 需要 2FA，1008 = 未绑定 2FA，1xxx = 账号锁定等）
    if ((status === 401 || status === 403) && /\/public\/auth\/(admin|tenant|agent)\/(login|register)/.test(originalRequest.url || '')) {
      const respData = err.response?.data
      const bizErr: any = new Error(respData?.message || '登录失败')
      bizErr.code = respData?.code
      bizErr.message = respData?.message
      bizErr.status = status
      return Promise.reject(bizErr)
    }

    if (status === 401) {
      // H5 终端用户请求：使用 enduser store 处理续期/登出
      if (isH5Request(originalRequest.url)) {
        const endUserStore = useEndUserStore()
        endUserStore.restore()

        // refresh 接口本身 401：直接清空登录态
        if (originalRequest.url?.includes('/public/enduser/refresh')) {
          endUserStore.clear()
          redirectToH5Login()
          return Promise.reject(err)
        }

        // 已重试过：登出
        if (originalRequest._retry) {
          endUserStore.clear()
          redirectToH5Login()
          return Promise.reject(err)
        }

        // 没有 refresh token：直接登出
        if (!endUserStore.refreshToken) {
          endUserStore.clear()
          redirectToH5Login()
          return Promise.reject(err)
        }

        // 标记重试
        originalRequest._retry = true

        // P1-07: 正在刷新则排队等待，避免并发 refresh 误登出
        if (isH5Refreshing) {
          return new Promise((resolve, reject) => {
            subscribeH5TokenRefresh((newToken: string) => {
              if (!newToken) {
                reject(err)
                return
              }
              originalRequest.headers.Authorization = `Bearer ${newToken}`
              resolve(http(originalRequest))
            })
          })
        }

        // 开始刷新
        isH5Refreshing = true
        try {
          const resp = await endUserRefreshApi(endUserStore.refreshToken)
          // 持久化新的 token（保留原 user 信息和 appKey）
          // P0 高危 10：后端返回 expires_in（相对秒数），store 内部存绝对时间戳（ms）
          endUserStore.setLogin({
            access_token: resp.access_token,
            refresh_token: resp.refresh_token,
            expires_at: Date.now() + resp.expires_in * 1000,
            user: (endUserStore.user ?? null) as any
          })
          const newToken = resp.access_token
          onH5TokenRefreshed(newToken)
          originalRequest.headers.Authorization = `Bearer ${newToken}`
          return http(originalRequest)
        } catch (refreshErr) {
          onH5TokenRefreshed('')
          endUserStore.clear()
          redirectToH5Login()
          return Promise.reject(refreshErr)
        } finally {
          isH5Refreshing = false
        }
      }

      // 三角色请求：走原有 refresh 流程
      const auth = useAuthStore()

      // 如果是 refresh 接口本身 401，直接登出
      if (originalRequest.url?.includes('/auth/refresh')) {
        auth.logout()
        redirectToLogin()
        return Promise.reject(err)
      }

      // 已重试过：登出
      if (originalRequest._retry) {
        auth.logout()
        redirectToLogin()
        return Promise.reject(err)
      }

      // 没有 refresh token：直接登出
      if (!auth.refreshToken) {
        auth.logout()
        redirectToLogin()
        return Promise.reject(err)
      }

      // 标记重试
      originalRequest._retry = true

      // 如果正在刷新，排队等待
      if (isRefreshing) {
        return new Promise((resolve, reject) => {
          subscribeTokenRefresh((newToken: string) => {
            if (!newToken) {
              reject(err)
              return
            }
            originalRequest.headers.Authorization = `Bearer ${newToken}`
            resolve(http(originalRequest))
          })
        })
      }

      // 开始刷新
      isRefreshing = true
      try {
        await auth.doRefresh()
        const newToken = auth.accessToken
        // v0.6.7 P0 修复：doRefresh 成功但 token 为空（异常 expires_at 被跳过更新）
        // 直接登出避免无效重试
        if (!newToken) {
          onTokenRefreshed('')
          auth.logout()
          redirectToLogin()
          return Promise.reject(err)
        }
        onTokenRefreshed(newToken)
        originalRequest.headers.Authorization = `Bearer ${newToken}`
        return http(originalRequest)
      } catch (refreshErr) {
        onTokenRefreshed('')
        auth.logout()
        redirectToLogin()
        return Promise.reject(refreshErr)
      } finally {
        isRefreshing = false
      }
    }

    if (status === 403) {
      ElMessage.error('无权限访问')
    } else if (status && status >= 500) {
      ElMessage.error('服务器异常，请稍后重试')
    } else if (err.message?.includes('timeout')) {
      ElMessage.error('请求超时')
    } else if (err.message === 'Network Error') {
      ElMessage.error('网络异常')
    } else if (!err.response) {
      ElMessage.error(err.message || '网络异常')
    }
    return Promise.reject(err)
  }
)

function redirectToLogin() {
  // v0.9.0：管理员（role=admin）登录过期跳 /admin/login，其他角色跳 /login
  // 通过 localStorage 中持久化的 keyauth-auth.role 判断（避免依赖 Pinia store 状态）
  let adminLogin = false
  try {
    const raw = localStorage.getItem('keyauth-auth')
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed?.role === 'admin') adminLogin = true
    }
  } catch { /* ignore */ }

  // 当前已在登录页则不再跳转，避免循环
  const targetPath = adminLogin ? '/admin/login' : '/login'
  if (location.pathname === targetPath) return
  // 如果当前路径以 /admin 开头，强制跳 /admin/login
  const isAdminPath = location.pathname.startsWith('/admin')
  const finalPath = isAdminPath ? '/admin/login' : targetPath
  if (location.pathname === finalPath) return
  ElMessage.error('登录已过期，请重新登录')
  location.href = finalPath + '?redirect=' + encodeURIComponent(location.pathname)
}

function redirectToH5Login() {
  if (!location.pathname.startsWith('/h5/login')) {
    ElMessage.error('登录已过期，请重新登录')
    location.href = '/h5/login?redirect=' + encodeURIComponent(location.pathname)
  }
}

export default http

// 通用请求方法
export const request = {
  get<T = any>(url: string, params?: any, config?: AxiosRequestConfig) {
    return http.get<any, T>(url, { params, ...config })
  },
  post<T = any>(url: string, data?: any, config?: AxiosRequestConfig) {
    return http.post<any, T>(url, data, config)
  },
  put<T = any>(url: string, data?: any, config?: AxiosRequestConfig) {
    return http.put<any, T>(url, data, config)
  },
  delete<T = any>(url: string, params?: any, config?: AxiosRequestConfig) {
    return http.delete<any, T>(url, { params, ...config })
  }
}

// ============== 安装向导（v0.3.6，无需 token） ==============
// 铁律 04：直接调 http 实例，绕过 request 拦截器的 token 注入

export interface InstallStatus {
  installed: boolean
  admin_name: string
  domain: string
  server_time: string
}

export interface InstallPayload {
  admin_username: string
  admin_password: string
  admin_email?: string
  admin_phone?: string
  platform_domain?: string
  platform_name?: string
  notify_email?: string
  agent_register_fee?: string
  platform_commission?: string
}

export interface InstallResult {
  installed: boolean
  admin_name: string
  installed_at: string
  message: string
}

export const installStatusApi = () => {
  return http.get<any, InstallStatus>('/install/status')
}

export const installApi = (data: InstallPayload) => {
  return http.post<any, InstallResult>('/install', data)
}
