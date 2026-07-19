import { request } from './http'

// 系统配置 API（铁律 05 核心实现）
export const getSysConfig = (keys: string[]) => {
  return request.post<Record<string, any>>('/admin/config/batch', { keys })
}

export const listSysConfig = (params: { group?: string; keyword?: string; page?: number; size?: number }) => {
  return request.get('/admin/config', params)
}

export const updateSysConfig = (key: string, data: { value: string; name?: string; group?: string; remark?: string }) => {
  return request.put(`/admin/config/${key}`, data)
}

export const resetSysConfig = (key: string) => {
  return request.post(`/admin/config/${key}/reset`)
}
