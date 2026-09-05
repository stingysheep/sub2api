<template>
  <div class="flex flex-wrap items-center gap-1" data-testid="monitor-actions">
    <button
      type="button"
      :title="t('admin.channelMonitor.runNow')"
      :aria-label="t('admin.channelMonitor.runNow')"
      @click.stop="$emit('run', row)"
      :disabled="running"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400"
    >
      <Icon name="refresh" size="sm" :class="running ? 'animate-spin' : ''" />
      <span class="text-xs">{{ t('admin.channelMonitor.runNow') }}</span>
    </button>
    <button
      v-if="showToggle"
      type="button"
      data-testid="monitor-toggle"
      :title="toggleTitle"
      :aria-label="toggleTitle"
      @click.stop="$emit('toggle', row)"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors"
      :class="row.enabled ? 'text-emerald-600 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-900/20' : 'text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400'"
    >
      <Icon :name="row.enabled ? 'ban' : 'play'" size="sm" />
      <span class="text-xs">{{ row.enabled ? t('common.inactive') : t('common.active') }}</span>
    </button>
    <button
      type="button"
      data-testid="monitor-duplicate"
      :title="duplicateTitle"
      :disabled="duplicating || Boolean(row.api_key_decrypt_failed)"
      @click.stop="$emit('duplicate', row)"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-dark-700 dark:hover:text-primary-400"
    >
      <Icon name="copy" size="sm" />
      <span class="text-xs">
        {{ duplicating ? t('admin.channelMonitor.duplicating') : t('admin.channelMonitor.duplicate') }}
      </span>
    </button>
    <button
      type="button"
      :title="t('common.edit')"
      :aria-label="t('common.edit')"
      @click.stop="$emit('edit', row)"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
    >
      <Icon name="edit" size="sm" />
      <span class="text-xs">{{ t('common.edit') }}</span>
    </button>
    <button
      type="button"
      :title="t('common.delete')"
      :aria-label="t('common.delete')"
      @click.stop="$emit('delete', row)"
      class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
    >
      <Icon name="trash" size="sm" />
      <span class="text-xs">{{ t('common.delete') }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  row: ChannelMonitor
  running: boolean
  duplicating: boolean
  showToggle?: boolean
}>(), {
  showToggle: false,
})

defineEmits<{
  (e: 'run', row: ChannelMonitor): void
  (e: 'toggle', row: ChannelMonitor): void
  (e: 'duplicate', row: ChannelMonitor): void
  (e: 'edit', row: ChannelMonitor): void
  (e: 'delete', row: ChannelMonitor): void
}>()

const { t } = useI18n()
const duplicateTitle = computed(() => {
  if (props.row.api_key_decrypt_failed) return t('admin.channelMonitor.duplicateKeyUnavailable')
  if (props.duplicating) return t('admin.channelMonitor.duplicating')
  return t('admin.channelMonitor.duplicate')
})
const toggleTitle = computed(() => props.row.enabled ? t('common.inactive') : t('common.active'))
</script>
