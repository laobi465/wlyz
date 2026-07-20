// H5 终端用户 API（v0.4.0 收尾项 C）
// 对应后端路由：
//   公开（无需 token）：
//     POST /api/v1/public/enduser/register
//     POST /api/v1/public/enduser/login
//     POST /api/v1/public/enduser/refresh
//     POST /api/v1/public/enduser/verify_code
//     POST /api/v1/public/enduser/reset_password
//     GET  /api/v1/public/notices/:id   （v0.4.x 残留项 2：公告详情）
//     GET  /api/v1/public/contact        （v0.4.x 残留项 4：客服联系方式）
//   鉴权（需 access_token）：
//     GET    /api/v1/h5/me
//     PUT    /api/v1/h5/me
//     POST   /api/v1/h5/me/password
//     POST   /api/v1/h5/logout
//     GET    /api/v1/h5/sessions
//     POST   /api/v1/h5/sessions/:jti/kick
//     POST   /api/v1/h5/cards/bind
//     POST   /api/v1/h5/cards/unbind
//     GET    /api/v1/h5/cards
//     GET    /api/v1/h5/cards/:id
//     GET    /api/v1/h5/orders           （v0.4.x 残留项 1：我的订单列表）
//     GET    /api/v1/h5/orders/:order_no （v0.4.x 残留项 1：订单详情）
import { request } from './http'

// ============== 类型定义 ==============

export interface EndUserLoginReq {
  app_key: string
  username: string
  password: string
}

export interface EndUserRegisterReq {
  app_key: string
  username: string
  password: string
  email?: string
  phone?: string
  verify_code?: string
}

export interface EndUserInfo {
  id: number
  tenant_id: number
  app_id: number
  username: string
  nickname: string
  avatar: string
  email: string
  phone: string
  status: string
  last_login_at: string
  last_login_ip: string
  created_at: string
}

export interface EndUserLoginResp {
  access_token: string
  refresh_token: string
  expires_at: number
  user: EndUserInfo
}

export interface EndUserCard {
  id: number
  card_key: string
  card_type: string
  status: string
  expires_at: string
  bound_at: string
  app_name?: string
}

export interface EndUserSession {
  jti: string
  user_agent: string
  ip: string
  expires_at: string
  created_at: string
  is_current: boolean
}

export type EndUserVerifyTarget = 'email' | 'phone'

export interface EndUserSendVerifyCodeReq {
  app_key: string
  target: string
  type: 'register' | 'reset_password'
}

export interface EndUserResetPasswordReq {
  app_key: string
  target: string
  verify_code: string
  new_password: string
}

export interface EndUserChangePasswordReq {
  old_password: string
  new_password: string
}

export interface EndUserUpdateProfileReq {
  nickname?: string
  avatar?: string
  email?: string
  phone?: string
}

export interface EndUserListCardsResp {
  list: EndUserCard[]
  total: number
  page: number
  page_size: number
}

export interface EndUserListSessionsResp {
  list: EndUserSession[]
  total: number
}

// ============== 公开端点 ==============

export const endUserRegisterApi = (data: EndUserRegisterReq) => {
  return request.post<EndUserLoginResp>('/public/enduser/register', data)
}

export const endUserLoginApi = (data: EndUserLoginReq) => {
  return request.post<EndUserLoginResp>('/public/enduser/login', data)
}

export const endUserRefreshApi = (refreshToken: string) => {
  return request.post<{ access_token: string; refresh_token: string; expires_at: number }>(
    '/public/enduser/refresh',
    { refresh_token: refreshToken }
  )
}

export const endUserSendVerifyCodeApi = (data: EndUserSendVerifyCodeReq) => {
  return request.post<{ message: string; target: string; expires_in?: number }>(
    '/public/enduser/verify_code',
    data
  )
}

export const endUserResetPasswordApi = (data: EndUserResetPasswordReq) => {
  return request.post<{ message: string }>('/public/enduser/reset_password', data)
}

// ============== 鉴权端点 ==============

export const endUserMeApi = () => {
  return request.get<EndUserInfo>('/h5/me')
}

export const endUserUpdateProfileApi = (data: EndUserUpdateProfileReq) => {
  return request.put<EndUserInfo>('/h5/me', data)
}

export const endUserChangePasswordApi = (data: EndUserChangePasswordReq) => {
  return request.post<{ message: string }>('/h5/me/password', data)
}

export const endUserLogoutApi = () => {
  return request.post<{ message: string }>('/h5/logout', {})
}

export const endUserListSessionsApi = () => {
  return request.get<EndUserListSessionsResp>('/h5/sessions')
}

export const endUserKickSessionApi = (jti: string) => {
  return request.post<{ message: string }>(`/h5/sessions/${jti}/kick`, {})
}

export const endUserBindCardApi = (cardKey: string) => {
  return request.post<EndUserCard>('/h5/cards/bind', { card_key: cardKey })
}

export const endUserUnbindCardApi = (cardId: number) => {
  return request.post<{ message: string }>('/h5/cards/unbind', { card_id: cardId })
}

export const endUserListMyCardsApi = (page: number, pageSize: number) => {
  return request.get<EndUserListCardsResp>('/h5/cards', { page, page_size: pageSize })
}

export const endUserGetCardDetailApi = (id: number) => {
  return request.get<EndUserCard>(`/h5/cards/${id}`)
}

// ============== v0.4.x 残留项 1：U-11 终端用户订单列表 H5 接入 ==============

export type EndUserOrderStatus = 'pending' | 'paid' | 'closed' | 'refunded'

export interface EndUserOrder {
  id: number
  order_no: string
  app_id: number
  tenant_id: number
  card_type_id: number
  quantity: number
  unit_price: number
  total_amount: number
  pay_channel: string
  pay_status: EndUserOrderStatus
  pay_trade_no: string
  paid_at: string | null
  created_at: string
  client_ip: string
}

export interface EndUserOrderCard {
  id: number
  card_key: string
  status: string
  expires_at: string | null
  activated_at: string | null
  duration_seconds: number
  max_uses: number
  used_count: number
}

export interface EndUserOrderDetail extends EndUserOrder {
  buyer_contact: string
  card_ids: number[]
  card_keys: string[]      // 仅 paid 时非空
  cards: EndUserOrderCard[] // 仅 paid 时非空：卡密明细
}

export interface EndUserListOrdersResp {
  list: EndUserOrder[]
  total: number
  page: number
  page_size: number
}

/** 我的订单列表（支持状态筛选） */
export const endUserListOrdersApi = (params: {
  page?: number
  page_size?: number
  status?: EndUserOrderStatus | ''
}) => {
  return request.get<EndUserListOrdersResp>('/h5/orders', params)
}

/** 订单详情（含卡密明文，仅 paid 时返回） */
export const endUserGetOrderApi = (orderNo: string) => {
  return request.get<EndUserOrderDetail>(`/h5/orders/${orderNo}`)
}

// ============== v0.4.x 残留项 2：U-12 公告详情 H5 页面 ==============

export interface EndUserNoticeDetail {
  id: number
  type: string
  tenant_id: number | null
  app_id: number | null
  title: string
  content: string
  content_format: 'text' | 'html'
  is_pinned: boolean
  show_badge: boolean
  start_at: string
  end_at: string | null
  view_count: number
  sort: number
  created_at: string
}

/** 公告列表项（精简字段，用于 H5 列表展示） */
export interface EndUserNoticeListItem {
  id: number
  title: string
  type: string
  is_pinned: boolean
  start_at: string
  end_at: string | null
  view_count: number
  created_at: string
}

/** 平台公告列表（公开端点，复用现有 GET /public/notices/platform） */
export const endUserListPlatformNoticesApi = () => {
  return request.get<{ list: EndUserNoticeListItem[] }>('/public/notices/platform')
}

/** 公告详情（公开端点） */
export const endUserGetNoticeApi = (id: number | string) => {
  return request.get<EndUserNoticeDetail>(`/public/notices/${id}`)
}

// ============== v0.4.x 残留项 4：U-14 联系客服 H5 页面 ==============

export interface ContactInfo {
  qq_group: string
  wechat: string
  email: string
  phone: string
}

/** 联系客服信息（公开端点，从 sys_config 读取） */
export const getContactInfoApi = () => {
  return request.get<ContactInfo>('/public/contact')
}
