import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import OperatorOverviewView from '../OperatorOverviewView.vue'
const api = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getChannelStatus: vi.fn(),
}))
const auth = await vi.hoisted(async () => {
  const { reactive } = await import('vue')
  return reactive({ user: { id: 71, role: 'operator' as 'operator' | 'user' } })
})
const navigation = vi.hoisted(() => ({ replace: vi.fn() }))
vi.mock('vue-router', () => ({
  useRouter: () => ({ replace: navigation.replace }),
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/api/operator', () => ({ operatorAPI: api }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
const wrappers: ReturnType<typeof shallowMount>[] = []
afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
})
function mount() {
  const wrapper = shallowMount(OperatorOverviewView, {
    global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } },
  })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.resetAllMocks()
  auth.user = { id: 71, role: 'operator' }
  api.listGroups.mockResolvedValue({
    items: [
      {
        id: 1,
        name: 'g',
        platform: 'openai',
        status: 'active',
        model_count: 2,
      },
    ],
  })
  api.getChannelStatus.mockResolvedValue({
    items: [{ id: 1, name: 'c', status: 'operational', latency_ms: 10 }],
  })
})
describe('OperatorOverviewView', () => {
  it('loads read-only group and channel summaries', async () => {
    const wrapper = mount()
    await flushPromises()
    expect(wrapper.text()).toContain('g')
    expect(wrapper.text()).toContain('c')
  })
  it('shows an error when either readonly source fails', async () => {
    api.getChannelStatus.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = mount()
    await flushPromises()
    expect(wrapper.text()).toContain('unavailable')
  })
})

describe('overview authorization response fencing', () => {
  it.each(['resolve', 'reject'] as const)(
    'discards old group and channel responses on revocation: %s',
    async (outcome) => {
      let resolveGroups!: (value: any) => void,
        rejectGroups!: (error: unknown) => void
      let resolveChannels!: (value: any) => void,
        rejectChannels!: (error: unknown) => void
      api.listGroups.mockImplementationOnce(
        () =>
          new Promise((resolve, reject) => {
            resolveGroups = resolve
            rejectGroups = reject
          }),
      )
      api.getChannelStatus.mockImplementationOnce(
        () =>
          new Promise((resolve, reject) => {
            resolveChannels = resolve
            rejectChannels = reject
          }),
      )
      const wrapper = mount()
      auth.user = { id: 71, role: 'user' }
      if (outcome === 'resolve') {
        resolveGroups({ items: [{ id: 99, name: 'private old group' }] })
        resolveChannels({ items: [{ id: 99, name: 'private old channel' }] })
      } else {
        rejectGroups(new Error('private old error'))
        rejectChannels(new Error('private old error'))
      }
      await flushPromises()
      const vm = wrapper.vm as any
      expect(navigation.replace).toHaveBeenCalledWith('/dashboard')
      expect(vm.groups).toEqual([])
      expect(vm.channels).toEqual([])
      expect(vm.groupsError).toBe('')
      expect(vm.channelsError).toBe('')
      expect(vm.loading).toBe(false)
      expect(wrapper.text()).not.toContain('private old')
    },
  )
})
