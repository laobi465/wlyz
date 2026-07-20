// H5 终端用户 API（v0.4.0 收尾项 C）
// 对应后端路由：
//   公开（无需 token）：
//     POST /api/v1/public/enduser/register
//     POST /api/v1/public/enduser/login
//     POST /api/v1/public/enduser/refresh
//     POST /api/v1/public/enduser/verify_code
//     POST /api/v1/public/enduser/reset_password
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
