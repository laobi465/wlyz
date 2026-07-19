// 支付相关 API
// 对应后端路由：/api/v1/pay/*, /api/v1/admin/settlements/*
import { request } from './http'

export type PayType = 'alipay' | 'wxpay' | 'qqpay'
export type PayStatus = 'pending' | 'paid' | 'closed' | 'refunded'

export interface PayOrder {
  order_no: string
  pay_status: PayStatus
  pay_channel: string
  total_amount: number
  quantity: number
  card_ids: number[]
  pay_trade_no: string
  paid_at: string | null
  created_at: string
}

/** 终端用户下单（无鉴权） */
export const createPayOrderApi = (data: {
  app_id: number
  card_type_id: number
  quantity: number
  pay_type: PayType
  buyer_contact?: string
  agent_id?: number
}) => {
  return request.post<{
    order_no: string
    order_id: number
    pay_url: string
    total_amount: number
    money: string
    pay_type: PayType
    expire_at: number
    quantity: number
    card_type: string
  }>('/pay/order', data)
}

/** 查询订单状态 */
export const getPayOrderApi = (orderNo: string) => {
  return request.get<PayOrder>(`/pay/order/${orderNo}`)
}

// ============== 超管：结算管理 ==============

export interface Settlement {
  id: number
  tenant_id: number
  order_id: number
  order_no: string
  gross_amount: number
  commission_rate: number
  commission_amount: number
  net_amount: number
  status: 'pending' | 'settled' | 'rejected'
  settled_at: string | null
  settle_batch_no: string
  settle_method: string
  settle_remark: string
  created_at: string
}

export const listSettlementsApi = (params: {
  tenant_id?: number
  status?: string
  page?: number
  page_size?: number
}) => {
  return request.get<{ list: Settlement[]; total: number; page: number; page_size: number }>(
    '/admin/settlements',
    params
  )
}

export const settleOrderApi = (id: number, data: { method?: 'manual' | 'alipay' | 'wechat' | 'bank'; remark?: string }) => {
  return request.post<{ id: number; status: string; settle_batch_no: string; settled_at: string }>(
    `/admin/settlements/${id}/settle`,
    data
  )
}

/** 测试平台易支付配置 */
export const testPayConfigApi = () => {
  return request.post<{ ok: boolean; gateway_url: string; pid: string; sign_type: string }>('/admin/pay/test', {})
}
