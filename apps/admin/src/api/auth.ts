// 认证相关 API
// 对应后端路由：
//   POST /api/v1/public/auth/admin/login
//   POST /api/v1/public/auth/tenant/login
//   POST /api/v1/public/auth/agent/login
//   POST /api/v1/public/auth/tenant/register
//   POST /api/v1/public/auth/agent/register
//   POST /api/v1/public/auth/refresh
//   POST /api/v1/{admin|tenant|agent}/auth/logout
//   GET  /api/v1/{admin|tenant|agent}/auth/me
import { request } from './http'

export type UserRole = 'admin' | 'tenant' | 'agent'

export interface LoginResp {
  access_token: string
  refresh_token: string
  expires_at: number
  user: {
    id: number
    username: string
    role: UserRole
    tenant_id?: number
    [key: string]: any
  }
  /** 当后端要求 2FA 时返回此字段，access_token 为空 */
  totp_required?: boolean
  /** 临时令牌，用于提交 2FA 验证 */
  temp_token?: string
}

export interface LoginReq {
  username: string
  password: string
  /** TOTP 6 位数字（启用 2FA 时必填） */
  totp_code?: string
  /** 登录临时令牌（2FA 二阶段） */
  temp_token?: string
  /** 验证码（待实现：图形验证码） */
  captcha?: string
}

/** 三角色统一登录入口 */
export const loginApi = (role: UserRole, data: LoginReq) => {
  return request.post<LoginResp>(`/public/auth/${role}/login`, data)
}

/** 开发者注册 */
export const tenantRegisterApi = (data: {
  username: string
  password: string
  email?: string
  phone?: string
  company?: string
  invite_code?: string
}) => {
  return request.post<LoginResp>('/public/auth/tenant/register', data)
}

/** 代理注册 */
export const agentRegisterApi = (data: {
  username: string
  password: string
  invite_code: string
  real_name?: string
  phone?: string
}) => {
  return request.post<LoginResp>('/public/auth/agent/register', data)
}

/** 刷新 token */
export const refreshTokenApi = (refreshToken: string) => {
  return request.post<{ access_token: string; refresh_token: string; expires_at: number }>(
    '/public/auth/refresh',
    { refresh_token: refreshToken }
  )
}

/** 当前用户信息 */
export const currentUserApi = (role: UserRole) => {
  return request.get(`/api/v1/${role}/auth/me`)
}

/** 登出（黑名单 refresh_token） */
export const logoutApi = (role: UserRole) => {
  return request.post(`/api/v1/${role}/auth/logout`, {})
}
