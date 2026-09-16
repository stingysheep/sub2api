import { reactive, ref } from 'vue'
import {
  operatorAPI,
  type OperatorBalanceHistoryItem,
  type OperatorPaymentOrder,
  type OperatorUsage,
  type OperatorUser,
} from '@/api/operator'
import { extractApiErrorMessage } from '@/utils/apiError'

type Pagination = { page: number; page_size: number; total: number }
type LoadState = { loading: boolean; error: string }
type Translate = (key: string) => string

/** Each callback is fenced by identity, request order, and selection intent. */
export function useOperatorUserReads(
  actorKey: () => string | null,
  t: Translate,
) {
  const users = ref<OperatorUser[]>([])
  const search = ref('')
  const selected = ref<OperatorUser | null>(null)
  const ledger = ref<OperatorBalanceHistoryItem[]>([])
  const orders = ref<OperatorPaymentOrder[]>([])
  const usage = ref<OperatorUsage | null>(null)
  const usersLoading = ref(false)
  const usersError = ref('')
  const ledgerState = reactive<LoadState>({ loading: false, error: '' })
  const ordersState = reactive<LoadState>({ loading: false, error: '' })
  const usageState = reactive<LoadState>({ loading: false, error: '' })
  const usagePeriod = ref<'today' | '7d' | '30d' | '90d'>('30d')
  const pagination = () =>
    reactive<Pagination>({ page: 1, page_size: 20, total: 0 })
  const userPagination = pagination()
  const ledgerPagination = pagination()
  const ordersPagination = pagination()
  let generation = 0
  let selectionIntent = 0
  let userRequest = 0
  const detailRequests = { ledger: 0, orders: 0, usage: 0 }

  function capture() {
    const actor = actorKey()
    const version = generation
    return () =>
      actor !== null && actor === actorKey() && version === generation
  }

  function captureSelection() {
    const current = capture()
    const intent = selectionIntent
    return () => current() && intent === selectionIntent
  }

  function reset() {
    generation += 1
    selectionIntent += 1
    users.value = []
    search.value = ''
    selected.value = null
    ledger.value = []
    orders.value = []
    usage.value = null
    usersLoading.value = false
    usersError.value = ''
    for (const state of [ledgerState, ordersState, usageState])
      Object.assign(state, { loading: false, error: '' })
    for (const page of [userPagination, ledgerPagination, ordersPagination])
      Object.assign(page, { page: 1, total: 0 })
  }

  function paginationFrom(
    response: { total: number; page?: number; page_size?: number },
    target: Pagination,
  ) {
    target.total = response.total || 0
    target.page = response.page || target.page
    target.page_size = response.page_size || target.page_size
  }

  async function loadUsers() {
    if (!actorKey()) return
    const current = capture()
    const request = ++userRequest
    const valid = () => current() && request === userRequest
    usersLoading.value = true
    usersError.value = ''
    try {
      const response = await operatorAPI.listUsers(
        userPagination.page,
        userPagination.page_size,
        search.value.trim() || undefined,
      )
      if (!valid()) return
      users.value = response.items
      paginationFrom(response, userPagination)
    } catch (error) {
      if (valid())
        usersError.value = extractApiErrorMessage(
          error,
          t('operator.users.loadError'),
        )
    } finally {
      if (valid()) usersLoading.value = false
    }
  }

  async function openUser(id: number) {
    if (!actorKey()) return false
    selectionIntent += 1
    const current = captureSelection()
    selected.value = null
    ledger.value = []
    orders.value = []
    usage.value = null
    try {
      const user = await operatorAPI.getUser(id)
      if (!current()) return false
      selected.value = user
      ledgerPagination.page = 1
      ordersPagination.page = 1
      void loadLedger()
      void loadOrders()
      void loadUsage()
      return true
    } catch (error) {
      if (current())
        usersError.value = extractApiErrorMessage(
          error,
          t('operator.users.detailLoadError'),
        )
      return false
    }
  }

  async function loadDetail<T>(
    name: keyof typeof detailRequests,
    state: LoadState,
    request: (id: number) => Promise<T>,
    apply: (response: T) => void,
  ) {
    if (!selected.value || !actorKey()) return
    const id = selected.value.id
    const current = captureSelection()
    const sequence = ++detailRequests[name]
    const valid = () =>
      current() &&
      selected.value?.id === id &&
      sequence === detailRequests[name]
    state.loading = true
    state.error = ''
    try {
      const response = await request(id)
      if (valid()) apply(response)
    } catch (error) {
      if (valid())
        state.error = extractApiErrorMessage(
          error,
          t('operator.users.detailLoadError'),
        )
    } finally {
      if (valid()) state.loading = false
    }
  }

  function loadLedger() {
    return loadDetail(
      'ledger',
      ledgerState,
      (id) =>
        operatorAPI.getBalanceHistory(
          id,
          ledgerPagination.page,
          ledgerPagination.page_size,
        ),
      (response) => {
        ledger.value = response.items
        paginationFrom(response, ledgerPagination)
      },
    )
  }
  function loadOrders() {
    return loadDetail(
      'orders',
      ordersState,
      (id) =>
        operatorAPI.getPaymentOrders(
          id,
          ordersPagination.page,
          ordersPagination.page_size,
        ),
      (response) => {
        orders.value = response.items
        paginationFrom(response, ordersPagination)
      },
    )
  }
  function loadUsage() {
    return loadDetail(
      'usage',
      usageState,
      (id) => operatorAPI.getUsage(id, usagePeriod.value),
      (response) => {
        usage.value = response
      },
    )
  }
  function searchUsers() {
    userPagination.page = 1
    void loadUsers()
  }
  function changeUserPage(page: number) {
    userPagination.page = page
    void loadUsers()
  }
  function changeLedgerPage(page: number) {
    ledgerPagination.page = page
    void loadLedger()
  }
  function changeOrdersPage(page: number) {
    ordersPagination.page = page
    void loadOrders()
  }

  return {
    users,
    search,
    selected,
    ledger,
    orders,
    usage,
    usersLoading,
    usersError,
    ledgerState,
    ordersState,
    usageState,
    usagePeriod,
    userPagination,
    ledgerPagination,
    ordersPagination,
    capture,
    captureSelection,
    reset,
    loadUsers,
    openUser,
    loadLedger,
    loadOrders,
    loadUsage,
    searchUsers,
    changeUserPage,
    changeLedgerPage,
    changeOrdersPage,
  }
}
