import axios, { type AxiosInstance, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const apiBase = import.meta.env.VITE_API_BASE || '/api/v1'

const http: AxiosInstance = axios.create({
  baseURL: apiBase,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' }
})

// 请求拦截：注入 Bearer token
http.interceptors.request.use(
  (config) => {
    const auth = useAuthStore()
    if (auth.accessToken) {
      config.headers.Authorization = `Bearer ${auth.accessToken}`
    }
    return config
  },
  (err) => Promise.reject(err)
)

// 是否正在刷新 token（避免并发刷新）
let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

const subscribeTokenRefresh = (cb: (token: string) => void) => {
  refreshSubscribers.push(cb)
}

const onTokenRefreshed = (token: string) => {
  refreshSubscribers.forEach((cb) => cb(token))
  refreshSubscribers = []
}

// 响应拦截
http.interceptors.response.use(
  (resp) => {
    const data = resp.data
    if (data && typeof data === 'object' && 'code' in data) {
      if (data.code === 0 || data.code === 200) {
        return data.data ?? data
      }
      // 业务错误
      ElMessage.error(data.message || `请求失败：${data.code}`)
      return Promise.reject(new Error(data.message || 'biz error'))
    }
    return data
  },
  async (err) => {
    const originalRequest = err.config as InternalAxiosRequestConfig & { _retry?: boolean }
    const status = err.response?.status

    if (status === 401) {
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
  if (location.pathname !== '/login') {
    ElMessage.error('登录已过期，请重新登录')
    location.href = '/login?redirect=' + encodeURIComponent(location.pathname)
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
