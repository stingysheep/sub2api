<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-end justify-between gap-4"><div><h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.profit.title') }}</h1><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.profit.description') }}</p></div><div class="flex items-center gap-3"><div class="rounded-lg border border-violet-100 bg-violet-50 px-3 py-2 dark:border-violet-900/40 dark:bg-violet-950/30"><p class="text-[11px] text-violet-600 dark:text-violet-300">{{ t('admin.profit.adminCost') }}</p><p class="text-sm font-semibold text-violet-700 dark:text-violet-200">{{ money(adminCost) }}</p></div><button type="button" class="btn btn-secondary" :disabled="loading" @click="loadStats"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />{{ t('common.refresh') }}</button></div></div>
      <div class="card p-4"><div class="flex flex-wrap items-center gap-3"><DateRangePicker :start-date="startDate" :end-date="endDate" @change="onDateRangeChange" /><span class="ml-auto text-xs text-gray-500 dark:text-gray-400">{{ startDate }} - {{ endDate }}</span></div></div>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"><div v-for="card in cards" :key="card.key" class="card p-5"><p class="text-sm text-gray-500 dark:text-gray-400">{{ card.label }}</p><p class="mt-2 text-2xl font-semibold" :class="card.color">{{ card.value }}</p><p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ card.hint }}</p></div></div>
      <div class="card overflow-hidden"><div class="flex flex-wrap items-start justify-between gap-4 border-b border-gray-200 px-4 py-3 dark:border-dark-700"><div><h2 class="font-medium text-gray-900 dark:text-white">{{ t('admin.profit.paidBalanceTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.paidBalanceHint') }}</p></div><div class="text-right"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.paidBalanceTotal') }}</p><p class="mt-1 text-xl font-semibold text-emerald-600 dark:text-emerald-400">{{ money(paidBalanceSummary.total_paid_balance) }}</p><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.paidBalanceUsers', { count: paidBalanceSummary.users_with_paid_balance }) }}</p></div></div><div v-if="paidBalanceSummary.ranking.length" class="divide-y divide-gray-100 dark:divide-dark-700"><div v-for="item in paidBalanceSummary.ranking" :key="item.user_id" class="flex items-center gap-3 px-4 py-3"><span class="w-7 shrink-0 text-center text-sm font-semibold text-gray-400">{{ item.rank }}</span><div class="min-w-0 flex-1"><p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ item.username || item.email }}</p><p class="truncate text-xs text-gray-500 dark:text-gray-400">{{ item.email }}</p></div><span class="shrink-0 font-mono text-sm font-semibold text-emerald-600 dark:text-emerald-400">{{ money(item.paid_balance) }}</span></div></div><div v-else class="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('admin.profit.paidBalanceEmpty') }}</div></div>
      <div class="card overflow-hidden"><div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700"><div><h2 class="font-medium text-gray-900 dark:text-white">{{ t('admin.profit.costSummaryTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.costSummaryHint') }}</p></div><span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.requests', { count: stats.total_requests }) }}</span></div><div class="grid grid-cols-1 divide-y divide-gray-100 sm:grid-cols-3 sm:divide-x sm:divide-y-0 dark:divide-dark-700"><div class="p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.paidRevenue') }}</p><p class="mt-1 font-medium text-emerald-600">{{ money(paidRevenue) }}</p></div><div class="p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.paidUpstreamCost') }}</p><p class="mt-1 font-medium text-orange-600">{{ money(paidUpstreamCost) }}</p></div><div class="p-4"><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.freeUsageCost') }}</p><p class="mt-1 font-medium text-red-600">{{ money(freeUsage) }}</p></div></div></div>
      <div class="card overflow-hidden"><div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700"><h2 class="font-medium text-gray-900 dark:text-white">{{ t('admin.profit.channelBreakdownTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.channelBreakdownHint') }}</p></div><ProfitTable :items="breakdown.groups" :name-label="t('admin.profit.channel')" /></div>
      <div class="card overflow-hidden"><div class="border-b border-gray-200 px-4 py-3 dark:border-dark-700"><h2 class="font-medium text-gray-900 dark:text-white">{{ t('admin.profit.modelBreakdownTitle') }}</h2><p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.modelBreakdownHint') }}</p></div><ProfitTable :items="breakdown.models" :name-label="t('admin.profit.model')" /></div>
      <div class="card p-5"><div class="flex items-center justify-between"><h2 class="font-medium text-gray-900 dark:text-white">{{ t('admin.profit.freeCreditTitle') }}</h2><span class="rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">{{ t('admin.profit.accountingBasis') }}</span></div><div class="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.freeIssued') }}</p><p class="mt-1 text-lg font-semibold text-sky-600">{{ money(freeIssued) }}</p></div><div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.freeConsumed') }}</p><p class="mt-1 text-lg font-semibold text-amber-600">{{ money(freeConsumed) }}</p></div><div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.freeUsed') }}</p><p class="mt-1 text-lg font-semibold text-amber-600">{{ money(freeUsed) }}</p><p class="mt-1 text-[11px] text-gray-500 dark:text-gray-400">{{ t('admin.profit.selectedFreeUsageCost') }}：{{ money(selectedFreeUsageCost) }}</p></div><div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.profit.freeUsageCost') }}</p><p class="mt-1 text-lg font-semibold text-red-600">{{ money(freeUsage) }}</p></div></div><p class="mt-3 text-sm text-gray-600 dark:text-gray-300">{{ t('admin.profit.freeCreditHint') }}</p></div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminUsageAPI, type ProfitBreakdownItem } from '@/api/admin/usage'
import { usersAPI, type PaidBalanceSummaryResponse } from '@/api/admin/users'
import { formatCurrency } from '@/utils/format'

const { t } = useI18n()
const today = new Date()
const iso = (date: Date) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
const startDate = ref(iso(new Date(today.getFullYear(), today.getMonth(), 1)))
const endDate = ref(iso(today))
const loading = ref(false)
const stats = ref({ total_requests: 0, total_actual_cost: 0, total_account_cost: 0, total_paid_balance_cost: 0, total_free_balance_cost: 0, total_free_upstream_cost: 0, selected_free_upstream_cost: 0, total_paid_upstream_cost: 0, total_free_balance_issued: 0, total_free_balance_consumed: 0 })
const adminStats = ref({ total_account_cost: 0 })
const breakdown = ref<{ groups: ProfitBreakdownItem[]; models: ProfitBreakdownItem[] }>({ groups: [], models: [] })
const paidBalanceSummary = ref<PaidBalanceSummaryResponse>({ total_paid_balance: 0, users_with_paid_balance: 0, ranking: [] })
const paidRevenue = computed(() => Number(stats.value.total_paid_balance_cost || 0))
const paidUpstreamCost = computed(() => Number(stats.value.total_paid_upstream_cost || 0))
const adminCost = computed(() => Number(adminStats.value.total_account_cost || 0))
const freeIssued = computed(() => Number(stats.value.total_free_balance_issued || 0))
const freeConsumed = computed(() => Number(stats.value.total_free_balance_consumed || 0))
const freeUsed = computed(() => Number(stats.value.total_free_balance_cost || 0))
const freeUsage = computed(() => Number(stats.value.total_free_upstream_cost || 0))
const selectedFreeUsageCost = computed(() => Number(stats.value.selected_free_upstream_cost || 0))
// 实际利润只核算付费余额收入及其对应的上游成本；管理员成本和免费额度亏损单独展示。
const actualProfit = computed(() => paidRevenue.value - paidUpstreamCost.value)
const money = (value: number) => formatCurrency(Number.isFinite(value) ? value : 0)
const cards = computed(() => [
  { key: 'revenue', label: t('admin.profit.paidRevenue'), value: money(paidRevenue.value), color: 'text-emerald-600 dark:text-emerald-400', hint: t('admin.profit.revenueHint') },
  { key: 'cost', label: t('admin.profit.paidUpstreamCost'), value: money(paidUpstreamCost.value), color: 'text-orange-600 dark:text-orange-400', hint: t('admin.profit.paidCostHint') },
  { key: 'profit', label: t('admin.profit.actualProfit'), value: money(actualProfit.value), color: actualProfit.value < 0 ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400', hint: t('admin.profit.profitHint') },
  { key: 'margin', label: t('admin.profit.margin'), value: paidRevenue.value > 0 ? `${((actualProfit.value / paidRevenue.value) * 100).toFixed(1)}%` : '—', color: 'text-gray-900 dark:text-white', hint: t('admin.profit.marginHint') },
])
const loadStats = async () => {
  loading.value = true
  try {
    const params = { start_date: startDate.value, end_date: endDate.value, billing_type: 0, account_cost_basis: 'current_account_rate' as const, nocache: 1 }
    const breakdownRequest = typeof adminUsageAPI.getProfitBreakdown === 'function'
      ? adminUsageAPI.getProfitBreakdown({ start_date: startDate.value, end_date: endDate.value })
      : Promise.resolve({ groups: [], models: [] })
    const [all, admins, detail] = await Promise.all([adminUsageAPI.getStats({ ...params, user_role: 'user' }), adminUsageAPI.getStats({ ...params, user_role: 'admin' }), breakdownRequest])
    stats.value = all
    adminStats.value = admins
    breakdown.value = detail
    try {
      paidBalanceSummary.value = await usersAPI.getPaidBalanceSummary()
    } catch (error) {
      console.error('Failed to load current paid balance summary:', error)
    }
  } finally { loading.value = false }
}
const onDateRangeChange = (range: { startDate: string; endDate: string }) => { startDate.value = range.startDate; endDate.value = range.endDate; void loadStats() }
const ProfitTable = defineComponent({
  props: { items: { type: Array as PropType<ProfitBreakdownItem[]>, required: true }, nameLabel: { type: String, required: true } },
  setup(props) { return () => h('div', { class: 'overflow-x-auto' }, [h('table', { class: 'w-full min-w-[760px] text-sm' }, [h('thead', [h('tr', { class: 'border-b border-gray-100 text-left text-xs text-gray-500 dark:border-dark-700 dark:text-gray-400' }, [h('th', { class: 'px-4 py-3' }, props.nameLabel), h('th', { class: 'px-4 py-3 text-right' }, t('admin.profit.requestsLabel')), h('th', { class: 'px-4 py-3 text-right' }, t('admin.profit.paidRevenue')), h('th', { class: 'px-4 py-3 text-right' }, t('admin.profit.paidUpstreamCost')), h('th', { class: 'px-4 py-3 text-right' }, t('admin.profit.profitLabel'))])]), h('tbody', props.items.length ? props.items.map(item => h('tr', { key: `${item.group_id}-${item.group_name}`, class: 'border-b border-gray-50 dark:border-dark-800' }, [h('td', { class: 'px-4 py-3 font-medium text-gray-900 dark:text-white' }, item.group_name), h('td', { class: 'px-4 py-3 text-right' }, String(item.requests)), h('td', { class: 'px-4 py-3 text-right text-emerald-600' }, money(item.paid_revenue)), h('td', { class: 'px-4 py-3 text-right text-orange-600' }, money(item.paid_upstream_cost)), h('td', { class: `px-4 py-3 text-right font-medium ${item.profit >= 0 ? 'text-emerald-600' : 'text-red-600'}` }, money(item.profit))])) : [h('tr', [h('td', { colSpan: 5, class: 'px-4 py-8 text-center text-sm text-gray-500' }, t('admin.profit.noBreakdown'))])])])]) }
})
onMounted(loadStats)
</script>
