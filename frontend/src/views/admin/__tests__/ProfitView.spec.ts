import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ProfitView from '../ProfitView.vue'

const { getStats, getPaidBalanceSummary } = vi.hoisted(() => ({ getStats: vi.fn(), getPaidBalanceSummary: vi.fn() }))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: { getStats },
  default: { getStats },
}))

vi.mock('@/api/admin/users', () => ({
  usersAPI: { getPaidBalanceSummary },
  default: { getPaidBalanceSummary },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('ProfitView local date range', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 8, 1, 0, 30, 0))
    getStats.mockReset().mockResolvedValue({
      total_requests: 0,
      total_actual_cost: 0,
      total_account_cost: 0,
    })
    getPaidBalanceSummary.mockReset().mockResolvedValue({ total_paid_balance: 0, users_with_paid_balance: 0, ranking: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('uses the local calendar month instead of shifting the first day through UTC', async () => {
    mount(ProfitView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DateRangePicker: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(getStats).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-09-01',
      end_date: '2026-09-01',
      account_cost_basis: 'current_account_rate',
      nocache: 1,
    }))
  })

  it('shows consumed free credit separately from its upstream cost', async () => {
    getStats.mockReset()
      .mockResolvedValueOnce({
        total_requests: 795,
        total_actual_cost: 6.293453,
        total_account_cost: 2.751081,
        total_paid_balance_cost: 0,
        total_free_balance_cost: 0.347252,
        total_free_upstream_cost: 0.192763,
        total_free_balance_issued: 34,
        total_free_balance_consumed: 6.293453,
      })
      .mockResolvedValueOnce({ total_account_cost: 68.926442 })

    const wrapper = mount(ProfitView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DateRangePicker: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.profit.freeConsumed')
    expect(wrapper.text()).toMatch(/6\.29/)
    expect(wrapper.text()).toContain('admin.profit.freeUsed')
    expect(wrapper.text()).toMatch(/0\.35/)
    expect(wrapper.text()).toContain('admin.profit.freeUsageCost')
    expect(wrapper.text()).toMatch(/0\.19/)
  })
})
