<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channelMonitor.organization.bulkEditTitle')"
    width="normal"
    @close="handleClose"
  >
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <p class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.channelMonitor.organization.bulkEditHint', { count: selectedCount }) }}
      </p>

      <div class="space-y-3">
        <label class="flex items-start gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <input v-model="enabled.endpoint" data-testid="bulk-endpoint-enabled" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700" />
          <span class="min-w-0 flex-1">
            <span class="block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.channelMonitor.organization.bulkEndpoint') }}</span>
            <input v-model.trim="values.endpoint" data-testid="bulk-endpoint-value" type="url" class="input mt-2" :disabled="!enabled.endpoint" :placeholder="t('admin.channelMonitor.organization.bulkEndpointPlaceholder')" />
          </span>
        </label>

        <label class="flex items-start gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <input v-model="enabled.primaryModel" data-testid="bulk-primary-model-enabled" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700" />
          <span class="min-w-0 flex-1">
            <span class="block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.channelMonitor.organization.bulkPrimaryModel') }}</span>
            <input v-model.trim="values.primaryModel" data-testid="bulk-primary-model-value" type="text" class="input mt-2" :disabled="!enabled.primaryModel" :placeholder="t('admin.channelMonitor.organization.bulkPrimaryModelPlaceholder')" />
          </span>
        </label>

        <div class="grid gap-3 sm:grid-cols-2">
          <label class="flex items-start gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <input v-model="enabled.interval" data-testid="bulk-interval-enabled" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700" />
            <span class="min-w-0 flex-1">
              <span class="block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.channelMonitor.organization.bulkInterval') }}</span>
              <input v-model.number="values.interval" data-testid="bulk-interval-value" type="number" min="15" max="3600" step="1" class="input mt-2" :disabled="!enabled.interval" />
            </span>
          </label>

          <label class="flex items-start gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <input v-model="enabled.jitter" data-testid="bulk-jitter-enabled" type="checkbox" class="mt-0.5 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700" />
            <span class="min-w-0 flex-1">
              <span class="block text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.channelMonitor.organization.bulkJitter') }}</span>
              <input v-model.number="values.jitter" data-testid="bulk-jitter-value" type="number" min="0" max="3585" step="1" class="input mt-2" :disabled="!enabled.jitter" />
            </span>
          </label>
        </div>

        <label class="flex items-center gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <input v-model="enabled.enabled" data-testid="bulk-enabled-enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700" />
          <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.channelMonitor.organization.bulkEnabled') }}</span>
          <select v-model="values.enabled" class="input ml-auto w-auto min-w-28" :disabled="!enabled.enabled">
            <option :value="true">{{ t('common.active') }}</option>
            <option :value="false">{{ t('common.inactive') }}</option>
          </select>
        </label>
      </div>

      <p v-if="errorMessage" class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>

      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="handleClose">{{ t('common.cancel') }}</button>
        <button type="submit" data-testid="bulk-submit" class="btn btn-primary" :disabled="saving || selectedCount < 1">
          {{ saving ? t('admin.channelMonitor.organization.bulkSaving') : t('admin.channelMonitor.organization.bulkSubmit') }}
        </button>
      </div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'

export interface MonitorBulkEditFields {
  endpoint?: string
  primary_model?: string
  interval_seconds?: number
  jitter_seconds?: number
  enabled?: boolean
}

const props = defineProps<{
  show: boolean
  selectedCount: number
  selectedIntervals: number[]
  saving: boolean
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'save', fields: MonitorBulkEditFields): void
}>()

const { t } = useI18n()
const errorMessage = ref('')
const enabled = reactive({
  endpoint: false,
  primaryModel: false,
  interval: false,
  jitter: false,
  enabled: false,
})
const minimumSelectedInterval = computed(() => {
  const validIntervals = props.selectedIntervals.filter((value) => Number.isFinite(value) && value > 0)
  return validIntervals.length > 0 ? Math.min(...validIntervals) : 3600
})

const values = reactive<{
  endpoint: string
  primaryModel: string
  interval: number
  jitter: number
  enabled: boolean
}>({
  endpoint: '',
  primaryModel: '',
  interval: 60,
  jitter: 0,
  enabled: true,
})

function resetForm() {
  enabled.endpoint = false
  enabled.primaryModel = false
  enabled.interval = false
  enabled.jitter = false
  enabled.enabled = false
  values.endpoint = ''
  values.primaryModel = ''
  values.interval = 60
  values.jitter = 0
  values.enabled = true
  errorMessage.value = ''
}

function handleClose() {
  if (props.saving) return
  emit('close')
}

function handleSubmit() {
  errorMessage.value = ''
  if (!Object.values(enabled).some(Boolean)) {
    errorMessage.value = t('admin.channelMonitor.organization.bulkSelectField')
    return
  }
  if (enabled.interval && (!Number.isInteger(values.interval) || values.interval < 15 || values.interval > 3600)) {
    errorMessage.value = t('admin.channelMonitor.organization.bulkIntervalInvalid')
    return
  }
  const effectiveInterval = enabled.interval ? values.interval : minimumSelectedInterval.value
  if (enabled.jitter && (!Number.isInteger(values.jitter) || values.jitter < 0 || values.jitter > effectiveInterval - 15)) {
    errorMessage.value = t('admin.channelMonitor.organization.bulkJitterInvalid')
    return
  }
  const fields: MonitorBulkEditFields = {}
  if (enabled.endpoint) fields.endpoint = values.endpoint.trim()
  if (enabled.primaryModel) fields.primary_model = values.primaryModel.trim()
  if (enabled.interval) fields.interval_seconds = values.interval
  if (enabled.jitter) fields.jitter_seconds = values.jitter
  if (enabled.enabled) fields.enabled = values.enabled
  emit('save', fields)
}

watch(() => props.show, (show) => {
  if (show) resetForm()
})
</script>
