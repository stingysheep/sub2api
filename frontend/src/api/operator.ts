import { apiClient } from './client'

export interface OperatorUser {
  id: number
  email: string
  username: string
  status: 'active' | 'disabled'
  // The server returns decimal text. Accept legacy numeric fixtures while keeping UI
  // rendering free from binary floating-point rounding.
  balance: string | number
  free_balance: string | number
  paid_balance: string | number
  created_at: string
  last_active_at: string | null
}

export interface OperatorUsersResponse {
  items: OperatorUser[]
  total: number
  page: number
  page_size: number
  pages?: number
}

export interface OperatorBalanceHistoryItem {
  id: number
  operation: 'add' | 'subtract'
  amount: string
  source: 'free' | 'paid'
  time: string
  reason: string
}

export interface OperatorPagedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface OperatorPaymentOrder {
  id: number
  amount: string
  currency: string
  status: string
  created_at: string
  paid_at: string | null
}

export interface OperatorUsage {
  period: 'today' | '7d' | '30d' | '90d' | string
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  usage_amount: string
  start_at: string
  end_at: string
}

export interface OperatorGroup {
  id: number
  name: string
  platform: string
  status: string
  model_count: number
  updated_at: string
}

export interface OperatorChannelStatus {
  id: number
  name: string
  status: string
  latency_ms: number | null
  updated_at: string
}

export interface OperatorBalanceAdjustmentRequest {
  operation: 'add' | 'subtract'
  amount: string
  source?: 'free' | 'paid'
  reason: string
}

export interface OperatorBalanceAdjustmentResponse {
  operation_id: string
  operation: 'add' | 'subtract'
  amount: string
  source: 'free' | 'paid' | ''
  before_balance: string
  after_balance: string
  before_free_balance: string
  after_free_balance: string
  before_paid_balance: string
  after_paid_balance: string
  free_amount: string
  paid_amount: string
  replayed: boolean
  cache_synced: boolean
}

export const operatorAPI = {
  async listUsers(page = 1, pageSize = 20, search?: string): Promise<OperatorUsersResponse> {
    const { data } = await apiClient.get<OperatorUsersResponse>('/operator/users', {
      params: { page, page_size: pageSize, search }
    })
    return data
  },
  async getUser(id: number): Promise<OperatorUser> {
    const { data } = await apiClient.get<OperatorUser>(`/operator/users/${id}`)
    return data
  },
  async getBalanceHistory(id: number, page = 1, pageSize = 20): Promise<OperatorPagedResponse<OperatorBalanceHistoryItem>> {
    const { data } = await apiClient.get<OperatorPagedResponse<OperatorBalanceHistoryItem>>(`/operator/users/${id}/balance-history`, {
      params: { page, page_size: pageSize }
    })
    return data
  },
  async getPaymentOrders(id: number, page = 1, pageSize = 20): Promise<OperatorPagedResponse<OperatorPaymentOrder>> {
    const { data } = await apiClient.get<OperatorPagedResponse<OperatorPaymentOrder>>(`/operator/users/${id}/payment-orders`, {
      params: { page, page_size: pageSize }
    })
    return data
  },
  async getUsage(id: number, period: 'today' | '7d' | '30d' | '90d' = '30d'): Promise<OperatorUsage> {
    const { data } = await apiClient.get<OperatorUsage>(`/operator/users/${id}/usage`, { params: { period } })
    return data
  },
  async listGroups(): Promise<{ items: OperatorGroup[] }> {
    const { data } = await apiClient.get<{ items: OperatorGroup[] }>('/operator/groups')
    return data
  },
  async getChannelStatus(): Promise<{ items: OperatorChannelStatus[] }> {
    const { data } = await apiClient.get<{ items: OperatorChannelStatus[] }>('/operator/channel-status')
    return data
  },
  async adjustBalance(id: number, request: OperatorBalanceAdjustmentRequest, idempotencyKey: string): Promise<OperatorBalanceAdjustmentResponse> {
    const { data } = await apiClient.post<OperatorBalanceAdjustmentResponse>(
      `/operator/users/${id}/balance-adjustments`, request,
      { headers: { 'Idempotency-Key': idempotencyKey } }
    )
    return data
  }
}

export default operatorAPI
