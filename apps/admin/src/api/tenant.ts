// 开发者控制台 API（除 apps/card_types/cards 已有专用模块外）
// 对应后端路由：/api/v1/tenant/*
// v0.3.1 已交付：dashboard / devices / orders / cloud_vars / versions / agents / invite_codes / pay_config / notices
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
  revenue_trend?: Array<{ date: string; amount: number }>
  recent_orders?: Array<{ id: number; order_no: string; amount: number; status: string; created_at: string }>
  top_apps?: Array<{ id: number; name: string; card_count: number; revenue: number }>
}

/** 开发者工作台（GET /tenant/dashboard）—— v0.3.1 已实现 */
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

/** 设备列表（GET /tenant/devices）—— v0.3.1 已实现 */
export const listTenantDevicesApi = (params: { page?: number; page_size?: number; app_id?: number; keyword?: string; online?: boolean }) => {
  return request.get<{ list: TenantDevice[]; total: number }>('/tenant/devices', params)
}

/** 踢设备下线（POST /tenant/devices/:id/kick）—— v0.3.1 已实现 */
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

/** 订单列表（GET /tenant/orders）—— v0.3.1 已实现 */
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

/** 云变量列表（GET /tenant/cloud_vars）—— v0.3.1 已实现 */
export const listTenantCloudVarsApi = (params: { page?: number; page_size?: number; app_id?: number; keyword?: string }) => {
  return request.get<{ list: TenantCloudVar[]; total: number }>('/tenant/cloud_vars', params)
}

/** 创建/更新云变量（POST /tenant/cloud_vars）—— v0.3.1 已实现 */
export const upsertTenantCloudVarApi = (data: { app_id: number; key: string; value: string; value_type?: string; description?: string; read_only?: boolean }) => {
  return request.post<TenantCloudVar>('/tenant/cloud_vars', data)
}

/** 删除云变量（DELETE /tenant/cloud_vars/:id）—— v0.3.1 已实现 */
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

/** 版本列表（GET /tenant/versions）—— v0.3.1 已实现 */
export const listTenantVersionsApi = (params: { page?: number; page_size?: number; app_id?: number; channel?: VersionChannel }) => {
  return request.get<{ list: TenantVersion[]; total: number }>('/tenant/versions', params)
}

/** 创建版本（POST /tenant/versions）—— v0.3.1 已实现 */
export const createTenantVersionApi = (data: { app_id: number; version: string; channel?: VersionChannel; download_url: string; update_log?: string; min_version?: string; force_update?: boolean; published?: boolean }) => {
  return request.post<TenantVersion>('/tenant/versions', data)
}

/** 删除版本（DELETE /tenant/versions/:id）—— v0.3.1 已实现 */
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

/** 开发者代理列表（GET /tenant/agents）—— v0.3.1 已实现 */
export const listTenantAgentsApi = (params: { page?: number; page_size?: number; keyword?: string; status?: string }) => {
  return request.get<{ list: TenantAgent[]; total: number }>('/tenant/agents', params)
}

/** 更新代理（PUT /tenant/agents/:id）—— v0.3.1 已实现 */
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

/** 邀请码列表（GET /tenant/invite_codes）—— v0.3.1 已实现 */
export const listTenantInviteCodesApi = (params: { page?: number; page_size?: number; status?: string }) => {
  return request.get<{ list: TenantInviteCode[]; total: number }>('/tenant/invite_codes', params)
}

/** 生成邀请码（POST /tenant/agents/invite_codes）—— v0.3.1 已实现 */
export const genTenantInviteCodeApi = (data: { count?: number; expire_days?: number; remark?: string }) => {
  return request.post<{ codes: TenantInviteCode[] }>('/tenant/agents/invite_codes', data)
}

/** 禁用邀请码（POST /tenant/invite_codes/:id/disable）—— v0.3.1 已实现 */
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

/** 开发者支付配置列表（GET /tenant/pay_config）—— v0.3.1 已实现 */
export const listTenantPayConfigApi = () => {
  return request.get<{ list: TenantPayConfig[] }>('/tenant/pay_config')
}

/** 保存开发者支付配置（POST /tenant/pay_config）—— v0.3.1 已实现 */
export const saveTenantPayConfigApi = (data: { channel: string; config: Record<string, any>; status?: string }) => {
  return request.post<TenantPayConfig>('/tenant/pay_config', data)
}

/** 测试开发者支付配置（POST /tenant/pay_config/:id/test）—— v0.3.1 已实现 */
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

/** 开发者公告列表（GET /tenant/notices）—— v0.3.1 已实现 */
export const listTenantNoticesApi = (params: { page?: number; page_size?: number; type?: string; status?: string }) => {
  return request.get<{ list: TenantNotice[]; total: number }>('/tenant/notices', params)
}

/** 创建公告（POST /tenant/notices）—— v0.3.1 已实现 */
export const createTenantNoticeApi = (data: { type: string; title: string; content: string; status?: string; pinned?: boolean; publish_at?: string; expire_at?: string }) => {
  return request.post<TenantNotice>('/tenant/notices', data)
}

/** 更新公告（PUT /tenant/notices/:id）—— v0.3.1 已实现 */
export const updateTenantNoticeApi = (id: number, data: Partial<TenantNotice>) => {
  return request.put<TenantNotice>(`/tenant/notices/${id}`, data)
}

/** 删除公告（DELETE /tenant/notices/:id）—— v0.3.1 已实现 */
export const deleteTenantNoticeApi = (id: number) => {
  return request.delete(`/tenant/notices/${id}`)
}

// ============== 财务审核：代理充值 / 提现审核（v0.3.2） ==============

/** 充值申请记录（联表 agent 取 username/phone） */
export interface TenantRechargeRequest {
  id: number
  agent_id: number
  tenant_id: number
  type: 'recharge'
  amount: number
  balance_after: number
  pay_method: string
  pay_voucher: string
  status: 'pending' | 'settled' | 'rejected'
  remark: string
  created_at: string
  updated_at: string
  agent_username: string
  agent_phone: string
}

/** 提现申请记录（联表 agent 取 username/phone） */
export interface TenantWithdrawal {
  id: number
  agent_id: number
  tenant_id: number
  amount: number
  pay_method: 'alipay' | 'wechat' | 'bank'
  pay_account: string
  status: 'pending' | 'approved' | 'rejected' | 'paid' | 'failed'
  audit_remark: string
  pay_trade_no: string
  paid_at: string | null
  audited_by: number | null
  created_at: string
  updated_at: string
  agent_username: string
  agent_phone: string
}

/** 充值申请列表（GET /tenant/recharge_requests）—— v0.3.2 已实现 */
export const listTenantRechargeRequestsApi = (params: {
  page?: number
  page_size?: number
  status?: string
  agent_id?: number
  keyword?: string
}) => {
  return request.get<{ list: TenantRechargeRequest[]; total: number }>('/tenant/recharge_requests', params)
}

/** 充值审核通过（POST /tenant/recharge_requests/:id/approve）—— v0.3.2 已实现 */
export const approveTenantRechargeApi = (id: number, data: { actual_amount?: number; remark?: string }) => {
  return request.post<{
    id: number
    status: 'settled'
    actual_amount: number
    balance_after: number
  }>(`/tenant/recharge_requests/${id}/approve`, data)
}

/** 充值审核驳回（POST /tenant/recharge_requests/:id/reject）—— v0.3.2 已实现 */
export const rejectTenantRechargeApi = (id: number, data: { reason: string }) => {
  return request.post<{ id: number; status: 'rejected'; reason: string }>(`/tenant/recharge_requests/${id}/reject`, data)
}

/** 提现申请列表（GET /tenant/withdrawals）—— v0.3.2 已实现 */
export const listTenantWithdrawalsApi = (params: {
  page?: number
  page_size?: number
  status?: string
  agent_id?: number
  keyword?: string
}) => {
  return request.get<{ list: TenantWithdrawal[]; total: number }>('/tenant/withdrawals', params)
}

/** 提现打款（POST /tenant/withdrawals/:id/pay）—— v0.3.2 已实现 */
export const payTenantWithdrawalApi = (id: number, data: { pay_trade_no?: string; remark?: string }) => {
  return request.post<{
    id: number
    status: 'paid'
    paid_at: string
    pay_trade_no: string
  }>(`/tenant/withdrawals/${id}/pay`, data)
}

/** 提现审核驳回（POST /tenant/withdrawals/:id/reject）—— v0.3.2 已实现 */
export const rejectTenantWithdrawalApi = (id: number, data: { reason: string }) => {
  return request.post<{
    id: number
    status: 'rejected'
    reason: string
    balance_after: number
  }>(`/tenant/withdrawals/${id}/reject`, data)
}
