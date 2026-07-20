// 代理门户 H5 公开 API（v0.4.x 残留项 2 P-06）
// 对应后端路由：
//   GET  /api/v1/public/portal/:agent_id        - 获取代理门户信息 + 可售卡类
//   POST /api/v1/public/portal/:agent_id/order  - 创建订单（走开发者支付通道）
// 注：无需登录鉴权，任何用户均可访问代理门户
import { request } from './http'

// ============== 类型定义 ==============

export interface PortalAgentInfo {
  agent_id: number
  username: string
  real_name: string
  tenant_id: number
  tenant_name: string
  subdomain?: string
  status: string
}

export interface PortalCardType {
  id: number
  app_id: number
  app_name: string
  name: string
  type: 'duration' | 'count' | 'permanent' | 'trial' | 'feature'
  duration_seconds: number
  max_uses: number
  price: number
  features: string
}

export interface PortalInfo {
  agent: PortalAgentInfo
  card_types: PortalCardType[]
  total: number
}

export interface PortalOrderReq {
  app_id: number
  card_type_id: number
  quantity: number
  pay_type: 'alipay' | 'wxpay' | 'qqpay'
  buyer_contact?: string
}

export interface PortalOrderResp {
  order_no: string
  order_id: number
  pay_url: string
  total_amount: number
  money: string
  pay_type: string
  pay_mode: 'platform' | 'tenant'
  expire_at: number
  quantity: number
  card_type: string
}

// ============== API 方法 ==============

/** 获取代理门户信息 + 可售卡类列表（公开接口） */
export const getPortalInfoApi = (agentId: number) => {
  return request.get<PortalInfo>(`/public/portal/${agentId}`)
}

/** 代理门户下单（公开接口，走开发者支付通道） */
export const createPortalOrderApi = (agentId: number, data: PortalOrderReq) => {
  return request.post<PortalOrderResp>(`/public/portal/${agentId}/order`, data)
}
