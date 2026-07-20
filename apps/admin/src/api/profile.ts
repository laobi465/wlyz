// 三角色通用账号设置 API
// 对应后端路由：
//   GET    /api/v1/{role}/auth/me           —— 当前用户信息（已实现）
//   POST   /api/v1/{role}/auth/logout        —— 登出（已实现）
//   POST   /api/v1/{role}/auth/change_password   —— 修改密码（v0.3.1 已实现）
//   PUT    /api/v1/{role}/auth/profile           —— 更新基本资料（v0.3.1 已实现）
//   POST   /api/v1/{role}/auth/2fa/setup         —— 2FA 生成密钥+二维码（v0.3.1 已实现）
//   POST   /api/v1/{role}/auth/2fa/verify        —— 2FA 启用验证（v0.3.1 已实现）
//   POST   /api/v1/{role}/auth/2fa/disable       —— 2FA 关闭（v0.3.1 已实现）
//   GET    /api/v1/{role}/auth/devices           —— 登录设备列表（v0.3.1 已实现）
//   DELETE /api/v1/{role}/auth/devices/:id       —— 踢下线（v0.3.1 已实现）
import { request } from './http'
import type { UserRole } from './auth'

/** 当前用户基本信息（GET /auth/me 实际返回的字段） */
export interface CurrentUser {
  user_id: number
  username: string
  role: UserRole
  tenant_id?: number
  email?: string
  phone?: string
  real_name?: string
  avatar?: string
  company?: string
  status?: string
  created_at?: string
  last_login_at?: string
  last_login_ip?: string
  totp_enabled?: boolean
}

/** 修改密码请求 */
export interface ChangePasswordReq {
  old_password: string
  new_password: string
  confirm_password: string
}

/** 更新基本资料请求 */
export interface UpdateProfileReq {
  real_name?: string
  email?: string
  phone?: string
  company?: string
  avatar?: string
}

/** 2FA 启用返回 */
export interface TwoFASetupResp {
  secret: string
  qr_code_url: string
  backup_codes: string[]
}

/** 登录设备记录 */
export interface LoginDevice {
  id: number
  device_name: string
  ip: string
  location: string
  user_agent: string
  last_active_at: string
  current: boolean
}

/** 当前用户信息（GET /{role}/auth/me）—— 已实现 */
export const currentUserApi = (role: UserRole) => {
  return request.get<CurrentUser>(`/${role}/auth/me`)
}

/** 修改登录密码（POST /{role}/auth/change_password）—— v0.3.1 已实现 */
export const changePasswordApi = (role: UserRole, data: ChangePasswordReq) => {
  return request.post(`/${role}/auth/change_password`, data)
}

/** 更新基本资料（PUT /{role}/auth/profile）—— v0.3.1 已实现 */
export const updateProfileApi = (role: UserRole, data: UpdateProfileReq) => {
  return request.put(`/${role}/auth/profile`, data)
}

/** 生成 2FA 密钥与二维码（POST /{role}/auth/2fa/setup）—— v0.3.1 已实现 */
export const setup2FAApi = (role: UserRole) => {
  return request.post<TwoFASetupResp>(`/${role}/auth/2fa/setup`, {})
}

/** 启用 2FA 验证（POST /{role}/auth/2fa/verify）—— v0.3.1 已实现 */
export const verify2FAApi = (role: UserRole, data: { code: string }) => {
  return request.post(`/${role}/auth/2fa/verify`, data)
}

/** 关闭 2FA（POST /{role}/auth/2fa/disable）—— v0.3.1 已实现 */
export const disable2FAApi = (role: UserRole, data: { code: string; password: string }) => {
  return request.post(`/${role}/auth/2fa/disable`, data)
}

/** 登录设备列表（GET /{role}/auth/devices）—— v0.3.1 已实现 */
export const listLoginDevicesApi = (role: UserRole) => {
  return request.get<{ list: LoginDevice[] }>(`/${role}/auth/devices`)
}

/** 踢指定设备下线
 *  后端路由：POST /{role}/auth/devices/:id/kick（注意是 POST 不是 DELETE，路径有 /kick 后缀）
 */
export const kickDeviceApi = (role: UserRole, id: number) => {
  return request.post(`/${role}/auth/devices/${id}/kick`, {})
}
