import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GroupConcurrencyPanel from '../GroupConcurrencyPanel.vue'
import type { GroupConcurrencySnapshot } from '@/api/admin/dashboard'

const { getGroupConcurrency } = vi.hoisted(() => ({ getGroupConcurrency: vi.fn() }))
vi.mock('@/api/admin/dashboard', () => ({ getGroupConcurrency }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, values?: Record<string, unknown>) => values ? `${key}:${JSON.stringify(values)}` : key }) }))

const snapshot: GroupConcurrencySnapshot = {
  timestamp: '2026-09-13T05:00:00Z',
  current_concurrency: 13,
  active_users: 3,
  group_attributed_slots: 10,
  unattributed_concurrency: 3,
  users: [
    { user_id: 11, user_label: 'a***e', current_in_use: 9 },
    { user_id: 22, user_label: 'b***b', current_in_use: 3 },
    { user_id: 44, user_label: 'd***d', current_in_use: 1 },
  ],
  groups: [
    { group_id: 1, group_name: 'Idle group', platform: 'openai', current_in_use: 0, active_users: 0, users: [] },
    { group_id: 2, group_name: 'Small group', platform: 'openai', current_in_use: 2, active_users: 1, users: [{ user_id: 22, user_label: 'b***b', current_in_use: 2 }] },
    { group_id: 3, group_name: 'Busy group', platform: 'openai', current_in_use: 8, active_users: 2, users: [{ user_id: 11, user_label: 'a***e', current_in_use: 6 }, { user_id: 33, user_label: 'c***c', current_in_use: 2 }] },
  ],
}
const wrappers: ReturnType<typeof mount>[] = []
function render() { const wrapper = mount(GroupConcurrencyPanel); wrappers.push(wrapper); return wrapper }
beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  getGroupConcurrency.mockReset().mockResolvedValue(structuredClone(snapshot))
})
afterEach(() => {
  wrappers.splice(0).forEach(wrapper => wrapper.unmount())
  vi.restoreAllMocks()
  vi.useRealTimers()
})

describe('GroupConcurrencyPanel', () => {
  it('sorts active groups by concurrency and optionally shows idle groups', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="group-concurrency-list"]').findAll(':scope > li').map(row => row.text())).toEqual([
      expect.stringContaining('Busy group'), expect.stringContaining('Small group'),
    ])
    expect(wrapper.text()).not.toContain('Idle group')
    expect(wrapper.text()).toContain('a***e · #11')
    expect(wrapper.text()).toContain('groupConcurrency.total:{"count":13}')
    expect(wrapper.get('[data-testid="active-user-list"]').text()).toContain('groupConcurrency.userConcurrency:{"count":9}')
    expect(wrapper.get('[data-testid="concurrency-attribution-note"]').text()).toContain('groupConcurrency.attribution')
    await wrapper.get('input').setValue(true)
    expect(wrapper.get('[data-testid="group-concurrency-list"]').findAll(':scope > li')[2].text()).toContain('Idle group')
  })
  it('collapses details while retaining the authoritative summary', async () => {
    const wrapper = render()
    await flushPromises()
    const toggle = wrapper.findAll('button').find(button => button.text().includes('groupConcurrency.collapse'))
    expect(toggle).toBeDefined()
    await toggle!.trigger('click')
    expect(wrapper.find('[data-testid="active-user-list"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="group-concurrency-list"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('groupConcurrency.total:{"count":13}')
    expect(wrapper.text()).toContain('groupConcurrency.expand')
  })
  it('updates automatically and removes groups that become idle', async () => {
    const wrapper = render()
    await flushPromises()
    getGroupConcurrency.mockResolvedValue({ ...snapshot, groups: [], users: [], current_concurrency: 0, active_users: 0, group_attributed_slots: 0, unattributed_concurrency: 0 })
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(getGroupConcurrency).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="group-concurrency-list"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('groupConcurrency.empty')
  })
  it('shows errors as unavailable instead of reporting idle', async () => {
    getGroupConcurrency.mockRejectedValue(new Error('offline'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('groupConcurrency.failed')
    expect(wrapper.text()).not.toContain('groupConcurrency.empty')
  })
  it('retains and marks stale data on refresh failure, then recovers', async () => {
    const wrapper = render()
    await flushPromises()
    getGroupConcurrency.mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.text()).toContain('Busy group')
    expect(wrapper.text()).toContain('groupConcurrency.stale')
    await vi.advanceTimersByTimeAsync(5000)
    expect(wrapper.text()).not.toContain('groupConcurrency.stale')
  })
  it('does not overlap requests and cancels on unmount', async () => {
    getGroupConcurrency.mockReturnValue(new Promise(() => {}))
    const wrapper = render()
    await vi.advanceTimersByTimeAsync(15000)
    expect(getGroupConcurrency).toHaveBeenCalledTimes(1)
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    const signal = getGroupConcurrency.mock.calls[0][0] as AbortSignal
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
    await vi.advanceTimersByTimeAsync(10000)
    expect(getGroupConcurrency).toHaveBeenCalledTimes(1)
  })
  it('pauses while hidden and refreshes when visible', async () => {
    render()
    await flushPromises()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(15000)
    expect(getGroupConcurrency).toHaveBeenCalledTimes(1)
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(getGroupConcurrency).toHaveBeenCalledTimes(2)
  })
})
