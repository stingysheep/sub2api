import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import Panel from '../GroupCategoriesPanel.vue'
import zh from '@/i18n/locales/zh'
import type { AdminGroup } from '@/types'
import type { GroupCategory } from '@/api/admin/groups'

const api = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), error: vi.fn(), success: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getCategories: api.get, updateCategories: api.put } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: api.error, showSuccess: api.success }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
  const value = key.split('.').reduce<any>((o, k) => o?.[k], zh)
  return typeof value === 'string' ? value.replace(/\{(\w+)\}/g, (_, k) => String(params?.[k] ?? k)) : key
} }) }))

const groups = [
  { id: 1, name: 'OpenAI 主组', platform: 'openai', status: 'active' },
  { id: 2, name: 'Claude 主组', platform: 'anthropic', status: 'active' },
] as unknown as AdminGroup[]
const categories: GroupCategory[] = [
  { id: 1, sort_order: 0, name: '生产', group_ids: [1] },
]

function make() {
  return mount(Panel, {
    props: { categories, groups, activeCategoryId: 'all' },
    global: {
      stubs: {
        BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" data-testid="editor"><slot/><slot name="footer"/></section>' },
        Icon: true,
        VueDraggable: { template: '<div><slot/></div>' },
      },
    },
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  api.get.mockResolvedValue(categories)
  api.put.mockImplementation(async (next) => next)
})

describe('GroupCategoriesPanel', () => {
  it('shows all, uncategorized, and custom category counts', () => {
    const wrapper = make()
    expect(wrapper.text()).toContain('全部分组2')
    expect(wrapper.text()).toContain('未分类分组1')
    expect(wrapper.text()).toContain('生产1')
  })

  it('saves a category with an assigned group', async () => {
    const wrapper = make()
    await wrapper.get('button[title="管理分类"]').trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('新增分类'))!.trigger('click')
    const editor = wrapper.get('[data-testid="editor"]')
    const categoryNameInputs = editor.findAll('input:not([type="checkbox"])')
    await categoryNameInputs[1].setValue('测试分类')
    const categoryGroupCheckboxes = editor.findAll('input[type="checkbox"]')
    await categoryGroupCheckboxes[categoryGroupCheckboxes.length - 1].setValue(true)
    await editor.get('.btn-primary').trigger('click')
    await flushPromises()
    expect(api.put).toHaveBeenCalledWith([
      { id: 1, sort_order: 0, name: '生产', group_ids: [1] },
      { id: 2, sort_order: 10, name: '测试分类', group_ids: [2] },
    ], categories)
  })
})
