import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  getUserApiKeys: vi.fn(),
  getAllGroups: vi.fn(),
  listGroups: vi.fn(),
  getUserBalanceHistory: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserApiKeys: apiMocks.getUserApiKeys,
      getUserBalanceHistory: apiMocks.getUserBalanceHistory,
    },
    groups: {
      getAll: apiMocks.getAllGroups,
      list: apiMocks.listGroups,
    },
    apiKeys: {
      updateApiKeyGroup: vi.fn(),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

import UserAllowedGroupsModal from '../UserAllowedGroupsModal.vue'
import UserApiKeysModal from '../UserApiKeysModal.vue'
import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const user = {
  id: 42,
  email: 'user@example.com',
  allowed_groups: [],
  group_rates: {},
  restrict_public_groups: false,
} as any

beforeEach(() => {
  vi.clearAllMocks()
  apiMocks.getUserApiKeys.mockResolvedValue({ items: [], total: 0 })
  apiMocks.getAllGroups.mockResolvedValue([])
  apiMocks.listGroups.mockResolvedValue({ items: [], total: 0 })
  apiMocks.getUserBalanceHistory.mockResolvedValue({ items: [], total: 0, total_recharged: 0 })
})

describe('lazy user dialogs initial load', () => {
  it('can initialize the API keys dialog while closed', async () => {
    mount(UserApiKeysModal, {
      props: { show: false, user },
      global: { stubs: { GroupBadge: true, GroupOptionItem: true } },
    })
    await flushPromises()

    expect(apiMocks.getUserApiKeys).not.toHaveBeenCalled()
    expect(apiMocks.getAllGroups).not.toHaveBeenCalled()
  })

  it('loads API keys when first mounted already open', async () => {
    mount(UserApiKeysModal, {
      props: { show: true, user },
      global: { stubs: { GroupBadge: true, GroupOptionItem: true } },
    })
    await flushPromises()

    expect(apiMocks.getUserApiKeys).toHaveBeenCalledOnce()
    expect(apiMocks.getUserApiKeys).toHaveBeenCalledWith(42)
    expect(apiMocks.getAllGroups).toHaveBeenCalledOnce()
  })

  it('loads allowed groups when first mounted already open', async () => {
    mount(UserAllowedGroupsModal, {
      props: { show: true, user },
      global: { stubs: { PlatformIcon: true } },
    })
    await flushPromises()

    expect(apiMocks.listGroups).toHaveBeenCalledOnce()
    expect(apiMocks.listGroups).toHaveBeenCalledWith(1, 1000)
  })

  it('loads balance history when first mounted already open', async () => {
    mount(UserBalanceHistoryModal, {
      props: { show: true, user },
      global: { stubs: { Icon: true, Select: true } },
    })
    await flushPromises()

    expect(apiMocks.getUserBalanceHistory).toHaveBeenCalledOnce()
    expect(apiMocks.getUserBalanceHistory).toHaveBeenCalledWith(42, 1, 15, undefined)
  })
})
