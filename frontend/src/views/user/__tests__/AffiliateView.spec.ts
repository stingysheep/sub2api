import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const { copyToClipboard, getAffiliateDetail } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      invitees: [
        { user_id: 2, email: 'invitee@example.com', username: 'invitee', total_rebate: 0.00000087, created_at: '2026-09-05T00:00:00Z' },
      ],
      leaderboard: [
        { rank: 1, email: 'top@example.com', invited_count: 10, history_rebate: 1.234567 },
        { rank: 2, email: 'second@example.com', invited_count: 5, history_rebate: 0.617283 },
      ],
    })
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toEqual(expect.arrayContaining([
        'min-w-0',
        'break-all',
        'sm:flex-1',
        'sm:truncate',
      ]))
      expect(Array.from(value.element.parentElement?.classList ?? [])).toEqual(expect.arrayContaining([
        'flex-col',
        'items-stretch',
        'sm:flex-row',
        'sm:items-center',
      ]))
    }

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toEqual(expect.arrayContaining([
        'w-full',
        'sm:w-auto',
        'sm:shrink-0',
      ]))
    }

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenNthCalledWith(
      1,
      `${window.location.origin}/register?aff=${encodeURIComponent(affiliateCode)}`,
      'affiliate.linkCopied',
    )
    expect(copyToClipboard).toHaveBeenNthCalledWith(2, affiliateCode, 'affiliate.codeCopied')
  })

  it('renders leaderboard columns, proportional progress, and requested precision', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          AffiliateNetworkArt: true,
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('affiliate.leaderboard.columns.rank')
    expect(wrapper.text()).toContain('$1.235')
    expect(wrapper.text()).toContain('$0.617')
    expect(wrapper.text()).toContain('$0.00000087')
    const bars = wrapper.findAll('[style]').filter((node) => (node.attributes('style') || '').includes('width:'))
    expect(bars).toHaveLength(2)
    expect((bars[0].attributes('style') || '')).toContain('width: 100%')
    expect((bars[1].attributes('style') || '')).toContain('width: 49.999')
  })

  it('renders the animated artwork beside the promotion hero on desktop', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          AffiliateNetworkArt: { template: '<div data-testid="affiliate-network-art" />' },
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="affiliate-network-art"]').exists()).toBe(true)
    expect(wrapper.html()).toContain('md:grid-cols-[minmax(0,1fr)_auto]')
  })
})
