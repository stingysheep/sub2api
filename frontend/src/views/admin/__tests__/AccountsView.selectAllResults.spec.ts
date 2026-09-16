import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'
import { adminAPI } from '@/api/admin'

const {
  listAccounts,
  listWithEtag,
  batchRefresh,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  showError
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  batchRefresh: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      batchDelete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh,
      bulkUpdate: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    },
    settings: { getUpstreamProviderProfiles: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const makeAccounts = (count: number) => Array.from({ length: count }, (_, index) => ({
  id: index + 1,
  name: `account-${index + 1}`,
  platform: 'grok',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  created_at: '2026-07-23T00:00:00Z',
  updated_at: '2026-07-23T00:00:00Z'
}))

const AccountBulkActionsBarStub = {
  props: ['selectedIds', 'totalResults', 'selectingAll', 'allResultsSelected'],
  emits: ['select-all-results', 'select-page', 'clear', 'refresh-token'],
  template: `
    <div>
      <span data-test="selected-count">{{ selectedIds.length }}</span>
      <span data-test="total-results">{{ totalResults }}</span>
      <span data-test="all-results-selected">{{ String(allResultsSelected) }}</span>
      <button data-test="select-page" @click="$emit('select-page')">select page</button>
      <button data-test="select-all-results" @click="$emit('select-all-results')">select all</button>
      <button data-test="clear" @click="$emit('clear')">clear</button>
      <button data-test="refresh-token" @click="$emit('refresh-token')">refresh token</button>
    </div>
  `
}

const AccountTableFiltersStub = {
  emits: ['change', 'update:searchQuery', 'update:filters'],
  template: '<button data-test="change-filter" @click="$emit(\'change\')">change filter</button>'
}

const mountView = () => mount(AccountsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: {
        template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
      },
      DataTable: {
        props: ['data'],
        template: '<div data-test="data-table"><div v-for="row in data" :key="row.id"><slot name="cell-select" :row="row" /></div></div>'
      },
      Pagination: true,
      ConfirmDialog: true,
      AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
      AccountTableFilters: AccountTableFiltersStub,
      UpstreamProviderProfilesPanel: true,
      AccountBulkActionsBar: AccountBulkActionsBarStub,
      AccountActionMenu: true,
      ImportDataModal: true,
      ReAuthAccountModal: true,
      AccountTestModal: true,
      AccountStatsModal: true,
      ScheduledTestsPanel: true,
      SyncFromCrsModal: true,
      TempUnschedStatusModal: true,
      ErrorPassthroughRulesModal: true,
      TLSFingerprintProfilesModal: true,
      CreateAccountModal: true,
      EditAccountModal: true,
      BulkEditAccountModal: true,
      PlatformTypeBadge: true,
      AccountCapacityCell: true,
      AccountStatusIndicator: true,
      AccountTodayStatsCell: true,
      AccountGroupsCell: true,
      AccountUsageCell: true,
      Icon: true
    }
  }
})

describe('admin AccountsView select all filtered results', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset()
    listWithEtag.mockReset()
    batchRefresh.mockReset()
    getBatchTodayStats.mockReset()
    getUpstreamBillingProbeSettings.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    showError.mockReset()

    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
    vi.mocked(adminAPI.settings.getUpstreamProviderProfiles).mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it.each(['search', 'status'])('refreshes category rows and select-all after %s changes', async (field) => {
    vi.useFakeTimers()
    vi.mocked(adminAPI.settings.getUpstreamProviderProfiles).mockResolvedValueOnce([
      { id: 7, name: 'Provider', sort_order: 0 } as never
    ])
    const rows = makeAccounts(2).map(row => ({ ...row, upstream_provider_profile_id: 7 }))
    listAccounts.mockImplementation(async (_page, _size, filters) => ({
      items: filters.search || filters.status ? [rows[1]] : rows,
      total: filters.search || filters.status ? 1 : 2, pages: 1
    }))
    const wrapper = mountView()
    try {
      await flushPromises()
      const panel = wrapper.findComponent({ name: 'UpstreamProviderProfilesPanel' })
      panel.vm.$emit('select', 7)
      await flushPromises()
      const filters = wrapper.getComponent(AccountTableFiltersStub)
      if (field === 'search') filters.vm.$emit('update:searchQuery', 'account-2')
      else {
        filters.vm.$emit('update:filters', { status: 'active' })
        filters.vm.$emit('change')
      }
      await vi.advanceTimersByTimeAsync(350)
      await flushPromises()
      expect(wrapper.findAll('[data-test="data-table"] input')).toHaveLength(1)
      expect(panel.props('profileAccounts').map((row: { id: number }) => row.id)).toEqual([2])
      await wrapper.get('[data-test="select-all-results"]').trigger('click')
      await flushPromises()
      expect(wrapper.getComponent(AccountBulkActionsBarStub).props('selectedIds')).toEqual([2])
    } finally {
      wrapper.unmount()
    }
  })

  it.each([
    { name: 'keeps only failed accounts selected', result: { total: 3, success: 2, failed: 1, errors: [{ account_id: 2, error: 'no refresh token available' }] }, expectedIds: [2] },
    { name: 'clears the selection after every account succeeds', result: { total: 3, success: 3, failed: 0 }, expectedIds: [] },
    { name: 'keeps the original selection when failure details are missing', result: { total: 3, success: 2, failed: 1 }, expectedIds: [1, 2, 3] },
  ])('$name after a batch token refresh and table reload', async ({ result, expectedIds }) => {
    listAccounts.mockResolvedValue({ items: makeAccounts(3), total: 3, page: 1, page_size: 20, pages: 1 })
    batchRefresh.mockResolvedValue(result)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-test="select-page"]').trigger('click')
    await wrapper.get('[data-test="refresh-token"]').trigger('click')
    await flushPromises()

    expect(batchRefresh).toHaveBeenCalledWith([1, 2, 3])
    expect(listAccounts).toHaveBeenCalledTimes(2)
    expect(wrapper.getComponent(AccountBulkActionsBarStub).props('selectedIds')).toEqual(expectedIds)
    expect(wrapper.findAll<HTMLInputElement>('[data-test="data-table"] input').map(input => input.element.checked))
      .toEqual([1, 2, 3].map(id => expectedIds.includes(id)))
    if (result.failed > 0) {
      expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.partialSuccess')
      await wrapper.get('[data-test="refresh-token"]').trigger('click')
      await flushPromises()
      expect(batchRefresh).toHaveBeenLastCalledWith(expectedIds)
    }
    wrapper.unmount()
  })

  it('does not start a pending debounced reload after unmount', async () => {
    vi.useFakeTimers()
    vi.mocked(adminAPI.settings.getUpstreamProviderProfiles).mockResolvedValueOnce([
      { id: 7, name: 'Provider', sort_order: 0 } as never
    ])
    const rows = makeAccounts(2).map(row => ({ ...row, upstream_provider_profile_id: 7 }))
    listAccounts.mockResolvedValue({ items: rows, total: 2, pages: 1 })
    const wrapper = mountView()
    await flushPromises()
    expect(listAccounts).toHaveBeenCalledTimes(2)
    wrapper.getComponent(AccountTableFiltersStub).vm.$emit('update:searchQuery', 'account-2')
    wrapper.unmount()
    listAccounts.mockClear()
    getBatchTodayStats.mockClear()
    await vi.advanceTimersByTimeAsync(350)
    await flushPromises()
    expect(listAccounts).not.toHaveBeenCalled()
    expect(getBatchTodayStats).not.toHaveBeenCalled()
  })

  it('invalidates an in-flight profile result immediately, before the next debounced load', async () => {
    vi.useFakeTimers()
    vi.mocked(adminAPI.settings.getUpstreamProviderProfiles).mockResolvedValueOnce([
      { id: 7, name: 'Provider', sort_order: 0 } as never
    ])
    const rows = makeAccounts(2).map(row => ({ ...row, upstream_provider_profile_id: 7 }))
    let resolveOld!: (value: { items: typeof rows; total: number; pages: number }) => void
    const old = new Promise<{ items: typeof rows; total: number; pages: number }>(resolve => { resolveOld = resolve })
    listAccounts.mockImplementation(async (_page, size, filters) => {
      if (size === 1000 && !filters.search) return old
      return { items: [rows[1]], total: 1, pages: 1 }
    })
    const wrapper = mountView()
    try {
      await flushPromises()
      const panel = wrapper.findComponent({ name: 'UpstreamProviderProfilesPanel' })
      panel.vm.$emit('select', 7)
      wrapper.getComponent(AccountTableFiltersStub).vm.$emit('update:searchQuery', 'account-2')
      resolveOld({ items: rows, total: 2, pages: 2 })
      await flushPromises()
      expect(panel.props('profileAccounts')).toEqual([])
      expect(listAccounts.mock.calls.filter(call => call[0] === 2)).toHaveLength(0)
      await wrapper.get('[data-test="select-all-results"]').trigger('click')
      expect(wrapper.getComponent(AccountBulkActionsBarStub).props('selectedIds')).toEqual([])
      await vi.advanceTimersByTimeAsync(350)
      await flushPromises()
      expect(panel.props('profileAccounts')).toEqual([rows[1]])
    } finally { wrapper.unmount() }
  })

  it('selects all matching IDs in one commit and clears the selection when filters change', async () => {
    const allAccounts = makeAccounts(45)
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => {
      if (pageSize === 1000) {
        return {
          items: allAccounts,
          total: 45,
          page: 1,
          page_size: 1000,
          pages: 1
        }
      }
      return {
        items: allAccounts.slice(0, 20),
        total: 45,
        page: 1,
        page_size: 20,
        pages: 3
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('45')
    expect(wrapper.get('[data-test="total-results"]').text()).toBe('45')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('true')
    expect(listAccounts).toHaveBeenCalledWith(1, 1000, expect.objectContaining({
      lite: '1',
      include_scheduler_score: '0'
    }))

    await wrapper.get('[data-test="change-filter"]').trigger('click')

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('0')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
  })

  it('keeps the original page selection when loading all results fails', async () => {
    const currentPage = makeAccounts(20)
    listAccounts.mockImplementation(async (_page: number, pageSize: number) => {
      if (pageSize === 1000) {
        throw new Error('load all failed')
      }
      return {
        items: currentPage,
        total: 45,
        page: 1,
        page_size: 20,
        pages: 3
      }
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="select-page"]').trigger('click')
    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('20')

    await wrapper.get('[data-test="select-all-results"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="selected-count"]').text()).toBe('20')
    expect(wrapper.get('[data-test="all-results-selected"]').text()).toBe('false')
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkActions.selectAllFailed')
  })
})
