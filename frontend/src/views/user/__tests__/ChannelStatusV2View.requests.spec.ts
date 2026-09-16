import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ChannelStatusV2View from '../ChannelStatusV2View.vue'
import type { MonitorFilter } from '@/api/channelMonitorV2'

const mocks = vi.hoisted(() => ({
  getDimensions: vi.fn(), getSnapshot: vi.fn(), getMatrix: vi.fn(),
  getModels: vi.fn(), getErrors: vi.fn(), getUsers: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/channelMonitorV2', () => mocks)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: true }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ replace: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, te: () => false, locale: { value: 'en' } })
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

type Tab = 'models' | 'errors' | 'users'
type ViewState = {
  activeTab: Tab
  filter: MonitorFilter
  modelRows: unknown[]
  errorRows: unknown[]
  userRows: unknown[]
  tabLoading: boolean
}
const mountView = () => shallowMount(ChannelStatusV2View, {
  global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } }
})
const metrics = { error_rate: 0, ttft: { p50_ms: 10, p95_ms: 20 }, tpm: 60, cache_rate: 0, rpm: 1 }
const cases = [
  { tab: 'models', api: 'getModels', rows: 'modelRows', row: { platform: 'openai', model: 'new', metrics } },
  { tab: 'errors', api: 'getErrors', rows: 'errorRows', row: { category: 'new', count: 1, rate: 0 } },
  { tab: 'users', api: 'getUsers', rows: 'userRows', row: { rank: 1, display_label: 'new', metrics } }
] as const

describe('ChannelStatusV2 tab request ownership', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    mocks.getDimensions.mockResolvedValue({ platforms: [], groups: [], models: [] })
    mocks.getSnapshot.mockResolvedValue(null)
    mocks.getMatrix.mockResolvedValue(null)
    for (const name of ['getModels', 'getErrors', 'getUsers'] as const) mocks[name].mockResolvedValue({ items: [] })
  })
  afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

  it('pauses polling while hidden, refreshes on return, and removes the listener on unmount', async () => {
    vi.useFakeTimers()
    let visibility: DocumentVisibilityState = 'visible'
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
    const removeListener = vi.spyOn(document, 'removeEventListener')
    const wrapper = mountView()
    try {
      await flushPromises()
      expect(mocks.getSnapshot).toHaveBeenCalledTimes(1)
      visibility = 'hidden'
      document.dispatchEvent(new Event('visibilitychange'))
      await vi.advanceTimersByTimeAsync(600_000)
      expect(mocks.getSnapshot).toHaveBeenCalledTimes(1)
      visibility = 'visible'
      document.dispatchEvent(new Event('visibilitychange'))
      await flushPromises()
      expect(mocks.getSnapshot).toHaveBeenCalledTimes(2)
      await vi.advanceTimersByTimeAsync(300_000)
      await flushPromises()
      expect(mocks.getSnapshot).toHaveBeenCalledTimes(3)
    } finally { wrapper.unmount() }
    expect(removeListener).toHaveBeenCalledWith('visibilitychange', expect.any(Function))
    document.dispatchEvent(new Event('visibilitychange'))
    await vi.advanceTimersByTimeAsync(600_000)
    expect(mocks.getSnapshot).toHaveBeenCalledTimes(3)
  })

  it.each(cases)('does not let an older $tab response overwrite a new filter', async ({ tab, api, rows, row }) => {
    const wrapper = mountView()
    const vm = wrapper.vm as unknown as ViewState
    try {
      await flushPromises()
      const old = deferred<{ items: unknown[] }>()
      mocks[api].mockReturnValueOnce(old.promise)
      if (tab === 'models') vm.filter.models = ['old']
      else vm.activeTab = tab
      await flushPromises()
      const signal = mocks[api].mock.lastCall?.[2] as AbortSignal | undefined
      mocks[api].mockResolvedValueOnce({ items: [row] })
      vm.filter.models = ['new']
      await flushPromises()
      expect(vm[rows]).toEqual([row])
      old.resolve({ items: [] })
      await flushPromises()
      expect(vm[rows]).toEqual([row])
      expect(signal?.aborted).toBe(true)
    } finally { wrapper.unmount() }
  })

  it('keeps loading owned by the latest request when switching away and back', async () => {
    const wrapper = mountView()
    const vm = wrapper.vm as unknown as ViewState
    try {
      await flushPromises()
      const old = deferred<{ items: unknown[] }>()
      mocks.getErrors.mockReturnValueOnce(old.promise)
      vm.activeTab = 'errors'
      await flushPromises()
      vm.activeTab = 'models'
      await flushPromises()
      const latest = deferred<{ items: unknown[] }>()
      mocks.getErrors.mockReturnValueOnce(latest.promise)
      vm.activeTab = 'errors'
      await flushPromises()
      old.resolve({ items: [] })
      await flushPromises()
      expect(vm.tabLoading).toBe(true)
      latest.resolve({ items: [] })
      await flushPromises()
      expect(vm.tabLoading).toBe(false)
    } finally { wrapper.unmount() }
  })

  it('does not let the parent refresh clear loading for a newer standalone tab request', async () => {
    const old = deferred<{ items: unknown[] }>()
    mocks.getModels.mockReturnValueOnce(old.promise)
    const wrapper = mountView()
    const vm = wrapper.vm as unknown as ViewState
    try {
      await flushPromises()
      const latest = deferred<{ items: unknown[] }>()
      mocks.getErrors.mockReturnValueOnce(latest.promise)
      vm.activeTab = 'errors'
      await flushPromises()
      old.resolve({ items: [] })
      await flushPromises()
      expect(vm.tabLoading).toBe(true)
      latest.resolve({ items: [] })
      await flushPromises()
      expect(vm.tabLoading).toBe(false)
    } finally { wrapper.unmount() }
  })

  it('still reports errors from the current tab request', async () => {
    const wrapper = mountView()
    try {
      await flushPromises()
      mocks.getErrors.mockRejectedValueOnce(new Error('current request failed'))
      ;(wrapper.vm as unknown as ViewState).activeTab = 'errors'
      await flushPromises()
      expect(mocks.showError).toHaveBeenCalledWith('current request failed')
      expect((wrapper.vm as unknown as ViewState).tabLoading).toBe(false)
    } finally { wrapper.unmount() }
  })

  it('ignores stale errors and aborts standalone tab requests on unmount', async () => {
    const wrapper = mountView()
    const vm = wrapper.vm as unknown as ViewState
    await flushPromises()
    const old = deferred<{ items: unknown[] }>()
    mocks.getErrors.mockReturnValueOnce(old.promise)
    vm.activeTab = 'errors'
    await flushPromises()
    const signal = mocks.getErrors.mock.lastCall?.[2] as AbortSignal | undefined
    wrapper.unmount()
    old.reject(new Error('obsolete request failed'))
    await flushPromises()
    expect(mocks.showError).not.toHaveBeenCalled()
    expect(signal?.aborted).toBe(true)
  })
})
