import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, post } }))

import { operatorAPI } from '@/api/operator'

describe('operator API contract', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 20 } })
    post.mockResolvedValue({ data: { operation_id: 'op_test', replayed: false, cache_synced: true } })
  })

  it('uses the scoped operator paths and pagination fields', async () => {
    await operatorAPI.listUsers(2, 25, 'alice')
    expect(get).toHaveBeenCalledWith('/operator/users', { params: { page: 2, page_size: 25, search: 'alice' } })
  })

  it('keeps the idempotency key in a header and amount as a decimal string', async () => {
    await operatorAPI.adjustBalance(7, { operation: 'add', amount: '1.25000000', source: 'free', reason: 'correction' }, 'idem-operator-7')
    expect(post).toHaveBeenCalledWith(
      '/operator/users/7/balance-adjustments',
      { operation: 'add', amount: '1.25000000', source: 'free', reason: 'correction' },
      { headers: { 'Idempotency-Key': 'idem-operator-7' } },
    )
  })

  it('passes pagination for balance history and payment orders', async () => {
    await operatorAPI.getBalanceHistory(7, 3, 15)
    expect(get).toHaveBeenCalledWith('/operator/users/7/balance-history', { params: { page: 3, page_size: 15 } })
    await operatorAPI.getPaymentOrders(7)
    expect(get).toHaveBeenCalledWith('/operator/users/7/payment-orders', { params: { page: 1, page_size: 20 } })
  })

  it('requests the contract default usage period', async () => {
    await operatorAPI.getUsage(7)
    expect(get).toHaveBeenCalledWith('/operator/users/7/usage', { params: { period: '30d' } })
  })
})
