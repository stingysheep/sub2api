import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import Panel from '../UpstreamProviderProfilesPanel.vue'
import zh from '@/i18n/locales/zh'
import type { UpstreamProviderProfile } from '@/api/admin/settings'
import type { Account } from '@/types'

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { settings: { getUpstreamProviderProfiles: api.get, updateUpstreamProviderProfiles: api.put } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: api.error, showSuccess: api.success }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
  const value = key.split('.').reduce<any>((o, k) => o?.[k], zh)
  return typeof value === 'string' ? value.replace(/\{(\w+)\}/g, (_, k) => String(params?.[k] ?? k)) : key
} }) }))

const original: UpstreamProviderProfile[] = [
  { id: 1, sort_order: 0, name: '站点一', name_prefix: 'one-', base_url: '', enabled: true },
  { id: 2, sort_order: 10, name: '站点二', name_prefix: 'two-', base_url: '', enabled: true },
]
const account = (id: number, profile: number, platform = 'openai') => ({ id, platform, extra: { upstream_provider_profile_id: profile } }) as Account
function make(profiles = original, ready = true, accounts = [account(1,1), account(2,2)]) {
  return mount(Panel, {
    props: { profiles, profilesReady: ready, activeProfileId: 'all', accounts: accounts.slice(0,1), profileAccounts: accounts },
    global: { stubs: {
      BaseDialog: { props: ['show','title'], template: '<section v-if="show" data-testid="editor"><h2>{{ title }}</h2><slot/><slot name="footer"/></section>' },
      Icon: true, PlatformIcon: true,
      VueDraggable: { props: ['modelValue'], template: '<div><slot/></div>' },
    } },
  })
}
const open = async (wrapper: ReturnType<typeof make>) => {
  await wrapper.get('button[title="管理分类"]').trigger('click'); await flushPromises()
}
beforeEach(() => {
  vi.clearAllMocks(); api.get.mockResolvedValue(original)
  api.put.mockImplementation(async profiles => profiles)
})
describe('upstream profile editor safety and navigation', () => {
  it('shows actual labels, all-account totals, and orphan categories without hiding accounts', () => {
    const wrapper = make(original, true, [account(1,1), account(2,2), account(3,9,'anthropic')])
    expect(wrapper.text()).not.toContain('admin.accounts.upstreamProfiles')
    expect(wrapper.get('.profile-nav-item').text()).toContain('全部账号3')
    expect(wrapper.get('[data-testid="missing-profile-9"]').text()).toContain('待恢复分类 #9')
  })
  it('prevents reorder before the initial profile fetch completes', () => {
    const wrapper = make([], false)
    expect(wrapper.get('button[title="调整分类顺序"]').attributes('disabled')).toBeDefined()
  })
  it('refetches the complete profiles before opening even with initially empty props', async () => {
    const wrapper = make([], false)
    await open(wrapper)
    expect(api.get).toHaveBeenCalledOnce()
    const editor = wrapper.get('[data-testid="editor"]')
    const inputs = editor.findAll('input').filter(input => input.attributes('type') !== 'checkbox')
    expect(inputs.map(i => (i.element as HTMLInputElement).value)).toContain('站点二')
    await editor.get('.btn-primary').trigger('click'); await flushPromises()
    expect(api.put).toHaveBeenCalledWith(original, original)
  })
  it('does not allow a failed GET to turn into a destructive empty save', async () => {
    api.get.mockRejectedValueOnce(new Error('offline'))
    const wrapper = make([], false); await open(wrapper)
    expect(wrapper.find('[data-testid="editor"]').exists()).toBe(false)
    expect(api.put).not.toHaveBeenCalled()
    expect(api.error).toHaveBeenCalledWith(zh.admin.accounts.upstreamProfiles.loadFailed)
  })
  it('adds a category without reusing any live account category ID', async () => {
    const wrapper = make(original,true,[account(1,1),account(2,9)])
    await open(wrapper)
    const editor = wrapper.get('[data-testid="editor"]')
    const add = editor.findAll('button').find(b => b.text().includes('新增分类'))!
    await add.trigger('click')
    const inputs = editor.findAll('input').filter(input => input.attributes('type') !== 'checkbox')
    await inputs[6].setValue('新中转站')
    await editor.get('.btn-primary').trigger('click'); await flushPromises()
    expect(api.put.mock.calls[0][0].map((p: UpstreamProviderProfile) => p.id)).toEqual([1,2,10])
    expect(api.put.mock.calls[0][1]).toEqual(original)
  })
  it('retains the draft and explains stale-save conflicts instead of overwriting newer data', async () => {
    api.put.mockRejectedValueOnce({ status:409, reason:'UPSTREAM_PROFILES_STALE', message:'changed' })
    const wrapper = make(); await open(wrapper)
    await wrapper.get('[data-testid="editor"] .btn-primary').trigger('click'); await flushPromises()
    expect(wrapper.find('[data-testid="editor"]').exists()).toBe(true)
    expect(api.error).toHaveBeenCalledWith(zh.admin.accounts.upstreamProfiles.stale)
  })
  it('blocks removing categories with existing accounts', async () => {
    const wrapper = make(); await open(wrapper)
    const buttons = wrapper.findAll('button[title="此分类仍有关联账号，请先调整账号归属。"]')
    expect(buttons).toHaveLength(2)
    expect(buttons.every(b => b.attributes('disabled') !== undefined)).toBe(true)
  })
})
