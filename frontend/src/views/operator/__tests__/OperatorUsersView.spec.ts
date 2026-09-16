import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import OperatorUsersView from '../OperatorUsersView.vue'

const api = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getUser: vi.fn(),
  getBalanceHistory: vi.fn(),
  getPaymentOrders: vi.fn(),
  getUsage: vi.fn(),
  adjustBalance: vi.fn(),
}))
const navigation = vi.hoisted(() => ({
  guard: undefined as undefined | (() => boolean),
  replace: vi.fn(),
}))
const auth = await vi.hoisted(async () => {
  const { reactive } = await import('vue')
  return reactive({
    user: { id: 71, role: 'operator' as 'operator' | 'admin' | 'user' },
  })
})
const language = await vi.hoisted(async () => {
  const { ref } = await import('vue')
  return { locale: ref('en') }
})
vi.mock('vue-router', () => ({
  useRouter: () => ({ replace: navigation.replace }),
  onBeforeRouteLeave: (guard: () => boolean) => {
    navigation.guard = guard
  },
}))
vi.mock('@/api/operator', () => ({ operatorAPI: api }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: language.locale }),
  }
})

const user = (id = 1) => ({
  id,
  email: `u${id}@x`,
  username: `u${id}`,
  status: 'active',
  balance: 10,
  free_balance: 4,
  paid_balance: 6,
})
const adjustmentResult = (cache_synced = true) => ({
  before_balance: '10',
  after_balance: '11.25',
  cache_synced,
  operation_id: 'op-1',
  operation: 'add',
  amount: '1.25',
  source: 'free',
  before_free_balance: '4',
  after_free_balance: '5.25',
  before_paid_balance: '6',
  after_paid_balance: '6',
  free_amount: '1.25',
  paid_amount: '0',
  replayed: false,
})

const wrappers: ReturnType<typeof shallowMount>[] = []
afterEach(() => {
  for (const wrapper of wrappers.splice(0)) wrapper.unmount()
})
function mount() {
  const wrapper = shallowMount(OperatorUsersView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Pagination: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}
function defaults() {
  api.listUsers.mockResolvedValue({
    items: [user()],
    total: 41,
    page: 1,
    page_size: 20,
  })
  api.getUser.mockImplementation(async (id: number) => user(id))
  api.getBalanceHistory.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
  })
  api.getPaymentOrders.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
  })
  api.getUsage.mockResolvedValue({
    period: '30d',
    request_count: 0,
    total_tokens: 0,
    usage_amount: '0',
    start_at: 's',
    end_at: 'e',
  })
  api.adjustBalance.mockResolvedValue(adjustmentResult())
}
beforeEach(() => {
  vi.resetAllMocks()
  localStorage.clear()
  language.locale.value = 'en'
  auth.user = { id: 71, role: 'operator' }
  defaults()
})

describe('OperatorUsersView', () => {
  it('warns before leaving a pending operation and preserves its key when navigation is cancelled', async () => {
    api.adjustBalance.mockRejectedValueOnce({ status: 0 })
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    expect(navigation.guard?.()).toBe(true)
    expect(confirm).not.toHaveBeenCalled()
    await vm.openUser(1)
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    await vm.adjust()
    const key = vm.pendingAdjustment.idempotencyKey
    expect(navigation.guard?.()).toBe(false)
    expect(vm.pendingAdjustment.idempotencyKey).toBe(key)
    expect(confirm).toHaveBeenCalledWith('operator.users.leavePendingWarning')
    confirm.mockReturnValue(true)
    expect(navigation.guard?.()).toBe(true)
    expect(vm.pendingAdjustment.idempotencyKey).toBe(key)
    confirm.mockRestore()
    wrapper.unmount()
  })
  it('only blocks browser unload while pending or saving and removes its listener on unmount', async () => {
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    const event = () => ({ preventDefault: vi.fn(), returnValue: 'untouched' })
    const initial = event()
    vm.warnBeforeUnload(initial)
    expect(initial.preventDefault).not.toHaveBeenCalled()
    expect(initial.returnValue).toBe('untouched')
    vm.saving = true
    const saving = event()
    vm.warnBeforeUnload(saving)
    expect(saving.preventDefault).toHaveBeenCalled()
    expect(saving.returnValue).toBe('')
    vm.saving = false
    api.adjustBalance.mockRejectedValueOnce({ status: 0 })
    await vm.openUser(1)
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    await vm.adjust()
    const pending = event()
    vm.warnBeforeUnload(pending)
    expect(pending.preventDefault).toHaveBeenCalled()
    expect(pending.returnValue).toBe('')
    const remove = vi.spyOn(window, 'removeEventListener')
    wrapper.unmount()
    expect(remove).toHaveBeenCalledWith('beforeunload', expect.any(Function))
    remove.mockRestore()
  })
  it.each([400, 422])(
    'releases a new operation rejected with %s so its amount can be corrected',
    async (status) => {
      api.adjustBalance.mockRejectedValueOnce({
        status,
        message: 'invalid amount',
      })
      const wrapper = mount()
      await flushPromises()
      const vm = wrapper.vm as any
      await vm.openUser(1)
      vm.activeTab = 'adjust'
      vm.adjustment.amount = '1'
      vm.adjustment.reason = 'support'
      await vm.adjust()
      await flushPromises()
      expect(vm.pendingAdjustment).toBeNull()
      expect(
        wrapper.find('form.grid input').attributes('disabled'),
      ).toBeUndefined()
      vm.adjustment.amount = '2'
      await vm.adjust()
      expect(api.adjustBalance.mock.calls[1][1].amount).toBe('2')
      expect(api.adjustBalance.mock.calls[1][2]).not.toBe(
        api.adjustBalance.mock.calls[0][2],
      )
    },
  )
  it.each([403, 429])(
    'retains an ambiguous first operation after retry status %s',
    async (status) => {
      api.adjustBalance
        .mockRejectedValueOnce({ status: 0 })
        .mockRejectedValueOnce({ status, message: 'rejected retry' })
      const wrapper = mount()
      await flushPromises()
      const vm = wrapper.vm as any
      await vm.openUser(1)
      vm.adjustment.amount = '1'
      vm.adjustment.reason = 'support'
      await vm.adjust()
      vm.adjustment.amount = '2'
      await vm.adjust()
      expect(vm.pendingAdjustment).not.toBeNull()
      await vm.adjust()
      expect(api.adjustBalance.mock.calls[1]).toEqual(
        api.adjustBalance.mock.calls[0],
      )
      expect(api.adjustBalance.mock.calls[2]).toEqual(
        api.adjustBalance.mock.calls[0],
      )
    },
  )
  it('retains an already committed unsynced operation after a rejected retry', async () => {
    api.adjustBalance
      .mockResolvedValueOnce(adjustmentResult(false))
      .mockRejectedValueOnce({ status: 400, message: 'retry rejected' })
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openUser(1)
    vm.activeTab = 'adjust'
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    await vm.adjust()
    await vm.adjust()
    await flushPromises()
    expect(vm.pendingAdjustment.cacheSynced).toBe(false)
    expect(wrapper.text()).toContain('operator.users.cachePending')
    await vm.adjust()
    expect(api.adjustBalance.mock.calls[2]).toEqual(
      api.adjustBalance.mock.calls[0],
    )
  })
  it('keeps a new conflicting operation frozen and displays the server conflict', async () => {
    api.adjustBalance.mockRejectedValueOnce({
      status: 409,
      message: 'Idempotency key conflict',
    })
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openUser(1)
    vm.activeTab = 'adjust'
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    await vm.adjust()
    await flushPromises()
    expect(vm.pendingAdjustment).not.toBeNull()
    expect(wrapper.text()).toContain('Idempotency key conflict')
    await vm.adjust()
    expect(api.adjustBalance.mock.calls[1]).toEqual(
      api.adjustBalance.mock.calls[0],
    )
  })
  it('validates the UTF-8 reason byte limit rather than character count', async () => {
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openUser(1)
    vm.adjustment.amount = '1'
    vm.adjustment.reason = '中'.repeat(334)
    await vm.adjust()
    expect(api.adjustBalance).not.toHaveBeenCalled()
    expect(vm.adjustmentError).toBe('operator.users.invalidAdjustment')
    vm.adjustment.reason = '中'.repeat(333) + 'a'
    await vm.adjust()
    expect(api.adjustBalance).toHaveBeenCalledTimes(1)
  })
  it('retries a network-ambiguous adjustment with exact frozen target, body, and key', async () => {
    api.adjustBalance
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(adjustmentResult())
    const wrapper = mount()
    await flushPromises()
    await (wrapper.vm as any).openUser(1)
    const vm = wrapper.vm as any
    vm.activeTab = 'adjust'
    vm.adjustment.amount = '1.25'
    vm.adjustment.reason = 'support'
    await vm.adjust()
    vm.adjustment.amount = '9.99'
    vm.adjustment.reason = 'edited after failure'
    await vm.adjust()
    expect(api.adjustBalance.mock.calls[0]).toEqual([
      1,
      { operation: 'add', amount: '1.25', source: 'free', reason: 'support' },
      expect.any(String),
    ])
    expect(api.adjustBalance.mock.calls[1]).toEqual(
      api.adjustBalance.mock.calls[0],
    )
  })
  it('does not persist the amount, source, or free-form reason for an ambiguous request', async () => {
    api.adjustBalance.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount()
    await flushPromises()
    await (wrapper.vm as any).openUser(1)
    const vm = wrapper.vm as any
    vm.activeTab = 'adjust'
    vm.adjustment.amount = '1.25000000'
    vm.adjustment.reason = 'private support context'
    await vm.adjust()
    const saved =
      localStorage.getItem('operator.pending-adjustment.v1.operator.71') || ''
    expect(saved).toContain('idempotencyKey')
    expect(saved).not.toContain('1.25000000')
    expect(saved).not.toContain('private support context')
    expect(saved).not.toContain('source')
    wrapper.unmount()
  })
  it('blocks a new adjustment after refresh until the operator acknowledges ledger review', async () => {
    localStorage.setItem(
      'operator.pending-adjustment.v1.operator.71',
      JSON.stringify({
        actorId: 71,
        role: 'operator',
        targetId: 1,
        idempotencyKey: 'existing-key',
        createdAt: '2026-01-01T00:00:00.000Z',
      }),
    )
    const wrapper = mount()
    await flushPromises()
    await (wrapper.vm as any).openUser(1)
    const vm = wrapper.vm as any
    vm.activeTab = 'adjust'
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'new request'
    await vm.adjust()
    expect(api.adjustBalance).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('operator.users.recoveredPending')
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
    vm.acknowledgeRecoveredPending()
    await vm.adjust()
    expect(api.adjustBalance).toHaveBeenCalledTimes(1)
    confirm.mockRestore()
    wrapper.unmount()
  })
  it('isolates a recovered pending record when the active actor changes', async () => {
    localStorage.setItem(
      'operator.pending-adjustment.v1.operator.71',
      JSON.stringify({
        actorId: 71,
        role: 'operator',
        targetId: 1,
        idempotencyKey: 'existing-key',
        createdAt: '2026-01-01T00:00:00.000Z',
      }),
    )
    const wrapper = mount()
    await flushPromises()
    expect((wrapper.vm as any).recoveredPending.targetId).toBe(1)
    auth.user = { id: 72, role: 'operator' }
    ;(wrapper.vm as any).restorePending()
    await wrapper.vm.$nextTick()
    expect((wrapper.vm as any).recoveredPending).toBeNull()
    wrapper.unmount()
  })
  it('keeps cache warning and same key after a submitted response reports unsynced cache', async () => {
    api.adjustBalance.mockResolvedValue(adjustmentResult(false))
    const wrapper = mount()
    await flushPromises()
    await (wrapper.vm as any).openUser(1)
    const vm = wrapper.vm as any
    vm.activeTab = 'adjust'
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    await vm.adjust()
    await flushPromises()
    expect(wrapper.text()).toContain('operator.users.cachePending')
    await vm.adjust()
    expect(api.adjustBalance.mock.calls[1]).toEqual(
      api.adjustBalance.mock.calls[0],
    )
    expect(wrapper.text()).toContain('operator.users.cachePending')
  })
  it('freezes the submitted form and prevents a second DOM submit while the request is in flight', async () => {
    let resolveAdjustment: (
      value: ReturnType<typeof adjustmentResult>,
    ) => void = () => undefined
    api.adjustBalance.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveAdjustment = resolve
        }),
    )
    const wrapper = mount()
    await flushPromises()
    await (wrapper.vm as any).openUser(1)
    const vm = wrapper.vm as any
    vm.activeTab = 'adjust'
    await wrapper.vm.$nextTick()
    const form = wrapper.find('form.grid')
    const inputs = form.findAll('input')
    await inputs[0].setValue('1.25')
    await form.find('textarea').setValue('support')
    await form.trigger('submit')
    await form.trigger('submit')
    expect(api.adjustBalance).toHaveBeenCalledTimes(1)
    expect(inputs[0].attributes('disabled')).toBeDefined()
    resolveAdjustment(adjustmentResult(false))
    await flushPromises()
    expect(wrapper.text()).toContain('operator.users.cachePending')
  })
  it('shows a load error without leaking an unhandled rejection', async () => {
    api.listUsers.mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount()
    await flushPromises()
    expect(wrapper.text()).toContain('offline')
  })
  it('loads later user, ledger, and order pages', async () => {
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    vm.changeUserPage(2)
    await flushPromises()
    expect(api.listUsers).toHaveBeenLastCalledWith(2, 20, undefined)
    await vm.openUser(1)
    vm.changeLedgerPage(2)
    await flushPromises()
    expect(api.getBalanceHistory).toHaveBeenLastCalledWith(1, 2, 20)
    vm.changeOrdersPage(2)
    await flushPromises()
    expect(api.getPaymentOrders).toHaveBeenLastCalledWith(1, 2, 20)
  })
  it('does not let a stale user-detail response overwrite a newer selection', async () => {
    let resolveFirst: (value: ReturnType<typeof user>) => void = () => undefined
    api.getUser
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveFirst = resolve
          }),
      )
      .mockResolvedValueOnce(user(2))
    const wrapper = mount()
    await flushPromises()
    const first = (wrapper.vm as any).openUser(1)
    const second = (wrapper.vm as any).openUser(2)
    await second
    resolveFirst(user(1))
    await first
    expect((wrapper.vm as any).selected.id).toBe(2)
  })
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((yes, no) => {
    resolve = yes
    reject = no
  })
  return { promise, resolve, reject }
}

describe('operator asynchronous identity isolation', () => {
  it.each(['resolve', 'reject'] as const)(
    'fences a pending user list after actor change: %s',
    async (outcome) => {
      const delayed = deferred<any>()
      api.listUsers.mockImplementationOnce(() => delayed.promise)
      const wrapper = mount()
      auth.user = { id: 72, role: 'operator' }
      await flushPromises()
      expect((wrapper.vm as any).users[0].id).toBe(1)
      if (outcome === 'resolve')
        delayed.resolve({
          items: [user(99)],
          total: 99,
          page: 5,
          page_size: 20,
        })
      else delayed.reject(new Error('previous actor private error'))
      await flushPromises()
      expect((wrapper.vm as any).users[0].id).toBe(1)
      expect((wrapper.vm as any).userPagination.page).toBe(1)
      expect(wrapper.text()).not.toContain('previous actor private error')
    },
  )

  it('fences actor A responses even after switching A to B to A', async () => {
    const delayed = deferred<any>()
    const wrapper = mount()
    await flushPromises()
    api.getUser.mockImplementationOnce(() => delayed.promise)
    const request = (wrapper.vm as any).openUser(99)
    auth.user = { id: 72, role: 'operator' }
    auth.user = { id: 71, role: 'operator' }
    delayed.resolve(user(99))
    await request
    expect((wrapper.vm as any).selected).toBeNull()
  })

  it.each(['resolve', 'reject'] as const)(
    'fences all detail callbacks on role revocation: %s',
    async (outcome) => {
      const ledger = deferred<any>(),
        orders = deferred<any>(),
        usage = deferred<any>()
      api.getBalanceHistory.mockImplementationOnce(() => ledger.promise)
      api.getPaymentOrders.mockImplementationOnce(() => orders.promise)
      api.getUsage.mockImplementationOnce(() => usage.promise)
      const wrapper = mount()
      await flushPromises()
      await (wrapper.vm as any).openUser(1)
      auth.user = { id: 71, role: 'user' }
      const vm = wrapper.vm as any
      if (outcome === 'resolve') {
        ledger.resolve({ items: [{ id: 99, reason: 'old ledger' }], total: 99 })
        orders.resolve({ items: [{ id: 99 }], total: 99 })
        usage.resolve({ period: 'private old usage' })
      } else {
        for (const request of [ledger, orders, usage])
          request.reject(new Error('private old detail error'))
      }
      await flushPromises()
      expect(navigation.replace).toHaveBeenCalledWith('/dashboard')
      expect(vm.users).toEqual([])
      expect(vm.selected).toBeNull()
      expect(vm.ledger).toEqual([])
      expect(vm.orders).toEqual([])
      expect(vm.usage).toBeNull()
      for (const state of [vm.ledgerState, vm.ordersState, vm.usageState])
        expect(state).toEqual({ loading: false, error: '' })
      expect(vm.lastAdjustment).toBeNull()
      expect(vm.pendingAdjustment).toBeNull()
      expect(wrapper.text()).not.toContain('private old')
    },
  )

  it.each(['resolve', 'reject'] as const)(
    'preserves old actor recovery but fences pending POST callbacks: %s',
    async (outcome) => {
      const delayed = deferred<any>()
      api.adjustBalance.mockImplementationOnce(() => delayed.promise)
      const wrapper = mount()
      await flushPromises()
      const vm = wrapper.vm as any
      await vm.openUser(1)
      vm.adjustment.amount = '1.25'
      vm.adjustment.reason = 'private old actor reason'
      const request = vm.adjust()
      const saved = localStorage.getItem(
        'operator.pending-adjustment.v1.operator.71',
      )
      auth.user = { id: 72, role: 'operator' }
      await vm.openUser(2)
      if (outcome === 'resolve') delayed.resolve(adjustmentResult())
      else delayed.reject({ status: 400, message: 'private old POST error' })
      await request
      await flushPromises()
      expect(vm.selected.id).toBe(2)
      expect(vm.lastAdjustment).toBeNull()
      expect(vm.pendingAdjustment).toBeNull()
      expect(vm.recoveredPending).toBeNull()
      expect(vm.saving).toBe(false)
      expect(vm.adjustment.reason).toBe('')
      expect(vm.adjustmentError).toBe('')
      expect(
        localStorage.getItem('operator.pending-adjustment.v1.operator.71'),
      ).toBe(saved)
      auth.user = { id: 71, role: 'operator' }
      expect(vm.recoveredPending.targetId).toBe(1)
    },
  )

  it('does not reopen the old target when a successful POST resolves after selecting another user', async () => {
    const delayed = deferred<any>()
    api.adjustBalance.mockImplementationOnce(() => delayed.promise)
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openUser(1)
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    const request = vm.adjust()
    await vm.openUser(2)
    delayed.resolve(adjustmentResult())
    await request
    expect(vm.selected.id).toBe(2)
    expect(vm.lastAdjustment).toBeNull()
    expect(api.getUser.mock.calls).toEqual([[1], [2]])
    expect(
      localStorage.getItem('operator.pending-adjustment.v1.operator.71'),
    ).not.toBeNull()
    expect(vm.pendingAdjustment.targetId).toBe(1)
    await vm.adjust()
    expect(api.adjustBalance).toHaveBeenCalledTimes(1)
  })

  it('fences the post-commit user refresh after a role change', async () => {
    const delayed = deferred<any>()
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    await vm.openUser(1)
    vm.adjustment.amount = '1'
    vm.adjustment.reason = 'support'
    api.getUser.mockImplementationOnce(() => delayed.promise)
    const request = vm.adjust()
    await flushPromises()
    auth.user = { id: 71, role: 'user' }
    delayed.resolve(user(1))
    await request
    expect(vm.selected).toBeNull()
    expect(vm.lastAdjustment).toBeNull()
    expect(vm.saving).toBe(false)
  })

  it('localizes status and uses the chosen locale for exact decimals and dates', async () => {
    const wrapper = mount()
    await flushPromises()
    const vm = wrapper.vm as any
    expect(wrapper.text()).toContain('operator.users.statuses.active')
    expect(vm.formatAmount('999999999999.12345678')).toBe(
      '999,999,999,999.12345678',
    )
    language.locale.value = 'de'
    expect(vm.formatAmount('999999999999.12345678')).toBe(
      '999.999.999.999,12345678',
    )
    const time = '2026-01-02T03:04:05Z'
    expect(vm.formatDateTime(time)).toBe(
      new Intl.DateTimeFormat('de', {
        dateStyle: 'medium',
        timeStyle: 'medium',
      }).format(new Date(time)),
    )
    vm.adjustment.amount = '0.00000000'
    vm.adjustment.reason = 'support'
    await vm.openUser(1)
    await vm.adjust()
    expect(api.adjustBalance).not.toHaveBeenCalled()
    vm.adjustment.amount = '0.00000001'
    await vm.adjust()
    expect(api.adjustBalance.mock.calls[0][1].amount).toBe('0.00000001')
  })
})

it('requires ledger acknowledgement for the recovered target rather than another selected user', async () => {
  localStorage.setItem(
    'operator.pending-adjustment.v1.operator.71',
    JSON.stringify({
      actorId: 71,
      role: 'operator',
      targetId: 1,
      idempotencyKey: 'existing',
      createdAt: '2026-01-01T00:00:00Z',
    }),
  )
  const wrapper = mount()
  await flushPromises()
  const vm = wrapper.vm as any
  await vm.openUser(2)
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(true)
  vm.acknowledgeRecoveredPending()
  expect(confirm).not.toHaveBeenCalled()
  expect(vm.recoveredPending.targetId).toBe(1)
  await vm.openUser(1)
  vm.acknowledgeRecoveredPending()
  expect(vm.recoveredPending).toBeNull()
  confirm.mockRestore()
})

it('keeps a confirmed adjustment result if its subsequent detail refresh fails', async () => {
  const wrapper = mount()
  await flushPromises()
  const vm = wrapper.vm as any
  await vm.openUser(1)
  vm.adjustment.amount = '1'
  vm.adjustment.reason = 'support'
  api.getUser.mockRejectedValueOnce(new Error('detail temporarily unavailable'))
  await vm.adjust()
  expect(vm.lastAdjustment.after_balance).toBe('11.25')
  expect(vm.pendingAdjustment).toBeNull()
  expect(vm.adjustmentError).toBe('')
  expect(vm.usersError).toBe('detail temporarily unavailable')
})
