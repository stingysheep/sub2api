import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UpstreamBalanceModal from '../UpstreamBalanceModal.vue'
import type { Account } from '@/types'

const { getUpstreamBalance } = vi.hoisted(() => ({ getUpstreamBalance: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getUpstreamBalance } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const account = {
  id: 42, name: 'relay-key', platform: 'openai', type: 'apikey', proxy_id: null,
  concurrency: 1, priority: 1, status: 'active', error_message: null, last_used_at: null,
  expires_at: null, auto_pause_on_expired: false, created_at: '', updated_at: '', schedulable: true,
  rate_limited_at: null, rate_limit_reset_at: null, overload_until: null,
  temp_unschedulable_until: null, temp_unschedulable_reason: null,
  session_window_start: null, session_window_end: null, session_window_status: null
} as Account

describe('UpstreamBalanceModal', () => {
  it('queries only by account id and renders balance entries without credentials', async () => {
    getUpstreamBalance.mockResolvedValue({
      provider: 'example-relay', fetched_at: '2026-09-17T00:00:00Z', status_code: 200,
      entries: [{ plan_name: 'Pro', remaining: 12.5, used: 7.5, total: 20, unit: 'USD', is_valid: true }]
    })
    const wrapper = mount(UpstreamBalanceModal, { props: { show: true, account }, attachTo: document.body })
    await vi.waitFor(() => expect(getUpstreamBalance).toHaveBeenCalledWith(42))
    await vi.waitFor(() => expect(document.body.textContent).toContain('example-relay'))
    expect(document.body.textContent).toContain('12.5 USD')
    expect(document.body.innerHTML).not.toContain('api_key')
    wrapper.unmount()
  })

  it('shows a safe error state when the request fails', async () => {
    getUpstreamBalance.mockRejectedValue(new Error('request failed'))
    const wrapper = mount(UpstreamBalanceModal, { props: { show: true, account }, attachTo: document.body })
    await vi.waitFor(() => expect(document.body.textContent).toContain('request failed'))
    wrapper.unmount()
  })
})
