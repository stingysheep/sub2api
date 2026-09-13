import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import GroupConcurrencyPanel from '../GroupConcurrencyPanel.vue'
import type { GroupConcurrencySnapshot } from '@/api/admin/dashboard'

const { getGroupConcurrency } = vi.hoisted(() => ({ getGroupConcurrency: vi.fn() }))
vi.mock('@/api/admin/dashboard', () => ({ getGroupConcurrency }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const snapshot: GroupConcurrencySnapshot = {
  timestamp: '2026-09-13T05:00:00Z',
  groups: [
    { group_id: 1, group_name: 'Idle group', platform: 'openai', current_in_use: 0 },
    { group_id: 2, group_name: 'Small group', platform: 'openai', current_in_use: 2 },
    { group_id: 3, group_name: 'Busy group', platform: 'openai', current_in_use: 8 },
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
    expect(wrapper.findAll('li').map(row => row.text())).toEqual([
      expect.stringContaining('Busy group'), expect.stringContaining('Small group'),
    ])
    expect(wrapper.text()).not.toContain('Idle group')
    await wrapper.get('input').setValue(true)
    expect(wrapper.findAll('li')[2].text()).toContain('Idle group')
  })
  it('updates automatically and removes groups that become idle', async () => {
    const wrapper = render()
    await flushPromises()
    getGroupConcurrency.mockResolvedValue({ ...snapshot, groups: [] })
    await vi.advanceTimersByTimeAsync(5000)
    await flushPromises()
    expect(getGroupConcurrency).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('li')).toHaveLength(0)
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
