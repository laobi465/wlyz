// 卡类与卡密管理 API
// 对应后端路由：/api/v1/tenant/card_types/*, /api/v1/tenant/cards/*
import { request } from './http'

export type CardTypeKind = 'duration' | 'count' | 'permanent' | 'trial' | 'feature'

export interface CardType {
  id: number
  tenant_id: number
  app_id: number
  name: string
  type: CardTypeKind
  duration_seconds: number
  max_uses: number
  price: number
  agent_base_price: number
  features: string
  status: 'active' | 'disabled'
  created_at: string
}

export type CardStatus = 'unused' | 'active' | 'expired' | 'banned' | 'disabled'

export interface Card {
  id: number
  tenant_id: number
  app_id: number
  card_type_id: number
  card_key: string
  checksum: string
  status: CardStatus
  batch_no: string
  prefix: string
  group_tag: string
  duration_seconds: number
  used_count: number
  max_uses: number
  bound_device_id: number | null
  activated_at: string | null
  expires_at: string | null
  last_verify_at: string | null
  creator_type: 'tenant' | 'agent' | 'auto'
  order_id: number | null
  banned_at: string | null
  banned_reason: string
  created_at: string
}

// ============== 卡类 ==============

export const listCardTypesApi = (params: { app_id?: number; page?: number; page_size?: number }) => {
  return request.get<{ list: CardType[]; total: number }>('/tenant/card_types', params)
}

export const createCardTypeApi = (data: {
  app_id: number
  name: string
  type: CardTypeKind
  duration_seconds?: number
  max_uses?: number
  price: number
  agent_base_price?: number
  features?: string
}) => {
  return request.post<CardType>('/tenant/card_types', data)
}

export const updateCardTypeApi = (id: number, data: Partial<CardType>) => {
  return request.put<CardType>(`/tenant/card_types/${id}`, data)
}

// ============== 卡密 ==============

export const listCardsApi = (params: {
  app_id?: number
  card_type_id?: number
  status?: CardStatus
  batch_no?: string
  page?: number
  page_size?: number
}) => {
  return request.get<{ list: Card[]; total: number }>('/tenant/cards', params)
}

export const getCardApi = (id: number) => {
  return request.get<Card>(`/tenant/cards/${id}`)
}

export const generateCardsApi = (data: {
  app_id: number
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
  }>('/tenant/cards/generate', data)
}

export const banCardApi = (id: number, reason?: string) => {
  return request.post(`/tenant/cards/${id}/ban`, { reason })
}

export const unbanCardApi = (id: number) => {
  return request.post(`/tenant/cards/${id}/unban`, {})
}

export const deleteCardApi = (id: number) => {
  return request.delete(`/tenant/cards/${id}`)
}
