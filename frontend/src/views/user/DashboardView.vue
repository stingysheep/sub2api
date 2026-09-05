<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Keep the referral artwork independent from usage-stat loading. A transient
           stats failure must not remove the dashboard's visual entry point. -->
      <div v-if="affiliateDetail" class="card overflow-hidden border-emerald-200 dark:border-emerald-900/60">
        <div class="grid gap-5 bg-gradient-to-r from-emerald-50 via-white to-teal-50 p-5 dark:from-emerald-950/40 dark:via-dark-800 dark:to-teal-950/30 md:grid-cols-[1.5fr_1fr] md:items-center">
          <div>
            <p class="text-xs font-medium text-emerald-700 dark:text-emerald-300">{{ t('affiliate.heroEyebrow') }}</p>
            <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{{ t('affiliate.heroTitle') }} <span class="text-emerald-600 dark:text-emerald-400">{{ displayAffiliateRate }}%</span></h1>
            <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ t('affiliate.heroDescription') }}</p>
            <div class="mt-4 flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
              <div class="flex min-w-0 flex-1 items-center gap-2 rounded-lg border border-gray-200 bg-white/80 px-3 py-2 dark:border-dark-600 dark:bg-dark-900/70">
                <Icon name="link" size="sm" class="shrink-0 text-emerald-600 dark:text-emerald-400" />
                <code class="min-w-0 flex-1 truncate text-xs text-gray-700 dark:text-gray-200">{{ affiliateLink }}</code>
              </div>
              <button class="btn btn-sm w-full shrink-0 bg-emerald-600 text-white hover:bg-emerald-700 sm:w-auto" @click="copyAffiliateLink"><Icon name="copy" size="sm" />{{ t('affiliate.heroCopyLink') }}</button>
            </div>
            <div class="mt-2 flex flex-wrap items-center gap-3 text-xs">
              <button class="font-medium text-emerald-700 hover:text-emerald-800 dark:text-emerald-300 dark:hover:text-emerald-200" @click="copyAffiliateCode"><Icon name="copy" size="xs" />{{ t('affiliate.heroCopyCode') }}</button>
              <router-link class="inline-flex items-center gap-1 font-medium text-gray-600 hover:text-gray-900 dark:text-dark-300 dark:hover:text-white" to="/affiliate"><Icon name="arrowRight" size="xs" />{{ t('affiliate.viewDetails') }}</router-link>
            </div>
          </div>
          <div class="hidden justify-center md:flex"><AffiliateNetworkArt compact /></div>
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" :platform-quotas="platformQuotas" />
        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2"><UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" /></div>
          <div class="lg:col-span-1"><UserDashboardQuickActions /></div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'; import { useAuthStore } from '@/stores/auth'; import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AffiliateNetworkArt from '@/components/affiliate/AffiliateNetworkArt.vue'
import AppLayout from '@/components/layout/AppLayout.vue'; import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'; import userAPI from '@/api/user'; import type { UserAffiliateDetail } from '@/types'; import { useClipboard } from '@/composables/useClipboard'; import { useI18n } from 'vue-i18n'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'; import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'; import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'
import { isLocalPreviewSession } from '@/utils/localPreview'

const authStore = useAuthStore(); const user = computed(() => authStore.user)
const { t } = useI18n(); const { copyToClipboard } = useClipboard()
const affiliateDetail = ref<UserAffiliateDetail | null>(null)
const stats = ref<UserStatsType | null>(null); const loading = ref(false); const loadingUsage = ref(false); const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const modelStats = ref<ModelStat[]>([]); const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const affiliateLink = computed(() => affiliateDetail.value ? `${window.location.origin}/register?aff=${encodeURIComponent(affiliateDetail.value.aff_code)}` : '')
const displayAffiliateRate = computed(() => {
  const value = affiliateDetail.value?.display_rebate_rate_percent ?? affiliateDetail.value?.effective_rebate_rate_percent ?? 0
  return Number.isInteger(value) ? String(value) : String(Math.round(value * 100) / 100)
})

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000))); const endDate = ref(formatDateLocalInput(new Date())); const granularity = ref('day')

const loadStats = async () => {
  loading.value = true
  try {
    if (!isLocalPreviewSession()) await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    if (isLocalPreviewSession()) {
      stats.value = {
        total_api_keys: 0, active_api_keys: 0, total_requests: 0,
        total_input_tokens: 0, total_output_tokens: 0, total_cache_creation_tokens: 0,
        total_cache_read_tokens: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0,
        today_requests: 0, today_input_tokens: 0, today_output_tokens: 0,
        today_cache_creation_tokens: 0, today_cache_read_tokens: 0, today_tokens: 0,
        today_cost: 0, today_actual_cost: 0, average_duration_ms: 0, rpm: 0, tpm: 0,
        by_platform: [],
      }
    } else {
      console.error('Failed to load dashboard stats:', error)
    }
  } finally { loading.value = false }
}
const loadCharts = async () => { loadingCharts.value = true; try { const res = await Promise.all([usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value as any }), usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })]); trendData.value = res[0].trend || []; modelStats.value = res[1].models || [] } catch (error) { console.error('Failed to load charts:', error) } finally { loadingCharts.value = false } }
const loadRecent = async () => { loadingUsage.value = true; try { const res = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = res.items.slice(0, 5) } catch (error) { console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const data = await getMyPlatformQuotas(); platformQuotas.value = data.platform_quotas ?? [] } catch (error) { console.warn('Failed to load platform quotas:', error); platformQuotas.value = [] } }
const loadAffiliate = async () => {
  try {
    affiliateDetail.value = await userAPI.getAffiliateDetail()
  } catch {
    // Keep the animated dashboard artwork visible in the loopback preview.
    if (isLocalPreviewSession() && user.value) {
      affiliateDetail.value = {
        user_id: user.value.id,
        aff_code: user.value.role === 'admin' ? 'PREVIEW-ADMIN' : 'PREVIEW-USER',
        inviter_id: null,
        aff_count: 12,
        aff_quota: 8.4,
        aff_frozen_quota: 1.2,
        aff_history_quota: 32.6,
        effective_rebate_rate_percent: 10,
        display_rebate_rate_percent: 10,
        invitees: [],
        leaderboard: [],
      }
    } else {
      affiliateDetail.value = null
    }
  }
}
const copyAffiliateLink = async () => { if (affiliateLink.value) await copyToClipboard(affiliateLink.value, t('affiliate.linkCopied')) }
const copyAffiliateCode = async () => { if (affiliateDetail.value?.aff_code) await copyToClipboard(affiliateDetail.value.aff_code, t('affiliate.codeCopied')) }
const refreshAll = () => { loadStats(); loadCharts(); loadRecent(); loadPlatformQuotas(); loadAffiliate() }

onMounted(() => { refreshAll() })
</script>
