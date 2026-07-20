// 代理控制台 API
// 对应后端路由：/api/v1/agent/* + /api/v1/public/auth/agent/register/*
// v0.3.1 已交付：dashboard / me / card_types / cards / orders / balance_logs / withdraw / recharge / notices
// v0.3.6 新增：registerConfig / register / registerOrderStatus
import { request } from './http'
import type { CardStatus } from './cards'

// ============== 类型定义 ==============

export interface AgentProfile {
  // 后端 agentProfile.AgentID 的 json tag 是 "agent_id"
  agent_id: number
  username: string
  tenant_id: number
  tenant_name?: string
  real_name: string
  phone: string
  balance: number
  frozen_balance: number
  total_commission: number
  total_withdraw: number
  status: 'active' | 'disabled' | 'pending'
  created_at: string
  /** 邀请该代理的开发者用户名 */
  inviter_username?: string
  /** 佣金模式：按比例 / 按差价 */
  commission_mode?: 'percentage' | 'diff'
  /** 佣金比例（百分比，仅 percentage 模式生效） */
  commission_rate?: number
}

export interface AgentDashboard {
  balance: number
  frozen_balance: number
  today_purchased: number
  today_spent: number
  total_purchased: number
  total_spent: number
  total_commission: number
  total_withdraw: number
  pending_withdraw: number
  recent_orders?: AgentOrder[]
}

export interface AgentCardType {
  id: number
  app_id: number
  app_name: string
  name: string
  type: 'duration' | 'count' | 'permanent' | 'trial' | 'feature'
  duration_seconds: number
  max_uses: number
  /** 终端用户售价 */
  price: number
  /** 代理结算价（从此价格扣款） */
  agent_base_price: number
  /** 代理佣金（按差价模式时 = price - agent_base_price） */
  agent_commission: number
  features: string
  status: 'active' | 'disabled'
}

export interface AgentCard {
  id: number
  app_id: number
  app_name?: string
  card_type_id: number
  card_type_name?: string
  card_key: string
  status: CardStatus
  batch_no: string
  used_count: number
  max_uses: number
  activated_at: string | null
  expires_at: string | null
  cost_price: number
  created_at: string
}

export type AgentOrderStatus = 'paid' | 'pending' | 'closed' | 'refunded'

export interface AgentOrder {
  id: number
  order_no: string
  app_id: number
  app_name?: string
  card_type_id: number
  card_type_name?: string
  quantity: number
  total_amount: number
  pay_status: AgentOrderStatus
  pay_channel: string
  paid_at: string | null
  created_at: string
  /** 该订单产生的佣金 */
  commission_amount: number
}

// Bug 12 P1：后端 AgentBalanceLog.Type 枚举 recharge/deduct/commission/withdraw/adjust
// Bug 13 P1：后端 AgentBalanceLog.Status 枚举 pending/settled/rejected
export type CommissionType = 'recharge' | 'deduct' | 'commission' | 'withdraw' | 'adjust'
export type CommissionStatus = 'pending' | 'settled' | 'rejected'

export interface AgentCommission {
  id: number
  type: CommissionType
  amount: number
  balance_after: number
  status: CommissionStatus
  related_order_no: string
  remark: string
  created_at: string
  /** 提现申请时填写的收款方式 */
  withdraw_method?: string
  /** 提现申请时填写的收款账号 */
  withdraw_account?: string
}

// ============== API 方法 ==============

/** 代理工作台 */
export const agentDashboardApi = () => {
  return request.get<AgentDashboard>('/agent/dashboard')
}

/** 当前代理信息（含余额、佣金统计） */
export const agentMeApi = () => {
  return request.get<AgentProfile>('/agent/auth/me')
}

/** 代理可购买的卡类列表（受开发者授权范围限制） */
export const listAgentCardTypesApi = (params: { app_id?: number; page?: number; page_size?: number }) => {
  return request.get<{ list: AgentCardType[]; total: number }>('/agent/card_types', params)
}

/** 代理卡密列表 */
export const listAgentCardsApi = (params: {
  app_id?: number
  card_type_id?: number
  status?: CardStatus
  batch_no?: string
  page?: number
  page_size?: number
}) => {
  return request.get<{ list: AgentCard[]; total: number }>('/agent/cards', params)
}

/** 代理购卡（扣余额生成卡密） */
export const agentGenerateCardsApi = (data: {
  card_type_id: number
  quantity: number
  prefix?: string
  group_tag?: string
}) => {
  return request.post<{
    batch_no: string
    quantity: number
    card_keys: string[]
    card_ids: number[]
    cost_total: number
    balance_after: number
  }>('/agent/cards/generate', data)
}

/** 代理订单列表 */
export const listAgentOrdersApi = (params: {
  status?: AgentOrderStatus
  page?: number
  page_size?: number
}) => {
  return request.get<{ list: AgentOrder[]; total: number }>('/agent/orders', params)
}

/** 佣金/流水明细列表 */
export const listAgentCommissionApi = (params: {
  type?: CommissionType
  status?: CommissionStatus
  page?: number
  page_size?: number
}) => {
  return request.get<{ list: AgentCommission[]; total: number }>('/agent/commission', params)
}

/** 提现申请 */
export const agentWithdrawApi = (data: {
  amount: number
  method: 'alipay' | 'wechat' | 'bank'
  account: string
  real_name?: string
  remark?: string
}) => {
  return request.post<{ id: number; status: string; amount: number }>('/agent/withdraw', data)
}

/** 充值申请（v0.3.1 已实现） */
export const agentRechargeApi = (data: {
  amount: number
  pay_method: 'alipay' | 'wechat' | 'bank' | 'manual'
  pay_voucher?: string
  remark?: string
}) => {
  return request.post<{ id: number; status: string; amount: number; message: string }>('/agent/recharge', data)
}

// ============== 消息通知 ==============

export interface AgentNotice {
  id: number
  // Bug 10 P1：与后端 Notice.Type 枚举对齐 platform/developer/app/agent_notify
  type: 'platform' | 'developer' | 'app' | 'agent_notify'
  title: string
  content: string
  pinned: boolean
  publish_at: string
  expire_at: string | null
  read: boolean
  created_at: string
}

/** 代理消息通知列表（GET /agent/notices）—— v0.3.1 已实现 */
export const listAgentNoticesApi = (params: { page?: number; page_size?: number; type?: string; unread_only?: boolean }) => {
  return request.get<{ list: AgentNotice[]; total: number }>('/agent/notices', params)
}

/** 标记通知为已读（POST /agent/notices/:id/read）—— v0.3.1 已实现 */
export const readAgentNoticeApi = (id: number) => {
  return request.post(`/agent/notices/${id}/read`, {})
}

// ============== 代理注册（v0.3.6 新增） ==============

/** 代理注册所需配置（未登录可读，不含敏感字段） */
export interface AgentRegisterConfig {
  register_fee: number
  pay_enabled: boolean
  pay_methods: string[] // ['alipay','wxpay','qqpay']
  order_expire_seconds: number
}

/** 代理注册订单状态 */
export type AgentRegisterOrderStatus = 'pending' | 'paid' | 'closed' | 'refunded'

/** 代理注册订单详情 */
export interface AgentRegisterOrder {
  order_no: string
  pay_status: AgentRegisterOrderStatus
  amount: number
  username: string
  created_at: string
  paid_at: string | null
  agent_id?: number
}

/** 代理注册返回（含支付跳转 URL） */
export interface AgentRegisterResult {
  order_no: string
  pay_url: string
  amount: number
  message: string
}

/** 读取代理注册配置（公开接口，未登录可调） */
export const agentRegisterConfigApi = () => {
  return request.get<AgentRegisterConfig>('/public/auth/agent/register/config')
}

/** 发起代理注册（创建预支付订单 + 返回 pay_url） */
export const agentRegisterApi = (data: {
  invite_code: string
  username: string
  password: string
  phone?: string
  pay_type: 'alipay' | 'wxpay' | 'qqpay'
}) => {
  return request.post<AgentRegisterResult>('/public/auth/agent/register', data)
}

/** 查询代理注册订单状态（支付完成跳回后调） */
export const agentRegisterOrderStatusApi = (orderNo: string) => {
  return request.get<AgentRegisterOrder>(`/public/auth/agent/register/order/${orderNo}`)
}

// ============== v0.4.x 残留项 1：代理子域名绑定 ==============

export type AgentSubdomainStatus = 'none' | 'pending' | 'approved' | 'rejected'

export interface AgentSubdomainInfo {
  enabled: boolean
  pattern: string
  subdomain: string
  subdomain_status: AgentSubdomainStatus
}

export interface AgentApplySubdomainReq {
  subdomain: string
}

/** 查询当前代理子域名绑定状态（GET /agent/subdomain） */
export const agentSubdomainStatusApi = () => {
  return request.get<AgentSubdomainInfo>('/agent/subdomain')
}

/** 申请子域名（POST /agent/subdomain/apply） */
export const agentApplySubdomainApi = (data: AgentApplySubdomainReq) => {
  return request.post<{ subdomain: string; subdomain_status: string; message: string }>(
    '/agent/subdomain/apply',
    data
  )
}

/** 解绑子域名（DELETE /agent/subdomain） */
export const agentUnbindSubdomainApi = () => {
  return request.delete<{ subdomain_status: string; message: string }>('/agent/subdomain')
}

// ============== v0.4.x 残留项 3（P-10）：代理扫码购卡 URL ==============

export interface AgentPortalQrCode {
  agent_id: number
  subdomain: string
  subdomain_status: AgentSubdomainStatus
  portal_url: string
  qrcode_api: string
}

/** 获取代理门户购卡二维码 URL（GET /agent/portal/qrcode） */
export const agentPortalQrCodeApi = () => {
  return request.get<AgentPortalQrCode>('/agent/portal/qrcode')
}
