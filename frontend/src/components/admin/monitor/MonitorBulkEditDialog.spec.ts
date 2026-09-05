import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import MonitorBulkEditDialog from '@/components/admin/monitor/MonitorBulkEditDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  emits: ['close'],
  template: '<div v-if="show"><slot /></div>',
}

function mountDialog(selectedIntervals = [60, 120]) {
  return mount(MonitorBulkEditDialog, {
    props: {
      show: true,
      selectedCount: selectedIntervals.length,
      selectedIntervals,
      saving: false,
    },
    global: {
      stubs: { BaseDialog: BaseDialogStub },
    },
  })
}

describe('MonitorBulkEditDialog', () => {
  it('emits only fields explicitly selected by the administrator', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-testid="bulk-endpoint-enabled"]').setValue(true)
    await wrapper.get('[data-testid="bulk-endpoint-value"]').setValue('https://api.example.com')
    await wrapper.get('[data-testid="bulk-interval-enabled"]').setValue(true)
    await wrapper.get('[data-testid="bulk-interval-value"]').setValue(90)
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toEqual([[
      {
        endpoint: 'https://api.example.com',
        interval_seconds: 90,
      },
    ]])
  })

  it('requires at least one field to be enabled', async () => {
    const wrapper = mountDialog()

    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.text()).toContain('admin.channelMonitor.organization.bulkSelectField')
  })

  it('validates jitter against the smallest selected monitor interval', async () => {
    const wrapper = mountDialog([30, 90])

    await wrapper.get('[data-testid="bulk-jitter-enabled"]').setValue(true)
    await wrapper.get('[data-testid="bulk-jitter-value"]').setValue(20)
    await wrapper.get('form').trigger('submit')

    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.text()).toContain('admin.channelMonitor.organization.bulkJitterInvalid')
  })
})
