// 应用管理 API
// 对应后端路由：/api/v1/tenant/apps/*
import { request } from './http'

export interface App {
  id: number
  tenant_id: number
  app_key: string
  name: string
  description: string
  icon: string
  status: 'active' | 'disabled'
  max_devices: number
  heartbeat_interval: number
  heartbeat_timeout: number
  offline_grace: number
  unbind_deduct_seconds: number
  agent_commission_mode: 'percentage' | 'diff'
  created_at: string
  updated_at: string
}

export interface AppListResp {
  list: App[]
  total: number
}

export const listAppsApi = (params: { page?: number; page_size?: number; keyword?: string; status?: string }) => {
  return request.get<AppListResp>('/tenant/apps', params)
}

export const getAppApi = (id: number) => {
  return request.get<App>(`/tenant/apps/${id}`)
}

export const createAppApi = (data: {
  name: string
  description?: string
  max_devices?: number
  heartbeat_interval?: number
  heartbeat_timeout?: number
  offline_grace?: number
  unbind_deduct_seconds?: number
  agent_commission_mode?: 'percentage' | 'diff'
}) => {
  return request.post<App>('/tenant/apps', data)
}

export const updateAppApi = (id: number, data: Partial<App>) => {
  return request.put<App>(`/tenant/apps/${id}`, data)
}

export const deleteAppApi = (id: number) => {
  return request.delete(`/tenant/apps/${id}`)
}

/** 重置应用密钥（AppKey/AppSecret/SignSecret 全部轮换） */
export const resetAppKeyApi = (id: number, data?: { reset_type?: 'all' | 'app_key' | 'app_secret' | 'sign_secret' }) => {
  return request.post<{ app_key: string; app_secret?: string; sign_secret?: string }>(
    `/tenant/apps/${id}/reset_key`,
    data || {}
  )
}
