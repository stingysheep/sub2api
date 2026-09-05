<template>
  <section class="space-y-4" data-testid="monitor-organization-panel">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h2 class="text-base font-bold text-gray-900 dark:text-white">
          {{ t('admin.channelMonitor.organization.title') }}
        </h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.channelMonitor.organization.description') }}
        </p>
      </div>
      <div class="flex items-center gap-2">
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
        <button type="button" class="btn btn-primary btn-sm" @click="openCreateGroup">
          <Icon name="plus" size="sm" />
          {{ t('admin.channelMonitor.organization.createGroup') }}
        </button>
      </div>
    </header>

    <div v-if="monitors.length" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-primary-200 bg-primary-50/60 px-3 py-2.5 dark:border-primary-800/50 dark:bg-primary-950/20">
      <span class="text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('admin.channelMonitor.organization.bulkSelectedCount', { count: selectedMonitorIds.length }) }}</span>
      <div class="flex items-center gap-2">
        <button type="button" class="btn btn-primary btn-sm" :disabled="bulkSaving || selectedMonitorIds.length === 0" @click="showBulkEditDialog = true">{{ t('admin.channelMonitor.organization.bulkEdit') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="bulkSaving" @click="selectAllMonitors">{{ t('admin.channelMonitor.organization.selectAllMonitors') }}</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="bulkSaving || selectedMonitorIds.length === 0" @click="clearSelection">{{ t('admin.channelMonitor.organization.clearSelection') }}</button>
      </div>
    </div>

    <div v-if="loading" class="rounded-xl border border-gray-200 bg-white px-4 py-8 text-center text-sm text-gray-400 dark:border-dark-700 dark:bg-dark-800">
      {{ t('admin.channelMonitor.organization.loading') }}
    </div>
    <div v-else-if="!sortedGroups.length && !ungroupedMonitors.length" class="rounded-xl border border-dashed border-gray-300 bg-white px-4 py-8 text-center text-sm text-gray-400 dark:border-dark-700 dark:bg-dark-800">
      {{ t('admin.channelMonitor.organization.empty') }}
    </div>

    <div v-else class="space-y-5">
      <section v-if="sortedGroups.length" class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3">
          <div>
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.channelMonitor.organization.createdGroups') }}</h3>
          </div>
          <span class="text-xs text-gray-400">{{ sortedGroups.length }}</span>
        </div>
        <VueDraggable
          v-model="orderedGroups"
          class="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-3"
          :animation="180"
          :disabled="saving"
          handle=".monitor-group-drag-handle"
          data-testid="monitor-group-order-list"
          @end="onGroupDragEnd"
        >
          <div
            v-for="group in orderedGroups"
            :key="group.id"
            class="monitor-group-card flex min-w-0 cursor-grab items-center gap-2 rounded-lg border border-gray-200 bg-gray-50 px-2.5 py-2 transition-colors hover:border-blue-300 active:cursor-grabbing dark:border-dark-600 dark:bg-dark-900/30 dark:hover:border-blue-700"
          >
            <Icon name="menu" size="sm" class="monitor-group-drag-handle shrink-0 cursor-grab text-gray-400 active:cursor-grabbing" />
            <span class="min-w-0 flex-1 truncate text-xs font-medium text-gray-800 dark:text-gray-100">{{ group.name }}</span>
            <span class="shrink-0 text-[11px] text-gray-400">{{ monitorsFor(group.id).length }}</span>
          </div>
        </VueDraggable>
      </section>

      <section v-for="group in sortedGroups" :key="group.id" class="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 pb-3 dark:border-dark-700">
          <div class="flex min-w-0 items-center gap-2">
            <input
              type="checkbox"
              class="h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
              :checked="isGroupFullySelected(group.id)"
              :indeterminate="isGroupPartiallySelected(group.id)"
              :aria-label="t('admin.channelMonitor.organization.selectGroup')"
              @click.stop
              @change="toggleGroupSelection(group.id)"
            />
            <span class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-900/30 dark:text-blue-300">
              <Icon name="grid" size="sm" />
            </span>
            <div class="min-w-0">
              <h3 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ group.name }}</h3>
              <p class="text-xs text-gray-400">{{ monitorsFor(group.id).length }} {{ t('admin.channelMonitor.organization.monitors') }}</p>
            </div>
          </div>
          <div class="flex items-center gap-1">
            <button type="button" class="icon-btn" :disabled="saving" :title="t('common.edit')" @click="openEditGroup(group)"><Icon name="edit" size="sm" /></button>
            <button type="button" class="icon-btn text-red-500 hover:bg-red-50" :disabled="saving" :title="t('common.delete')" @click="removeGroup(group)"><Icon name="trash" size="sm" /></button>
          </div>
        </div>

        <VueDraggable
          v-model="monitorLists[monitorListKey(group.id)]"
          class="mt-4 flex min-h-24 flex-wrap gap-3 rounded-lg border border-dashed border-transparent p-1 transition-colors empty:border-gray-200 empty:bg-gray-50/60 dark:empty:border-dark-600 dark:empty:bg-dark-900/20"
          :animation="180"
          :disabled="saving"
          handle=".monitor-drag-handle"
          :group="{ name: 'channel-monitor-cards', pull: true, put: true }"
          :empty-insert-threshold="80"
          :data-testid="`monitor-drop-zone-${group.id}`"
          @end="onMonitorDragEnd"
        >
          <article v-for="monitor in monitorListFor(group.id)" :key="monitor.id" class="flex min-h-[112px] w-full cursor-grab flex-col justify-between rounded-xl border border-gray-200 bg-gray-50/70 p-3 transition-shadow hover:shadow-md active:cursor-grabbing sm:w-[calc(50%-0.375rem)] xl:w-[calc(33.333%-0.5rem)] dark:border-dark-600 dark:bg-dark-900/30">
            <div class="min-w-0">
              <div class="flex items-start justify-between gap-2">
                <div class="flex min-w-0 items-start gap-2">
                  <input
                    type="checkbox"
                    class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
                    :checked="isMonitorSelected(monitor.id)"
                    :aria-label="t('admin.channelMonitor.organization.selectMonitor', { name: monitor.name })"
                    @click.stop
                    @change="toggleMonitorSelection(monitor.id)"
                  />
                  <div class="monitor-drag-handle flex min-w-0 cursor-grab items-start gap-2 active:cursor-grabbing">
                  <Icon name="menu" size="sm" class="mt-0.5 shrink-0 text-gray-400" />
                  <div class="min-w-0">
                  <h4 class="truncate text-sm font-semibold text-gray-800 dark:text-gray-100" :title="monitor.name">{{ monitor.name }}</h4>
                    <p class="mt-1 truncate font-mono text-[11px] text-gray-400" :title="monitor.primary_model">{{ monitor.primary_model || t('admin.channelMonitor.organization.noModel') }}</p>
                  </div>
                </div>
                </div>
                <span :class="['h-2 w-2 shrink-0 rounded-full', statusClass(monitor)]" :title="monitor.primary_status || t('admin.channelMonitor.organization.unknownStatus')" />
              </div>
              <p class="mt-2 truncate text-[11px] text-gray-400">{{ providerLabel(monitor.provider) }}<span v-if="monitor.group_name"> · {{ monitor.group_name }}</span></p>
            </div>
            <div class="mt-3 space-y-2 border-t border-gray-200/80 pt-2 dark:border-dark-600">
              <div class="flex items-center justify-between gap-2">
                <select class="h-7 min-w-0 max-w-[9rem] rounded-md border border-gray-300 bg-white px-1.5 text-[11px] text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300" :value="String(group.id)" :disabled="saving" :aria-label="t('admin.channelMonitor.organization.assignGroup')" @change="assignMonitor(monitor, $event)">
                  <option value="">{{ t('admin.channelMonitor.organization.ungrouped') }}</option>
                  <option v-for="option in sortedGroups" :key="option.id" :value="String(option.id)">{{ option.name }}</option>
                </select>
                <MonitorActionsCell
                  :row="monitor"
                  :running="props.runningId === monitor.id"
                  :duplicating="props.duplicatingIds.has(monitor.id)"
                  :show-toggle="true"
                  @run="emit('run', $event)"
                  @toggle="emit('toggle', $event)"
                  @duplicate="emit('duplicate', $event)"
                  @edit="emit('edit', $event)"
                  @delete="emit('delete', $event)"
                />
              </div>
            </div>
          </article>
          <p v-if="!monitorListFor(group.id).length" class="w-full self-stretch px-3 py-4 text-center text-xs text-gray-400 dark:text-gray-500">{{ t('admin.channelMonitor.organization.noMonitors') }}</p>
        </VueDraggable>
      </section>

      <section v-if="sortedGroups.length || ungroupedMonitors.length" class="rounded-2xl border border-dashed border-gray-300 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/20">
        <div class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-2">
            <input
              type="checkbox"
              class="h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
              :checked="isGroupFullySelected(null)"
              :indeterminate="isGroupPartiallySelected(null)"
              :aria-label="t('admin.channelMonitor.organization.selectGroup')"
              @click.stop
              @change="toggleGroupSelection(null)"
            />
            <div>
            <h3 class="text-sm font-semibold text-gray-800 dark:text-gray-100">{{ t('admin.channelMonitor.organization.ungrouped') }}</h3>
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.channelMonitor.organization.ungroupedHint') }}</p>
            </div>
          </div>
        </div>
        <VueDraggable
          v-model="monitorLists.ungrouped"
          class="mt-4 flex min-h-24 flex-wrap gap-3 rounded-lg border border-dashed border-transparent p-1 transition-colors empty:border-gray-200 empty:bg-white/60 dark:empty:border-dark-600 dark:empty:bg-dark-800/30"
          :animation="180"
          :disabled="saving"
          handle=".monitor-drag-handle"
          :group="{ name: 'channel-monitor-cards', pull: true, put: true }"
          :empty-insert-threshold="80"
          data-testid="monitor-drop-zone-ungrouped"
          @end="onMonitorDragEnd"
        >
          <article v-for="monitor in ungroupedMonitors" :key="monitor.id" class="flex min-h-[112px] w-full cursor-grab flex-col justify-between rounded-xl border border-gray-200 bg-white p-3 transition-shadow hover:shadow-md active:cursor-grabbing sm:w-[calc(50%-0.375rem)] xl:w-[calc(33.333%-0.5rem)] dark:border-dark-600 dark:bg-dark-800">
            <div class="min-w-0">
              <div class="flex items-start gap-2">
                <input
                  type="checkbox"
                  class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700"
                  :checked="isMonitorSelected(monitor.id)"
                  :aria-label="t('admin.channelMonitor.organization.selectMonitor', { name: monitor.name })"
                  @click.stop
                  @change="toggleMonitorSelection(monitor.id)"
                />
                <div class="monitor-drag-handle flex min-w-0 items-start gap-2 cursor-grab active:cursor-grabbing">
                  <Icon name="menu" size="sm" class="mt-0.5 shrink-0 text-gray-400" />
                  <h4 class="truncate text-sm font-semibold text-gray-800 dark:text-gray-100" :title="monitor.name">{{ monitor.name }}</h4>
                </div>
                <span :class="['h-2 w-2 shrink-0 rounded-full', statusClass(monitor)]" />
              </div>
              <p class="mt-1 truncate font-mono text-[11px] text-gray-400">{{ monitor.primary_model || t('admin.channelMonitor.organization.noModel') }}</p>
            </div>
            <div class="mt-3 space-y-2 border-t border-gray-100 pt-2 dark:border-dark-700">
              <div class="flex items-center justify-between gap-2">
                <span class="min-w-0 truncate text-[11px] text-gray-400">{{ providerLabel(monitor.provider) }}</span>
                <select class="h-7 min-w-0 max-w-[9rem] rounded-md border border-gray-300 bg-white px-1.5 text-[11px] text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300" value="" :disabled="saving" :aria-label="t('admin.channelMonitor.organization.assignGroup')" @change="assignMonitor(monitor, $event)">
                  <option value="">{{ t('admin.channelMonitor.organization.ungrouped') }}</option>
                  <option v-for="option in sortedGroups" :key="option.id" :value="String(option.id)">{{ option.name }}</option>
                </select>
              </div>
              <MonitorActionsCell
                :row="monitor"
                :running="props.runningId === monitor.id"
                :duplicating="props.duplicatingIds.has(monitor.id)"
                :show-toggle="true"
                @run="emit('run', $event)"
                @toggle="emit('toggle', $event)"
                @duplicate="emit('duplicate', $event)"
                @edit="emit('edit', $event)"
                @delete="emit('delete', $event)"
              />
            </div>
          </article>
          <p v-if="!ungroupedMonitors.length" class="w-full self-stretch px-3 py-4 text-center text-xs text-gray-400 dark:text-gray-500">{{ t('admin.channelMonitor.organization.noMonitors') }}</p>
        </VueDraggable>
      </section>
    </div>

    <BaseDialog :show="showGroupDialog" :title="editingGroup ? t('admin.channelMonitor.organization.editGroup') : t('admin.channelMonitor.organization.createGroup')" width="normal" @close="closeGroupDialog">
      <form class="space-y-4" @submit.prevent="saveGroup">
        <label class="block">
          <span class="input-label">{{ t('admin.channelMonitor.organization.groupName') }}</span>
          <input v-model="groupNameDraft" class="input" required maxlength="100" :placeholder="t('admin.channelMonitor.organization.groupNamePlaceholder')" />
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" @click="closeGroupDialog">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving || !groupNameDraft.trim()">{{ saving ? t('common.loading') : t('common.save') }}</button>
        </div>
      </form>
    </BaseDialog>

    <MonitorBulkEditDialog
      :show="showBulkEditDialog"
      :selected-count="selectedMonitorIds.length"
      :selected-intervals="selectedMonitors.map((monitor) => monitor.interval_seconds)"
      :saving="bulkSaving"
      @close="showBulkEditDialog = false"
      @save="handleBulkEditSave"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { VueDraggable } from 'vue-draggable-plus'
import { channelMonitorAPI, type ChannelMonitor, type ChannelMonitorGroup } from '@/api/admin/channelMonitor'
import MonitorActionsCell from './MonitorActionsCell.vue'
import MonitorBulkEditDialog, { type MonitorBulkEditFields } from './MonitorBulkEditDialog.vue'

const props = withDefaults(defineProps<{
  runningId?: number | null
  duplicatingIds?: Set<number>
}>(), {
  runningId: null,
  duplicatingIds: () => new Set<number>(),
})

const emit = defineEmits<{
  (event: 'changed'): void
  (event: 'run', row: ChannelMonitor): void
  (event: 'toggle', row: ChannelMonitor): void
  (event: 'duplicate', row: ChannelMonitor): void
  (event: 'edit', row: ChannelMonitor): void
  (event: 'delete', row: ChannelMonitor): void
}>()
const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const groups = ref<ChannelMonitorGroup[]>([])
const orderedGroups = ref<ChannelMonitorGroup[]>([])
const monitors = ref<ChannelMonitor[]>([])
const monitorLists = ref<Record<string, ChannelMonitor[]>>({})
const showGroupDialog = ref(false)
const editingGroup = ref<ChannelMonitorGroup | null>(null)
const groupNameDraft = ref('')
const selectedMonitorIds = ref<number[]>([])
const showBulkEditDialog = ref(false)
const bulkSaving = ref(false)

const sortedGroups = computed(() => orderedGroups.value)
const sortMonitors = (items: ChannelMonitor[]) => [...items].sort((a, b) => (a.monitor_sort_order ?? 0) - (b.monitor_sort_order ?? 0) || a.id - b.id)
const monitorListKey = (groupId: number | null) => groupId == null ? 'ungrouped' : String(groupId)
const monitorListFor = (groupId: number | null) => monitorLists.value[monitorListKey(groupId)] || []
const ungroupedMonitors = computed(() => monitorListFor(null))
const rebuildMonitorLists = () => {
  const lists: Record<string, ChannelMonitor[]> = { ungrouped: [] }
  for (const group of orderedGroups.value) lists[String(group.id)] = []
  for (const monitor of monitors.value) {
    const key = monitorListKey(monitor.monitor_group_id ?? null)
    if (!lists[key]) lists[key] = []
    lists[key].push(monitor)
  }
  for (const key of Object.keys(lists)) lists[key] = sortMonitors(lists[key])
  monitorLists.value = lists
}
const monitorsFor = (groupId: number) => monitorListFor(groupId)
const monitorIdsFor = (groupId: number | null) => monitorListFor(groupId).map((monitor) => monitor.id)
const isMonitorSelected = (monitorId: number) => selectedMonitorIds.value.includes(monitorId)
const isGroupFullySelected = (groupId: number | null) => {
  const ids = monitorIdsFor(groupId)
  return ids.length > 0 && ids.every((id) => isMonitorSelected(id))
}
const isGroupPartiallySelected = (groupId: number | null) => {
  const ids = monitorIdsFor(groupId)
  const selected = ids.filter((id) => isMonitorSelected(id)).length
  return selected > 0 && selected < ids.length
}
const toggleMonitorSelection = (monitorId: number) => {
  selectedMonitorIds.value = isMonitorSelected(monitorId)
    ? selectedMonitorIds.value.filter((id) => id !== monitorId)
    : [...selectedMonitorIds.value, monitorId]
}
const toggleGroupSelection = (groupId: number | null) => {
  const ids = monitorIdsFor(groupId)
  const selected = new Set(selectedMonitorIds.value)
  if (ids.length > 0 && ids.every((id) => selected.has(id))) {
    ids.forEach((id) => selected.delete(id))
  } else {
    ids.forEach((id) => selected.add(id))
  }
  selectedMonitorIds.value = [...selected]
}
const selectAllMonitors = () => {
  selectedMonitorIds.value = monitors.value.map((monitor) => monitor.id)
}
const selectedMonitors = computed(() => monitors.value.filter((monitor) => selectedMonitorIds.value.includes(monitor.id)))
const clearSelection = () => {
  selectedMonitorIds.value = []
}
function providerLabel(provider: string) {
  const key = `monitorCommon.providers.${provider}`
  return t(key) === key ? provider : t(key)
}

function statusClass(monitor: ChannelMonitor) {
  if (monitor.primary_status === 'failed' || monitor.primary_status === 'error') return 'bg-red-500'
  if (monitor.primary_status === 'degraded') return 'bg-amber-500'
  return 'bg-emerald-500'
}

async function load() {
  loading.value = true
  try {
    const [groupResponse, monitorResponse] = await Promise.all([
      channelMonitorAPI.listGroups(),
      channelMonitorAPI.list({ page: 1, page_size: 200 }),
    ])
    groups.value = groupResponse.items || []
    orderedGroups.value = [...groups.value].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
    monitors.value = monitorResponse.items || []
    const validIds = new Set(monitors.value.map((monitor) => monitor.id))
    selectedMonitorIds.value = selectedMonitorIds.value.filter((id) => validIds.has(id))
    rebuildMonitorLists()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.organization.loadError')))
  } finally {
    loading.value = false
  }
}

async function handleBulkEditSave(fields: MonitorBulkEditFields) {
  const targetIds = [...selectedMonitorIds.value]
  if (!targetIds.length) return
  bulkSaving.value = true
  const failedIds: number[] = []
  const updatedById = new Map<number, ChannelMonitor>()
  try {
    for (const monitorId of targetIds) {
      try {
        const updated = await channelMonitorAPI.update(monitorId, fields)
        const current = monitors.value.find((monitor) => monitor.id === monitorId)
        updatedById.set(monitorId, current ? { ...current, ...updated } : updated)
      } catch (error) {
        failedIds.push(monitorId)
        console.error('Failed to bulk update channel monitor:', monitorId, error)
      }
    }
    if (updatedById.size > 0) {
      monitors.value = monitors.value.map((monitor) => updatedById.get(monitor.id) || monitor)
      rebuildMonitorLists()
    }
    if (failedIds.length > 0) {
      selectedMonitorIds.value = failedIds
      appStore.showError(t('admin.channelMonitor.organization.bulkPartial', { failed: failedIds.length }))
      return
    }
    selectedMonitorIds.value = []
    showBulkEditDialog.value = false
    appStore.showSuccess(t('admin.channelMonitor.organization.bulkSuccess', { count: targetIds.length }))
    emit('changed')
  } finally {
    bulkSaving.value = false
  }
}

function openCreateGroup() {
  editingGroup.value = null
  groupNameDraft.value = ''
  showGroupDialog.value = true
}

function openEditGroup(group: ChannelMonitorGroup) {
  editingGroup.value = group
  groupNameDraft.value = group.name
  showGroupDialog.value = true
}

function closeGroupDialog() {
  if (saving.value) return
  showGroupDialog.value = false
  editingGroup.value = null
}

async function saveGroup() {
  const name = groupNameDraft.value.trim()
  if (!name || saving.value) return
  saving.value = true
  try {
    if (editingGroup.value) {
      const updated = await channelMonitorAPI.updateGroup(editingGroup.value.id, { name })
      groups.value = groups.value.map((group) => group.id === updated.id ? { ...group, ...updated } : group)
      orderedGroups.value = orderedGroups.value.map((group) => group.id === updated.id ? { ...group, ...updated } : group)
    } else {
      await channelMonitorAPI.createGroup({ name })
      await load()
    }
    showGroupDialog.value = false
    editingGroup.value = null
    appStore.showSuccess(t('admin.channelMonitor.organization.saved'))
    emit('changed')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.organization.saveError')))
  } finally {
    saving.value = false
  }
}

async function removeGroup(group: ChannelMonitorGroup) {
  if (!window.confirm(t('admin.channelMonitor.organization.deleteConfirm', { name: group.name }))) return
  saving.value = true
  try {
    await channelMonitorAPI.deleteGroup(group.id)
    groups.value = groups.value.filter((item) => item.id !== group.id)
    orderedGroups.value = orderedGroups.value.filter((item) => item.id !== group.id)
    monitors.value = monitors.value.map((monitor) => monitor.monitor_group_id === group.id ? { ...monitor, monitor_group_id: null, monitor_sort_order: 0 } : monitor)
    rebuildMonitorLists()
    appStore.showSuccess(t('admin.channelMonitor.organization.deleted'))
    emit('changed')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.organization.saveError')))
  } finally {
    saving.value = false
  }
}

async function onGroupDragEnd() {
  const updates = orderedGroups.value.map((group, position) => ({ id: group.id, sort_order: position * 10 }))
  groups.value = groups.value.map((group) => ({ ...group, sort_order: updates.find((item) => item.id === group.id)?.sort_order ?? group.sort_order }))
  await persistGroupOrder(updates)
}

async function persistGroupOrder(updates: { id: number; sort_order: number }[]) {
  saving.value = true
  try {
    await channelMonitorAPI.updateGroupSortOrder(updates)
    emit('changed')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.organization.saveError')))
    await load()
  } finally {
    saving.value = false
  }
}

async function onMonitorDragEnd() {
  const updates: { id: number; monitor_group_id: number | null; monitor_sort_order: number }[] = orderedGroups.value.flatMap((group) =>
    monitorListFor(group.id).map((monitor, index) => ({
      id: monitor.id,
      monitor_group_id: group.id,
      monitor_sort_order: index * 10,
    }))
  )
  updates.push(...ungroupedMonitors.value.map((monitor, index) => ({
    id: monitor.id,
    monitor_group_id: null,
    monitor_sort_order: index * 10,
  })))
  await persistMonitorOrder(updates)
}

async function assignMonitor(monitor: ChannelMonitor, event: Event) {
  const value = (event.target as HTMLSelectElement).value
  const nextGroupId = value ? Number(value) : null
  const oldGroupId = monitor.monitor_group_id ?? null
  const destination = [...monitorListFor(nextGroupId)]
  const moved = { ...monitor, monitor_group_id: nextGroupId, monitor_sort_order: destination.length * 10 }
  monitors.value = monitors.value.map((item) => item.id === monitor.id ? moved : item)
  rebuildMonitorLists()
  const affected = new Set<number | null>([oldGroupId, nextGroupId])
  saving.value = true
  try {
    const updates = [...affected].flatMap((groupId) => {
      const list = groupId == null ? ungroupedMonitors.value : monitorsFor(groupId)
      return list.map((item, index) => ({ id: item.id, monitor_group_id: groupId, monitor_sort_order: index * 10 }))
    })
    await channelMonitorAPI.updateMonitorSortOrder(updates)
    monitors.value = monitors.value.map((item) => updates.find((update) => update.id === item.id) ? { ...item, ...updates.find((update) => update.id === item.id) } : item)
    rebuildMonitorLists()
    emit('changed')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.organization.saveError')))
    await load()
  } finally {
    saving.value = false
  }
}

async function persistMonitorOrder(updates: { id: number; monitor_group_id: number | null; monitor_sort_order: number }[]) {
  if (!updates.length) return
  saving.value = true
  try {
    await channelMonitorAPI.updateMonitorSortOrder(updates)
    const order = new Map(updates.map((update) => [update.id, update]))
    monitors.value = monitors.value.map((monitor) => order.has(monitor.id) ? { ...monitor, ...order.get(monitor.id)! } : monitor)
    rebuildMonitorLists()
    emit('changed')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channelMonitor.organization.saveError')))
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.375rem;
  padding: 0.3rem;
  color: rgb(156 163 175);
  transition: background-color 150ms, color 150ms;
}
.icon-btn:hover:not(:disabled) {
  background: rgb(243 244 246);
  color: rgb(37 99 235);
}
.icon-btn:disabled {
  cursor: not-allowed;
  opacity: 0.35;
}
</style>
