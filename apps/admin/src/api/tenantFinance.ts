// 开发者财务 API（v0.3.4 新增）
// 对应后端路由：/api/v1/tenant/settlements | /balance_* | /withdrawals/mine | /withdraw
// + 平台审核相关：/api/v1/admin/tenant_withdrawals | /reconciliation
import { request } from './http'

// ============== 通用类型 ==============

/** 平台抽成结算记录（platform_settlement 表） */
export interface PlatformSettlement {
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

/** 开发者余额流水（tenant_balance_log 表） */
export interface TenantBalanceLog {
  id: number
  tenant_id: number
  type: 'settle' | 'withdraw' | 'refund' | 'adjust' | string
  amount: number
  balance_after: number
  related_order_id: number | null
  related_settlement_id: number | null
  related_withdraw_id: number | null
  pay_method: string
  settle_batch_no: string
  status: 'pending' | 'settled' | 'rejected'
  remark: string
  created_at: string
}

/** 开发者提现申请（tenant_withdraw 表） */
export interface TenantWithdrawal {
  id: number
  tenant_id: number
  amount: number
  pay_method: 'alipay' | 'wechat' | 'bank' | string
  pay_account: string
  status: 'pending' | 'paid' | 'rejected' | 'failed'
  audit_remark: string
  pay_trade_no: string
  paid_at: string | null
  audited_by: number | null
  created_at: string
}

/** 开发者提现申请列表项（联表 sys_tenant） */
export interface AdminTenantWithdrawal extends TenantWithdrawal {
  tenant_username: string
  tenant_company: string
}

/** 余额概览返回 */
export interface TenantBalanceOverview {
  balance: number
  frozen_balance: number
  settled_total: number
  withdrawn_total: number
  pending_withdraw: number
  updated_at: string
}

/** 对账报表返回 */
export interface ReconciliationData {
  start_date: string
  end_date: string
  order_count: number
  gross_total: number
  commission_sum: number
  net_total: number
  settled_sum: number
  pending_sum: number
  withdrawn_sum: number
  pending_withdraw_sum: number
  balance_theory: number
}

// ============== 开发者侧 API ==============

/** 开发者查询自己的结算记录（GET /tenant/settlements） */
export const listTenantSettlementsApi = (params: {
  page?: number
  page_size?: number
  status?: string
  order_no?: string
  start_date?: string
  end_date?: string
}) => {
  return request.get<{ list: PlatformSettlement[]; total: number; page: number; page_size: number; pending_sum: number; settled_sum: number }>('/tenant/settlements', params)
}

/** 开发者余额概览（GET /tenant/balance_overview） */
export const tenantBalanceOverviewApi = () => {
  return request.get<TenantBalanceOverview>('/tenant/balance_overview')
}

/** 开发者查询自己的余额流水（GET /tenant/balance_logs） */
export const listTenantBalanceLogsApi = (params: {
  page?: number
  page_size?: number
  type?: string
  status?: string
  start_date?: string
  end_date?: string
}) => {
  return request.get<{ list: TenantBalanceLog[]; total: number; page: number; page_size: number }>('/tenant/balance_logs', params)
}

/** 开发者查询自己的提现申请（GET /tenant/withdrawals/mine） */
export const listTenantOwnWithdrawalsApi = (params: {
  page?: number
  page_size?: number
  status?: string
  start_date?: string
  end_date?: string
}) => {
  return request.get<{ list: TenantWithdrawal[]; total: number; page: number; page_size: number }>('/tenant/withdrawals/mine', params)
}

/** 开发者发起提现申请（POST /tenant/withdraw） */
export const tenantWithdrawApi = (data: {
  amount: number
  pay_method: 'alipay' | 'wechat' | 'bank'
  pay_account: string
  remark?: string
}) => {
  return request.post<{ id: number; status: string; amount: number; balance_after: number; message: string }>('/tenant/withdraw', data)
}

// ============== 平台超管审核 API ==============

/** 超管查询开发者提现申请列表（GET /admin/tenant_withdrawals） */
export const listAdminTenantWithdrawalsApi = (params: {
  page?: number
  page_size?: number
  status?: string
  tenant_id?: number
  keyword?: string
  start_date?: string
  end_date?: string
}) => {
  return request.get<{ list: AdminTenantWithdrawal[]; total: number; page: number; page_size: number }>('/admin/tenant_withdrawals', params)
}

/** 超管打款开发者提现（POST /admin/tenant_withdrawals/:id/pay） */
export const payAdminTenantWithdrawalApi = (id: number, data: {
  pay_trade_no?: string
  remark?: string
}) => {
  return request.post<{ id: number; status: string; paid_at: string; pay_trade_no: string }>(`/admin/tenant_withdrawals/${id}/pay`, data)
}

/** 超管驳回开发者提现（POST /admin/tenant_withdrawals/:id/reject） */
export const rejectAdminTenantWithdrawalApi = (id: number, data: { reason: string }) => {
  return request.post<{ id: number; status: string; balance_after: number }>(`/admin/tenant_withdrawals/${id}/reject`, data)
}

/** 超管批量结算（POST /admin/settlements/batch_settle） */
export const batchSettleApi = (data: {
  settlement_ids: number[]
  method?: 'manual' | 'alipay' | 'wechat' | 'bank'
  remark?: string
}) => {
  return request.post<{ batch_no: string; success_count: number; tenant_count: number; balances: Record<number, number>; settled_at: string }>('/admin/settlements/batch_settle', data)
}

/** 对账报表（GET /admin/reconciliation） */
export const reconciliationApi = (params: {
  start_date?: string
  end_date?: string
  tenant_id?: number
}) => {
  return request.get<ReconciliationData>('/admin/reconciliation', params)
}
