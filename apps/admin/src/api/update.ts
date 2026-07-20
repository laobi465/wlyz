// 在线更新 API（v0.4.0）
// 对应后端路由：/api/v1/admin/update/*
// 包含：状态查询 / 手动触发 / 历史 / 回滚 / 单条详情 / 弹窗通知轻量轮询
import { request } from './http'

// ============== 类型定义 ==============

/** 更新状态（GET /admin/update/status） */
export interface UpdateStatus {
  current_commit: string
  is_locked: boolean
  auto_update: boolean
  branch: string
  success_count: number
  failed_count: number
  latest_log?: UpdateLog
}

/** 更新审计日志 */
export interface UpdateLog {
  id: number
  trigger_source: string // webhook / manual / rollback
  trigger_by: number
  trigger_ip: string
  commit_before: string
  commit_after: string
  branch: string
  status: string // pending / running / success / failed / rolled_back
  steps_json: string
  log_text: string
  error_message: string
  duration_ms: number
  rolled_back_from: number
  created_at: string
  updated_at: string
}

/** 弹窗通知轻量轮询响应（GET /admin/update/poll，v0.4.0） */
export interface UpdatePoll {
  enabled: boolean
  interval_seconds: number
  current_commit: string
  is_locked: boolean
  last_update_at: number | null // unix 时间戳
  last_status: string | null
  last_trigger: string | null
  last_commit: string | null
}

// ============== API ==============

/** 更新状态（GET /admin/update/status） */
export const updateStatusApi = () => {
  return request.get<UpdateStatus>('/admin/update/status')
}

/** 手动触发更新（POST /admin/update/trigger） */
export const triggerUpdateApi = (data: { branch?: string }) => {
  return request.post<{ log_id: number }>('/admin/update/trigger', data)
}

/** 更新历史（GET /admin/update/history） */
export const listUpdateHistoryApi = (params: {
  page?: number
  page_size?: number
  status?: string
  trigger_source?: string
}) => {
  return request.get<{ list: UpdateLog[]; total: number; page: number; page_size: number }>(
    '/admin/update/history',
    params
  )
}

/** 单条详情（GET /admin/update/logs/:id） */
export const getUpdateLogApi = (id: number) => {
  return request.get<UpdateLog>(`/admin/update/logs/${id}`)
}

/** 手动回滚（POST /admin/update/rollback） */
export const rollbackUpdateApi = (data: { log_id: number }) => {
  return request.post<{ log_id: number; status: string }>('/admin/update/rollback', data)
}

/** 弹窗通知轻量轮询（GET /admin/update/poll，v0.4.0） */
export const pollUpdateApi = () => {
  return request.get<UpdatePoll>('/admin/update/poll')
}
