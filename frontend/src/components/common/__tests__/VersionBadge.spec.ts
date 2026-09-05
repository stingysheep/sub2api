import { reactive } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import VersionBadge from '../VersionBadge.vue'

const appStore = reactive({
  versionLoading: false,
  currentVersion: '0.1.184-shep.8',
  latestVersion: '0.1.185',
  hasUpdate: true,
  releaseInfo: null,
  buildType: 'source',
  fetchVersion: vi.fn(),
  clearVersionCache: vi.fn(),
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({ isAdmin: true }),
  useAppStore: () => appStore,
}))

vi.mock('@/api/admin/system', () => ({
  performUpdate: vi.fn(),
  restartService: vi.fn(),
  getRollbackVersions: vi.fn(),
  rollback: vi.fn(),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => ({
        'version.updateBadge': 'Update',
        'version.updateAvailable': 'A new version is available!',
        'version.upToDate': 'Up to date',
      }[key] ?? key),
    }),
  }
})

describe('VersionBadge', () => {
  beforeEach(() => {
    appStore.currentVersion = '0.1.184-shep.8'
    appStore.latestVersion = '0.1.185'
    appStore.hasUpdate = true
    appStore.fetchVersion.mockClear()
  })

  it('keeps the local version visible and appends an update reminder', () => {
    const wrapper = mount(VersionBadge, {
      global: { stubs: { Icon: true, transition: false } },
    })

    expect(wrapper.text()).toContain('v0.1.184-shep.8')
    expect(wrapper.text()).toContain('Update')
  })

  it('shows only the local version when no newer release exists', () => {
    appStore.latestVersion = '0.1.184'
    appStore.hasUpdate = false
    const wrapper = mount(VersionBadge, {
      global: { stubs: { Icon: true, transition: false } },
    })

    expect(wrapper.text()).toContain('v0.1.184-shep.8')
    expect(wrapper.text()).not.toContain('Update')
  })
})
