<template>
  <section class="mb-6 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
    <div class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-200 px-4 py-4 dark:border-dark-700">
      <div>
        <h2 class="font-semibold text-gray-900 dark:text-white">{{ type === 'invites' ? t('admin.affiliates.analytics.inviteTitle') : t('admin.affiliates.analytics.rebateTitle') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.affiliates.analytics.periodHint') }}</p>
      </div>
      <button type="button" class="btn btn-secondary px-2 md:px-3" :disabled="loading" :title="t('common.refresh')" @click="load">
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>
    <div v-if="loading" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
    <div v-else class="p-4">
      <div class="overflow-x-auto">
        <table class="w-full min-w-[760px] text-sm">
          <thead><tr class="border-b border-gray-100 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400">
            <th class="px-3 py-2">{{ t('admin.affiliates.analytics.rank') }}</th>
            <th class="px-3 py-2">{{ t('admin.affiliates.analytics.inviter') }}</th>
            <th class="px-3 py-2 text-right">{{ type === 'invites' ? t('admin.affiliates.analytics.periodInvites') : t('admin.affiliates.analytics.periodRebateRows') }}</th>
            <th class="px-3 py-2 text-right">{{ t('admin.affiliates.analytics.change') }}</th>
            <th class="px-3 py-2 text-right">{{ type === 'invites' ? t('admin.affiliates.analytics.totalInvites') : t('admin.affiliates.analytics.periodRebateAmount') }}</th>
            <th class="px-3 py-2 text-right">{{ type === 'invites' ? t('admin.affiliates.analytics.lastInvite') : t('admin.affiliates.analytics.totalRebate') }}</th>
          </tr></thead>
          <tbody v-if="ranking.length">
            <tr v-for="row in ranking" :key="`${type}-${row.inviter_id}`" class="border-b border-gray-50 dark:border-dark-800">
              <td class="px-3 py-3 font-semibold text-gray-400">{{ row.rank }}</td>
              <td class="px-3 py-3"><div class="font-medium text-gray-900 dark:text-white">{{ row.inviter_username || row.inviter_email }}</div><div class="text-xs text-gray-500 dark:text-gray-400">{{ row.inviter_email }}</div></td>
              <td class="px-3 py-3 text-right font-medium">{{ row.current_count }}</td>
              <td class="px-3 py-3 text-right" :class="delta(row) > 0 ? 'text-emerald-600' : delta(row) < 0 ? 'text-red-600' : 'text-gray-500'">{{ formatDelta(delta(row)) }}</td>
              <td class="px-3 py-3 text-right">{{ type === 'invites' ? row.total_count : money(row.current_amount) }}</td>
              <td class="px-3 py-3 text-right">{{ type === 'invites' ? formatDate(row.last_activity_at) : money(row.total_amount) }}</td>
            </tr>
          </tbody>
          <tbody v-else><tr><td colspan="6" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.affiliates.analytics.empty') }}</td></tr></tbody>
        </table>
      </div>

      <div v-if="type === 'invites'" class="mt-6 overflow-x-auto">
        <div class="mb-2 flex items-baseline justify-between gap-3"><h3 class="font-medium text-gray-900 dark:text-white">{{ t('admin.affiliates.analytics.growthTitle') }}</h3><span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.affiliates.analytics.growthHint') }}</span></div>
        <table class="w-full min-w-[620px] text-sm">
          <thead><tr class="border-b border-gray-100 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400"><th class="px-3 py-2">{{ t('admin.affiliates.analytics.date') }}</th><th class="px-3 py-2 text-right">{{ t('admin.affiliates.analytics.naturalGrowth') }}</th><th class="px-3 py-2 text-right">{{ t('admin.affiliates.analytics.invitedGrowth') }}</th><th class="px-3 py-2 text-right">{{ t('admin.affiliates.analytics.totalGrowth') }}</th><th class="px-3 py-2 text-right">{{ t('admin.affiliates.analytics.invitedShare') }}</th></tr></thead>
          <tbody><tr v-for="row in analytics.registration_growth" :key="row.date" class="border-b border-gray-50 dark:border-dark-800"><td class="px-3 py-2 text-gray-700 dark:text-gray-300">{{ row.date }}</td><td class="px-3 py-2 text-right">{{ row.natural_count }}</td><td class="px-3 py-2 text-right font-medium text-blue-600 dark:text-blue-400">{{ row.invited_count }}</td><td class="px-3 py-2 text-right font-semibold">{{ row.total_count }}</td><td class="px-3 py-2 text-right text-gray-500">{{ formatShare(row.invited_share) }}</td></tr><tr v-if="!analytics.registration_growth.length"><td colspan="5" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.affiliates.analytics.empty') }}</td></tr></tbody>
        </table>
      </div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { affiliatesAPI, type AffiliateAdminAnalytics, type AffiliateAdminRanking } from '@/api/admin/affiliates'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores/app'

type AnalyticsType = 'invites' | 'rebates'
const props = defineProps<{ type: AnalyticsType; startAt?: string; endAt?: string }>()
const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const analytics = ref<AffiliateAdminAnalytics>({ start_at: '', end_at: '', previous_start_at: '', previous_end_at: '', top_inviters: [], top_rebate_earners: [], registration_growth: [] })
const ranking = computed(() => props.type === 'invites' ? analytics.value.top_inviters : analytics.value.top_rebate_earners)
function timezone() { try { return Intl.DateTimeFormat().resolvedOptions().timeZone } catch { return 'UTC' } }
function delta(row: AffiliateAdminRanking) { return row.current_count - row.previous_count }
function formatDelta(value: number) { return value > 0 ? `+${value}` : String(value) }
function money(value: number) { return `$${Number(value || 0).toFixed(8)}` }
function formatShare(value: number) { return `${(Number(value || 0) * 100).toFixed(1)}%` }
function formatDate(value?: string | null) { return value ? new Date(value).toLocaleDateString() : '-' }
async function load() {
  loading.value = true
  try { analytics.value = await affiliatesAPI.getAnalytics({ start_at: props.startAt || undefined, end_at: props.endAt || undefined, timezone: timezone() }) }
  catch (error) { appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error'))) }
  finally { loading.value = false }
}
watch(() => [props.startAt, props.endAt], () => { void load() })
onMounted(load)
</script>
