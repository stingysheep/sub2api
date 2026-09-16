import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn(), isLoading: { value: false } })
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn(), cancelPendingPrefetch: vi.fn(), resetPrefetchState: vi.fn() })
}))
vi.mock('@/api/setup', () => ({ getSetupStatus: vi.fn().mockResolvedValue({ needs_setup: true }) }))
vi.mock('@/api/admin/system', () => ({ checkUpdates: vi.fn(), default: { checkUpdates: vi.fn() }, systemAPI: { checkUpdates: vi.fn() } }))
vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))
vi.mock('@/api', () => ({ authAPI: { getCurrentUser: vi.fn(), logout: vi.fn() }, isTotp2FARequired: () => false }))
const { complianceFetchStatus } = vi.hoisted(() => ({ complianceFetchStatus: vi.fn() }))
vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => ({ initialized: false, fetchStatus: complianceFetchStatus, requireAcknowledgement: vi.fn() })
}))

import router from '@/router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { loadLocaleMessages } from '@/i18n'

describe('actual operator router guard', () => {
  beforeEach(async () => {
    await loadLocaleMessages('en')
    setActivePinia(createPinia())
    const auth = useAuthStore()
    const app = useAppStore()
    auth.token = 'test-operator-token'
    auth.user = {
      id: 7, email: 'operator@example.com', username: 'operator', role: 'operator',
      balance: 0, concurrency: 1, status: 'active', allowed_groups: null,
      balance_notify_enabled: false, balance_notify_threshold: null, balance_notify_extra_emails: [],
      created_at: '', updated_at: ''
    }
    app.cachedPublicSettings = { backend_mode_enabled: true } as any
    app.publicSettingsLoaded = true
    await router.push('/operator/users')
  })

  it('allows both explicitly registered operator pages in backend mode', async () => {
    expect(router.currentRoute.value.path).toBe('/operator/users')
    await router.push('/operator/overview')
    expect(router.currentRoute.value.path).toBe('/operator/overview')
  })

  it('rejects admin pages while preserving backend mode restriction for ordinary user routes', async () => {
    await router.push('/admin/dashboard')
    expect(router.currentRoute.value.path).toBe('/operator/users')
    await router.push('/dashboard')
    expect(router.currentRoute.value.path).toBe('/operator/users')
  })

  it('does not invoke admin compliance checks for an operator scoped route', async () => {
    expect(complianceFetchStatus).not.toHaveBeenCalled()
    await router.push('/operator/overview')
    expect(complianceFetchStatus).not.toHaveBeenCalled()
  })

  it('redirects an authenticated operator away from login in backend mode', async () => {
    await router.push('/login')
    expect(router.currentRoute.value.path).toBe('/operator/users')
  })

  it('denies unknown roles and a demoted or logged out operator', async () => {
    const auth = useAuthStore()
    auth.user = { ...auth.user!, role: 'auditor' as any }
    await router.push('/operator/overview')
    expect(router.currentRoute.value.path).toBe('/login')

    auth.user = { ...auth.user!, role: 'user' }
    await router.push('/operator/users')
    expect(router.currentRoute.value.path).toBe('/login')

    auth.user = { ...auth.user!, role: 'operator' }
    auth.token = null
    await router.push('/operator/overview')
    expect(router.currentRoute.value.path).toBe('/login')
  })
})
