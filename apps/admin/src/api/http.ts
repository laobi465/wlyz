import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const apiBase = import.meta.env.VITE_API_BASE || '/api/v1'

const http: AxiosInstance = axios.create({
  baseURL: apiBase,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' }
})

// 请求拦截
http.interceptors.request.use(
  (config) => {
    const auth = useAuthStore()
    if (auth.token) {
      config.headers.Authorization = `Bearer ${auth.token}`
    }
    return config
  },
  (err) => Promise.reject(err)
)

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
  (err) => {
    if (err.response?.status === 401) {
      const auth = useAuthStore()
      auth.logout()
      ElMessage.error('登录已过期，请重新登录')
      // 跳转登录
      if (location.pathname !== '/login') {
        location.href = '/login?redirect=' + encodeURIComponent(location.pathname)
      }
    } else if (err.response?.status === 403) {
      ElMessage.error('无权限访问')
    } else {
      ElMessage.error(err.message || '网络异常')
    }
    return Promise.reject(err)
  }
)

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
