<template>
  <div>
    <div
      v-if="loading && items.length === 0"
      class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <div
        v-for="i in 6"
        :key="i"
        class="p-5 rounded-2xl min-h-[280px] bg-white/70 dark:bg-dark-800/60 border border-gray-200/80 dark:border-dark-700/70 animate-pulse"
      >
        <div class="flex items-start gap-3">
          <div class="w-9 h-9 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-200 dark:bg-dark-700"></div>
          </div>
          <div class="h-6 w-16 rounded-full bg-gray-200 dark:bg-dark-700"></div>
        </div>
        <div class="mt-5 grid grid-cols-2 gap-2">
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-16 rounded-xl bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
        <div class="mt-6 h-5 w-full rounded bg-gray-100 dark:bg-dark-900/40"></div>
      </div>
    </div>

    <EmptyState
      v-else-if="items.length === 0"
      :title="t('channelStatus.empty.title')"
      :description="t('channelStatus.empty.description')"
    />

    <div v-else class="space-y-8">
      <section
        v-for="group in groupedItems"
        :key="group.id"
        class="space-y-3"
        :data-testid="`monitor-user-group-${group.id}`"
      >
        <div class="flex items-center gap-3">
          <span class="h-5 w-1 rounded-full bg-primary-500"></span>
          <h2 class="text-base font-bold text-gray-900 dark:text-white">{{ group.name }}</h2>
          <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-300">{{ group.items.length }}</span>
        </div>
        <div class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          <MonitorCard
            v-for="item in group.items"
            :key="item.id"
            :item="item"
            :window="window"
            :availability-value="resolveAvailability(item)"
            :countdown-seconds="countdownSeconds"
            @click="emit('cardClick', item)"
          />
        </div>
      </section>

      <section v-if="ungroupedItems.length" class="space-y-3" data-testid="monitor-user-ungrouped">
        <div class="flex items-center gap-3">
          <span class="h-5 w-1 rounded-full bg-gray-300 dark:bg-dark-600"></span>
          <h2 class="text-base font-bold text-gray-900 dark:text-white">{{ t('admin.channelMonitor.organization.ungrouped') }}</h2>
          <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-300">{{ ungroupedItems.length }}</span>
        </div>
        <div class="grid gap-5 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          <MonitorCard
            v-for="item in ungroupedItems"
            :key="item.id"
            :item="item"
            :window="window"
            :availability-value="resolveAvailability(item)"
            :countdown-seconds="countdownSeconds"
            @click="emit('cardClick', item)"
          />
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView, UserMonitorDetail } from '@/api/channelMonitor'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorCard from './MonitorCard.vue'

const props = defineProps<{
  items: UserMonitorView[]
  window: '7d' | '15d' | '30d'
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
}>()

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
}>()

const { t } = useI18n()

type MonitorGroup = {
  id: number
  name: string
  sortOrder: number
  items: UserMonitorView[]
}

const sortItems = (items: UserMonitorView[]) => [...items].sort((a, b) =>
  (a.monitor_sort_order ?? 0) - (b.monitor_sort_order ?? 0) || a.id - b.id,
)

const groupedItems = computed<MonitorGroup[]>(() => {
  const groups = new Map<number, MonitorGroup>()
  for (const item of props.items) {
    if (item.monitor_group_id == null) continue
    const id = item.monitor_group_id
    const existing = groups.get(id)
    if (existing) {
      existing.items.push(item)
      continue
    }
    groups.set(id, {
      id,
      name: item.monitor_group_name || t('admin.channelMonitor.organization.ungrouped'),
      sortOrder: item.monitor_group_sort_order ?? 0,
      items: [item],
    })
  }
  return [...groups.values()]
    .map(group => ({ ...group, items: sortItems(group.items) }))
    .sort((a, b) => a.sortOrder - b.sortOrder || a.id - b.id)
})

const ungroupedItems = computed(() => sortItems(props.items.filter(item => item.monitor_group_id == null)))

function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') {
    return item.availability_7d ?? null
  }
  const detail = props.detailCache[item.id]
  if (!detail) return null
  const primary = detail.models.find(m => m.model === item.primary_model)
  if (!primary) return null
  return props.window === '15d' ? primary.availability_15d ?? null : primary.availability_30d ?? null
}
</script>
