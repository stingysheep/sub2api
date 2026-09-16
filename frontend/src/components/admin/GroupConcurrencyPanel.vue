<template>
  <section class="card p-4" aria-labelledby="group-concurrency-title" :aria-busy="refreshing">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 id="group-concurrency-title" class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.dashboard.groupConcurrency.title') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.groupConcurrency.description') }}</p>
      </div>
      <div class="flex items-center gap-3">
        <label v-if="!collapsed" class="flex items-center gap-1.5 text-xs text-gray-600 dark:text-gray-300">
          <input v-model="showIdle" type="checkbox" class="rounded border-gray-300 text-primary-600" />
          {{ t('admin.dashboard.groupConcurrency.showIdle') }}
        </label>
        <button v-if="!collapsed" type="button" class="btn btn-secondary btn-sm" :disabled="refreshing" @click="refresh">{{ t('common.refresh') }}</button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :aria-expanded="!collapsed"
          aria-controls="group-concurrency-details"
          @click="collapsed = !collapsed"
        >
          {{ collapsed ? t('admin.dashboard.groupConcurrency.expand') : t('admin.dashboard.groupConcurrency.collapse') }}
        </button>
      </div>
    </div>
    <p v-if="error" role="status" class="mt-3 rounded-md bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
      {{ snapshot ? t('admin.dashboard.groupConcurrency.stale') : t('admin.dashboard.groupConcurrency.failed') }}
    </p>
    <p v-if="!snapshot && !error" role="status" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</p>
    <template v-if="snapshot">
      <div class="mt-4 flex flex-wrap items-baseline gap-x-5 gap-y-1 border-b border-gray-100 pb-3 dark:border-dark-700">
        <span class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.dashboard.groupConcurrency.activeGroups', { count: activeCount }) }}</span>
        <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.dashboard.groupConcurrency.total', { count: snapshot.current_concurrency }) }}</span>
        <span class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.dashboard.groupConcurrency.activeUsers', { count: snapshot.active_users }) }}</span>
        <span class="ml-auto text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.groupConcurrency.updatedAt', { time: updatedAt }) }}</span>
      </div>
      <div v-if="!collapsed" id="group-concurrency-details">
        <p
        v-if="snapshot.unattributed_concurrency > 0 || snapshot.group_attributed_slots !== snapshot.current_concurrency"
        class="mt-3 rounded-md bg-blue-50 px-3 py-2 text-xs text-blue-800 dark:bg-blue-900/20 dark:text-blue-300"
        data-testid="concurrency-attribution-note"
      >
        {{ t('admin.dashboard.groupConcurrency.attribution', {
          attributed: snapshot.group_attributed_slots,
          total: snapshot.current_concurrency,
          unattributed: snapshot.unattributed_concurrency
        }) }}
        </p>
        <div v-if="snapshot.users.length" class="mt-3 rounded-lg border border-gray-100 p-3 dark:border-dark-700">
        <p class="mb-2 text-xs font-medium text-gray-700 dark:text-gray-200">{{ t('admin.dashboard.groupConcurrency.activeUserList') }}</p>
        <ul class="grid gap-x-5 gap-y-1 sm:grid-cols-2" data-testid="active-user-list">
          <li v-for="user in snapshot.users" :key="user.user_id" class="flex items-center justify-between gap-3 text-xs">
            <span class="min-w-0 truncate text-gray-600 dark:text-gray-300">{{ user.user_label }} · #{{ user.user_id }}</span>
            <span class="shrink-0 font-medium tabular-nums text-gray-800 dark:text-gray-100">{{ t('admin.dashboard.groupConcurrency.userConcurrency', { count: user.current_in_use }) }}</span>
          </li>
        </ul>
        </div>
        <ul v-if="visibleGroups.length" class="mt-1 max-h-80 divide-y divide-gray-100 overflow-y-auto dark:divide-dark-700" :class="{ 'opacity-60': error }" data-testid="group-concurrency-list">
        <li v-for="group in visibleGroups" :key="group.group_id" class="py-3">
          <div class="flex items-center gap-3">
          <span class="h-2 w-2 shrink-0 rounded-full" :class="group.current_in_use > 0 ? 'bg-emerald-500' : 'bg-gray-300 dark:bg-dark-600'" aria-hidden="true" />
          <div class="min-w-0 flex-1">
            <p class="break-words text-sm font-medium text-gray-900 dark:text-white">{{ group.group_name || (group.group_id === 0 ? t('admin.dashboard.groupConcurrency.unassigned') : `#${group.group_id}`) }}</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ group.current_in_use > 0 ? t('admin.dashboard.groupConcurrency.groupActiveUsers', { count: group.active_users }) : t('admin.dashboard.groupConcurrency.idle') }}<span v-if="group.platform"> · {{ group.platform }}</span>
            </p>
          </div>
          <div class="shrink-0 text-right">
            <span class="text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ group.current_in_use }}</span>
            <span class="ml-1.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.groupConcurrency.unit') }}</span>
          </div>
          </div>
          <ul v-if="group.users.length" class="ml-5 mt-2 space-y-1 border-l border-gray-200 pl-3 dark:border-dark-600">
            <li v-for="user in group.users" :key="user.user_id" class="flex items-center justify-between gap-3 text-xs">
              <span class="min-w-0 truncate text-gray-600 dark:text-gray-300">{{ user.user_label }} · #{{ user.user_id }}</span>
              <span class="shrink-0 tabular-nums text-gray-700 dark:text-gray-200">{{ t('admin.dashboard.groupConcurrency.userConcurrency', { count: user.current_in_use }) }}</span>
            </li>
          </ul>
        </li>
        </ul>
        <p v-else class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.groupConcurrency.empty') }}</p>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getGroupConcurrency, type GroupConcurrencySnapshot } from '@/api/admin/dashboard'

const { t } = useI18n()
const snapshot = ref<GroupConcurrencySnapshot | null>(null)
const refreshing = ref(false)
const error = ref(false)
const showIdle = ref(false)
const collapsed = ref(false)
const sortedGroups = computed(() => [...(snapshot.value?.groups || [])].sort((a, b) => b.current_in_use - a.current_in_use || a.group_id - b.group_id))
const visibleGroups = computed(() => sortedGroups.value.filter(group => showIdle.value || group.current_in_use > 0))
const activeCount = computed(() => sortedGroups.value.filter(group => group.current_in_use > 0).length)
const updatedAt = computed(() => snapshot.value ? new Date(snapshot.value.timestamp).toLocaleTimeString() : '')
let timer: ReturnType<typeof setTimeout> | undefined
let controller: AbortController | undefined
let disposed = false

// Schedule after completion so a slow backend never causes overlapping polls.
const refresh = async () => {
  if (disposed || refreshing.value || document.hidden) return
  clearTimeout(timer)
  const request = new AbortController()
  controller = request
  refreshing.value = true
  try {
    const result = await getGroupConcurrency(request.signal)
    if (!disposed && !request.signal.aborted) {
      snapshot.value = result
      error.value = false
    }
  } catch {
    if (!disposed && !request.signal.aborted) error.value = true
  } finally {
    refreshing.value = false
    if (!disposed && !document.hidden) timer = setTimeout(refresh, 5000)
  }
}

const onVisibilityChange = () => {
  if (document.hidden) {
    clearTimeout(timer)
    controller?.abort()
  } else {
    void refresh()
  }
}
onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  void refresh()
})
onUnmounted(() => {
  disposed = true
  clearTimeout(timer)
  controller?.abort()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>
