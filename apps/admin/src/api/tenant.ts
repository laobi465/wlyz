// 开发者控制台 API（除 apps/card_types/cards 已有专用模块外）
// 对应后端路由：/api/v1/tenant/*
// 注：除 /tenant/apps、/tenant/card_types、/tenant/cards 已实现外，
//     其余业务接口当前为 501 占位（v0.3.0 交付），前端需优雅降级（铁律 06 待核实）。
import { request } from './http'

// ============== 工作台 ==============

export interface TenantDashboardData {
  app_total: number
  app_active: number
  card_total: number
  card_active: number
  card_used: number
  device_online: number
  device_total: number
  order_today: number
  revenue_today: number
  revenue_month: number
  settlement_pending: number
  settlement_amount: number
  agent_total: number
  /** 待核实：v0.3.0 后端补全 */
  revenue_trend?: Array<{ date: string; amount: number }>
  /** 待核实 */
  recent_orders?: Array<{ id: number; order_no: string; amount: number; status: string; created_at: string }>
  /** 待核实 */
  top_apps?: Array<{ id: number; name: string; card_count: number; revenue: number }>
}

/** 开发者工作台（GET /tenant/dashboard）—— 当前 501，待 v0.3.0 */
export const tenantDashboardApi = () => {
  return request.get<TenantDashboardData>('/tenant/dashboard')
}

// ============== 设备管理 ==============

export interface TenantDevice {
  id: number
  app_id: number
  app_name: string
  card_id: number
  card_key: string
  device_id: string
  device_name: string
  ip: string
  location: string
  user_agent: string
  heartbeat_at: string
  is_online: boolean
  created_at: string
}

/** 设备列表（GET /tenant/devices）—— 待核实，v0.3.0 */
export const listTenantDevicesApi = (params: { page?: number; page_size?: number; app_id?: number; keyword?: string; online?: boolean }) => {
  return request.get<{ list: TenantDevice[]; total: number }>('/tenant/devices', params)
}

/** 踢设备下线（POST /tenant/devices/:id/kick）—— 待核实 */
export const kickTenantDeviceApi = (id: number) => {
  return request.post(`/tenant/devices/${id}/kick`, {})
}

// ============== 订单管理 ==============

export type TenantOrderStatus = 'paid' | 'pending' | 'closed' | 'refunded'
export type TenantOrderChannel = 'h5' | 'agent' | 'manual'

export interface TenantOrder {
  id: number
  order_no: string
  app_id: number
  app_name: string
  card_type_id: number
  card_type_name: string
  buyer_username: string
  agent_username?: string
  quantity: number
  unit_price: number
  total_amount: number
  commission_amount: number
  net_amount: number
  pay_status: TenantOrderStatus
  pay_channel: string
  channel: TenantOrderChannel
  paid_at: string | null
  created_at: string
}

/** 订单列表（GET /tenant/orders）—— 待核实，v0.3.0 */
export const listTenantOrdersApi = (params: { page?: number; page_size?: number; app_id?: number; status?: TenantOrderStatus; channel?: TenantOrderChannel; start_date?: string; end_date?: string; keyword?: string }) => {
  return request.get<{ list: TenantOrder[]; total: number }>('/tenant/orders', params)
}

// ============== 云变量 ==============

export interface TenantCloudVar {
  id: number
  app_id: number
  app_name: string
  key: string
  value: string
  value_type: 'string' | 'number' | 'json' | 'bool'
  description: string
  read_only: boolean
  updated_at: string
  created_at: string
}

/** 云变量列表（GET /tenant/cloud_vars）—— 待核实，v0.3.0 */
export const listTenantCloudVarsApi = (params: { page?: number; page_size?: number; app_id?: number; keyword?: string }) => {
  return request.get<{ list: TenantCloudVar[]; total: number }>('/tenant/cloud_vars', params)
}

/** 创建/更新云变量（POST /tenant/cloud_vars）—— 待核实 */
export const upsertTenantCloudVarApi = (data: { app_id: number; key: string; value: string; value_type?: string; description?: string; read_only?: boolean }) => {
  return request.post<TenantCloudVar>('/tenant/cloud_vars', data)
}

/** 删除云变量（DELETE /tenant/cloud_vars/:id）—— 待核实 */
export const deleteTenantCloudVarApi = (id: number) => {
  return request.delete(`/tenant/cloud_vars/${id}`)
}

// ============== 版本管理 ==============

export type VersionChannel = 'stable' | 'beta' | 'alpha'

export interface TenantVersion {
  id: number
  app_id: number
  app_name: string
  version: string
  channel: VersionChannel
  download_url: string
  update_log: string
  min_version: string
  force_update: boolean
  published: boolean
  published_at: string | null
  created_at: string
}

/** 版本列表（GET /tenant/versions）—— 待核实，v0.3.0 */
export const listTenantVersionsApi = (params: { page?: number; page_size?: number; app_id?: number; channel?: VersionChannel }) => {
  return request.get<{ list: TenantVersion[]; total: number }>('/tenant/versions', params)
}

/** 创建版本（POST /tenant/versions）—— 待核实 */
export const createTenantVersionApi = (data: { app_id: number; version: string; channel?: VersionChannel; download_url: string; update_log?: string; min_version?: string; force_update?: boolean; published?: boolean }) => {
  return request.post<TenantVersion>('/tenant/versions', data)
}

/** 删除版本（DELETE /tenant/versions/:id）—— 待核实 */
export const deleteTenantVersionApi = (id: number) => {
  return request.delete(`/tenant/versions/${id}`)
}

// ============== 代理管理（开发者维度） ==============

export interface TenantAgent {
  id: number
  username: string
  real_name: string
  phone: string
  balance: number
  frozen_balance: number
  total_commission: number
  total_withdraw: number
  status: 'active' | 'disabled' | 'pending'
  commission_mode: 'percentage' | 'diff'
  commission_rate: number
  inviter_username?: string
  created_at: string
  last_active_at?: string
}

/** 开发者代理列表（GET /tenant/agents）—— 当前 501，待 v0.3.0 */
export const listTenantAgentsApi = (params: { page?: number; page_size?: number; keyword?: string; status?: string }) => {
  return request.get<{ list: TenantAgent[]; total: number }>('/tenant/agents', params)
}

/** 更新代理（PUT /tenant/agents/:id）—— 待核实 */
export const updateTenantAgentApi = (id: number, data: Partial<Pick<TenantAgent, 'status' | 'commission_mode' | 'commission_rate'>>) => {
  return request.put<TenantAgent>(`/tenant/agents/${id}`, data)
}

// ============== 邀请码 ==============

export interface TenantInviteCode {
  id: number
  code: string
  status: 'unused' | 'used' | 'expired' | 'disabled'
  used_by_username?: string
  used_at: string | null
  expire_at: string | null
  remark: string
  created_at: string
}

/** 邀请码列表（GET /tenant/invite_codes）—— 待核实，v0.3.0 */
export const listTenantInviteCodesApi = (params: { page?: number; page_size?: number; status?: string }) => {
  return request.get<{ list: TenantInviteCode[]; total: number }>('/tenant/invite_codes', params)
}

/** 生成邀请码（POST /tenant/agents/invite_codes）—— 当前 501 */
export const genTenantInviteCodeApi = (data: { count?: number; expire_days?: number; remark?: string }) => {
  return request.post<{ codes: TenantInviteCode[] }>('/tenant/agents/invite_codes', data)
}

/** 禁用邀请码（POST /tenant/invite_codes/:id/disable）—— 待核实 */
export const disableTenantInviteCodeApi = (id: number) => {
  return request.post(`/tenant/invite_codes/${id}/disable`, {})
}

// ============== 支付配置 ==============

export interface TenantPayConfig {
  id: number
  tenant_id: number
  channel: 'epay' | 'alipay' | 'wechat' | 'stripe'
  config: {
    pid?: string
    key?: string
    api_url?: string
    notify_url?: string
    return_url?: string
  }
  status: 'active' | 'disabled'
  created_at: string
  updated_at: string
}

/** 开发者支付配置列表（GET /tenant/pay_config）—— 待核实，v0.3.0 */
export const listTenantPayConfigApi = () => {
  return request.get<{ list: TenantPayConfig[] }>('/tenant/pay_config')
}

/** 保存开发者支付配置（POST /tenant/pay_config）—— 待核实 */
export const saveTenantPayConfigApi = (data: { channel: string; config: Record<string, any>; status?: string }) => {
  return request.post<TenantPayConfig>('/tenant/pay_config', data)
}

/** 测试开发者支付配置（POST /tenant/pay_config/:id/test）—— 待核实 */
export const testTenantPayConfigApi = (id: number) => {
  return request.post<{ success: boolean; message: string }>(`/tenant/pay_config/${id}/test`, {})
}

// ============== 公告（开发者发布给代理/H5） ==============

export interface TenantNotice {
  id: number
  type: 'tenant' | 'agent' | 'h5'
  title: string
  content: string
  status: 'draft' | 'published' | 'archived'
  pinned: boolean
  publish_at: string
  expire_at: string | null
  created_at: string
}

/** 开发者公告列表（GET /tenant/notices）—— 待核实，v0.3.0 */
export const listTenantNoticesApi = (params: { page?: number; page_size?: number; type?: string; status?: string }) => {
  return request.get<{ list: TenantNotice[]; total: number }>('/tenant/notices', params)
}

/** 创建公告（POST /tenant/notices）—— 待核实 */
export const createTenantNoticeApi = (data: { type: string; title: string; content: string; status?: string; pinned?: boolean; publish_at?: string; expire_at?: string }) => {
  return request.post<TenantNotice>('/tenant/notices', data)
}
