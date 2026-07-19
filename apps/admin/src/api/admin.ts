// 平台超管 API
// 对应后端路由：/api/v1/admin/*
// v0.3.1 已交付：dashboard / tenants / packages / agents / notices / logs / security / config / settlements
import { request } from './http'

// ============== 工作台 ==============

export interface AdminDashboardData {
  tenant_total: number
  tenant_active: number
  agent_total: number
  agent_active: number
  app_total: number
  card_total: number
  card_active: number
  order_today: number
  revenue_today: number
  revenue_month: number
  settlement_pending: number
  settlement_amount: number
  recent_tenants?: Array<{ id: number; username: string; created_at: string; status: string }>
  recent_orders?: Array<{ id: number; order_no: string; amount: number; status: string; created_at: string }>
  revenue_trend?: Array<{ date: string; amount: number }>
}

/** 平台看板（GET /admin/dashboard）—— v0.3.1 已实现 */
export const adminDashboardApi = () => {
  return request.get<AdminDashboardData>('/admin/dashboard')
}

// ============== 开发者管理 ==============

export interface AdminTenant {
  id: number
  username: string
  email: string
  phone: string
  company: string
  status: 'active' | 'disabled' | 'pending'
  package_id: number
  package_name?: string
  app_count: number
  card_count: number
  balance: number
  created_at: string
  expired_at: string | null
  remark: string
}

export interface AdminTenantListResp {
  list: AdminTenant[]
  total: number
}

/** 开发者列表（GET /admin/tenants）—— v0.3.1 已实现 */
export const listAdminTenantsApi = (params: { page?: number; page_size?: number; keyword?: string; status?: string }) => {
  return request.get<AdminTenantListResp>('/admin/tenants', params)
}

/** 创建开发者（POST /admin/tenants）—— v0.3.1 已实现 */
export const createAdminTenantApi = (data: {
  username: string
  password: string
  email?: string
  phone?: string
  company?: string
  package_id?: number
  expire_days?: number
  remark?: string
}) => {
  return request.post<AdminTenant>('/admin/tenants', data)
}

/** 更新开发者（PUT /admin/tenants/:id）—— v0.3.1 已实现 */
export const updateAdminTenantApi = (id: number, data: Partial<AdminTenant> & { password?: string; expire_days?: number }) => {
  return request.put<AdminTenant>(`/admin/tenants/${id}`, data)
}

// ============== 套餐管理 ==============

export interface AdminPackage {
  id: number
  name: string
  description: string
  max_apps: number
  max_cards: number
  max_agents: number
  price_monthly: number
  price_yearly: number
  features: string
  status: 'active' | 'disabled'
  created_at: string
}

/** 套餐列表（GET /admin/packages）—— v0.3.1 已实现 */
export const listAdminPackagesApi = (params: { page?: number; page_size?: number; keyword?: string }) => {
  return request.get<{ list: AdminPackage[]; total: number }>('/admin/packages', params)
}

/** 创建套餐（POST /admin/packages）—— v0.3.1 已实现 */
export const createAdminPackageApi = (data: Omit<AdminPackage, 'id' | 'created_at'>) => {
  return request.post<AdminPackage>('/admin/packages', data)
}

// ============== 代理管理（平台维度） ==============

export interface AdminAgent {
  id: number
  username: string
  real_name: string
  phone: string
  tenant_id: number
  tenant_name?: string
  balance: number
  frozen_balance: number
  total_commission: number
  total_withdraw: number
  status: 'active' | 'disabled' | 'pending'
  commission_mode: 'percentage' | 'diff'
  commission_rate: number
  inviter_username?: string
  created_at: string
}

/** 平台代理列表（GET /admin/agents）—— v0.3.1 已实现 */
export const listAdminAgentsApi = (params: { page?: number; page_size?: number; keyword?: string; status?: string; tenant_id?: number }) => {
  return request.get<{ list: AdminAgent[]; total: number }>('/admin/agents', params)
}

/** 平台代理详情/更新（PUT /admin/agents/:id）—— v0.3.1 已实现 */
export const updateAdminAgentApi = (id: number, data: Partial<Pick<AdminAgent, 'status' | 'commission_mode' | 'commission_rate' | 'balance'>>) => {
  return request.put<AdminAgent>(`/admin/agents/${id}`, data)
}

// ============== 公告管理 ==============

export type NoticeType = 'platform' | 'tenant' | 'agent'
export type NoticeStatus = 'draft' | 'published' | 'archived'

export interface AdminNotice {
  id: number
  type: NoticeType
  title: string
  content: string
  status: NoticeStatus
  pinned: boolean
  sort: number
  publish_at: string
  expire_at: string | null
  created_at: string
  updated_at: string
}

/** 平台公告列表（GET /admin/notices）—— v0.3.1 已实现 */
export const listAdminNoticesApi = (params: { page?: number; page_size?: number; type?: NoticeType; status?: NoticeStatus; keyword?: string }) => {
  return request.get<{ list: AdminNotice[]; total: number }>('/admin/notices', params)
}

/** 创建公告（POST /admin/notices）—— v0.3.1 已实现 */
export const createAdminNoticeApi = (data: {
  type: NoticeType
  title: string
  content: string
  status?: NoticeStatus
  pinned?: boolean
  sort?: number
  publish_at?: string
  expire_at?: string
}) => {
  return request.post<AdminNotice>('/admin/notices', data)
}

/** 更新公告（PUT /admin/notices/:id）—— v0.3.1 已实现 */
export const updateAdminNoticeApi = (id: number, data: Partial<AdminNotice>) => {
  return request.put<AdminNotice>(`/admin/notices/${id}`, data)
}

/** 删除公告（DELETE /admin/notices/:id）—— v0.3.1 已实现 */
export const deleteAdminNoticeApi = (id: number) => {
  return request.delete(`/admin/notices/${id}`)
}

// ============== 日志审计 ==============

export type AdminLogType = 'login' | 'operation' | 'pay' | 'security' | 'system'

export interface AdminLog {
  id: number
  type: AdminLogType
  user_id: number
  username: string
  role: string
  action: string
  target: string
  ip: string
  user_agent: string
  status: 'success' | 'fail'
  detail: string
  created_at: string
}

/** 日志审计列表（GET /admin/logs）—— v0.3.1 已实现 */
export const listAdminLogsApi = (params: { page?: number; page_size?: number; type?: AdminLogType; user_id?: number; start_date?: string; end_date?: string; keyword?: string }) => {
  return request.get<{ list: AdminLog[]; total: number }>('/admin/logs', params)
}

// ============== 安全防护 ==============

export interface AdminSecurityStats {
  ip_blacklist_count: number
  ip_blacklist_active: number
  failed_login_today: number
  failed_login_blocked: number
  totp_enabled_users: number
  sensitive_ops_today: number
  recent_blocked_ips?: Array<{ ip: string; reason: string; blocked_at: string }>
}

export interface IpBlacklistItem {
  id: number
  ip: string
  reason: string
  expire_at: string | null
  created_by: string
  created_at: string
}

/** 安全看板（GET /admin/security）—— v0.3.1 已实现 */
export const adminSecurityStatsApi = () => {
  return request.get<AdminSecurityStats>('/admin/security')
}

/** IP 黑名单列表（GET /admin/security/ip_blacklist）—— v0.3.1 已实现 */
export const listIpBlacklistApi = (params: { page?: number; page_size?: number }) => {
  return request.get<{ list: IpBlacklistItem[]; total: number }>('/admin/security/ip_blacklist', params)
}

/** 加入 IP 黑名单（POST /admin/security/ip_blacklist）—— 待核实 */
export const addIpBlacklistApi = (data: { ip: string; reason: string; expire_hours?: number }) => {
  return request.post<IpBlacklistItem>('/admin/security/ip_blacklist', data)
}

/** 移出 IP 黑名单（DELETE /admin/security/ip_blacklist/:id）—— 待核实 */
export const removeIpBlacklistApi = (id: number) => {
  return request.delete(`/admin/security/ip_blacklist/${id}`)
}
