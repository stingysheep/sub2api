<template>
  <AppLayout>
    <div class="space-y-5">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_22rem] xl:items-start">
          <div class="min-w-0 space-y-5">
            <section class="card overflow-hidden border-emerald-300 dark:border-emerald-900/70">
              <div class="border-t-2 border-emerald-500 bg-white p-5 dark:bg-dark-800 sm:p-6">
                <div class="grid gap-5 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                  <div class="min-w-0">
                    <div class="flex items-center gap-2 text-xs font-semibold text-emerald-700 dark:text-emerald-300">
                      <Icon name="link" size="sm" />
                      <span>{{ t('affiliate.heroEyebrow') }}</span>
                    </div>
                    <h1 class="mt-3 text-2xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-3xl">
                      {{ t('affiliate.heroTitle') }}
                      <span class="text-emerald-600 dark:text-emerald-400">{{ formattedDisplayRebateRate }}%</span>
                    </h1>
                    <p class="mt-2 max-w-3xl text-sm leading-6 text-gray-600 dark:text-gray-300">{{ t('affiliate.heroDescription') }}</p>

                    <div class="mt-5 flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
                      <div class="flex min-w-0 flex-1 flex-col items-stretch gap-2 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 dark:border-dark-600 dark:bg-dark-900 sm:flex-row sm:items-center">
                        <Icon name="link" size="sm" class="shrink-0 text-emerald-600 dark:text-emerald-400" />
                        <code class="min-w-0 break-all flex-1 truncate text-xs text-gray-700 dark:text-gray-200 sm:flex-1 sm:truncate sm:text-sm">{{ inviteLink }}</code>
                      </div>
                      <button class="btn w-full shrink-0 bg-emerald-600 text-white hover:bg-emerald-700 sm:w-auto sm:shrink-0" @click="copyInviteLink">
                        <Icon name="copy" size="sm" />
                        {{ t('affiliate.copyLink') }}
                      </button>
                      <button type="button" class="btn btn-secondary w-full shrink-0 sm:w-auto" @click="showRules = true">
                        <Icon name="arrowRight" size="sm" />
                        {{ t('affiliate.rulesButton') }}
                      </button>
                    </div>

                    <div class="mt-3 flex flex-col gap-2 text-xs text-gray-500 dark:text-dark-400 sm:flex-row sm:items-center sm:justify-between">
                      <span class="flex min-w-0 flex-col items-stretch gap-1 sm:flex-row sm:items-center">{{ t('affiliate.yourCode') }}: <code class="min-w-0 break-all font-semibold text-gray-700 dark:text-gray-200 sm:flex-1 sm:truncate">{{ detail.aff_code }}</code></span>
                      <button type="button" class="btn btn-secondary btn-sm w-full shrink-0 sm:w-auto sm:shrink-0" @click="copyCode">
                        <Icon name="copy" size="sm" />
                        <span>{{ t('affiliate.copyCode') }}</span>
                      </button>
                    </div>
                  </div>
                  <div class="hidden justify-center md:flex"><AffiliateNetworkArt compact /></div>
                </div>
              </div>
            </section>

            <div class="grid gap-4 sm:grid-cols-3">
              <div class="card p-5">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</p>
                <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCount(detail.aff_count) }}</p>
              </div>
              <div class="card p-5">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
                <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">{{ formatCurrency(detail.aff_quota) }}</p>
                <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">{{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}</p>
              </div>
              <div class="card p-5">
                <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
                <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(detail.aff_history_quota) }}</p>
              </div>
            </div>

            <section class="card p-5 sm:p-6">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h2>
                  <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
                </div>
                <button class="btn btn-primary w-full sm:w-auto" :disabled="transferring || detail.aff_quota <= 0" @click="transferQuota">
                  <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
                  <Icon v-else name="dollar" size="sm" />
                  <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
                </button>
              </div>
              <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">{{ t('affiliate.transfer.empty') }}</p>
            </section>

            <section class="card p-5 sm:p-6">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invitees.title') }}</h2>
              <div v-if="detail.invitees.length === 0" class="mt-4 rounded-lg border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ t('affiliate.invitees.empty') }}</div>
              <div v-else class="mt-4 overflow-x-auto">
                <table class="w-full min-w-[560px] text-left text-sm">
                  <thead><tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400"><th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.email') }}</th><th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.username') }}</th><th class="px-3 py-2 text-right font-medium">{{ t('affiliate.invitees.columns.rebate') }}</th><th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th></tr></thead>
                  <tbody><tr v-for="item in detail.invitees" :key="item.user_id" class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"><td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.email || '-' }}</td><td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ item.username || '-' }}</td><td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrencyPrecision(item.total_rebate, 8) }}</td><td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td></tr></tbody>
                </table>
              </div>
            </section>
          </div>

          <aside class="card overflow-hidden p-0 xl:sticky xl:top-24">
            <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700">
              <div class="flex items-center justify-between gap-3">
                <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('affiliate.leaderboard.title') }}</h2>
                <Icon name="trophy" size="sm" class="text-amber-500" />
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.leaderboard.description') }}</p>
            </div>
            <div class="grid grid-cols-[2.25rem_minmax(0,1fr)_3.25rem_6.5rem] items-center gap-2 border-b border-gray-200 px-5 py-2 text-[11px] font-medium text-gray-400 dark:border-dark-700 dark:text-dark-500">
              <span>{{ t('affiliate.leaderboard.columns.rank') }}</span>
              <span>{{ t('affiliate.leaderboard.columns.account') }}</span>
              <span class="text-right">{{ t('affiliate.leaderboard.columns.invites') }}</span>
              <span class="text-right">{{ t('affiliate.leaderboard.columns.historyRebate') }}</span>
            </div>
            <div class="divide-y divide-gray-100 dark:divide-dark-700">
              <div v-for="item in (detail.leaderboard || [])" :key="item.rank" class="relative grid grid-cols-[2.25rem_minmax(0,1fr)_3.25rem_6.5rem] items-center gap-2 px-5 py-3.5" :class="item.rank === 1 ? 'bg-amber-50/80 dark:bg-amber-950/20' : item.rank === 2 ? 'bg-slate-50/80 dark:bg-slate-900/30' : item.rank === 3 ? 'bg-orange-50/80 dark:bg-orange-950/20' : 'bg-white dark:bg-dark-800'">
                <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold ring-1 ring-inset" :class="item.rank === 1 ? 'bg-amber-100 text-amber-700 ring-amber-200 dark:bg-amber-900/40 dark:text-amber-300 dark:ring-amber-700/60' : item.rank === 2 ? 'bg-slate-100 text-slate-700 ring-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:ring-slate-600' : item.rank === 3 ? 'bg-orange-100 text-orange-700 ring-orange-200 dark:bg-orange-900/30 dark:text-orange-300 dark:ring-orange-700/60' : 'bg-gray-100 text-gray-700 ring-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:ring-dark-600'">{{ item.rank }}</span>
                <p class="min-w-0 truncate text-sm text-gray-800 dark:text-gray-200">{{ item.email }}</p>
                <span class="text-right text-sm text-gray-600 dark:text-gray-300">{{ item.invited_count }}</span>
                <span class="text-right text-sm font-semibold text-emerald-600 dark:text-emerald-400">{{ formatCurrencyPrecision(item.history_rebate, 3) }}</span>
                <div class="pointer-events-none absolute inset-x-0 bottom-0 h-0.5 bg-emerald-100/80 dark:bg-emerald-950/60"><div class="h-full bg-emerald-400/80 transition-[width] duration-500 dark:bg-emerald-400/70" :style="{ width: `${leaderboardProgress(item.history_rebate)}%` }"></div></div>
              </div>
              <p v-if="(detail.leaderboard || []).length === 0" class="px-5 py-8 text-center text-sm text-gray-500">{{ t('affiliate.leaderboard.empty') }}</p>
            </div>
          </aside>
        </div>
      </template>
    </div>
    <BaseDialog :show="showRules" :title="t('affiliate.rulesTitle')" width="normal" @close="showRules = false">
      <div class="space-y-5 text-sm text-gray-600 dark:text-gray-300">
        <div class="rounded-lg border border-emerald-200 bg-emerald-50/70 p-4 dark:border-emerald-900/60 dark:bg-emerald-950/20">
          <p class="font-semibold text-emerald-800 dark:text-emerald-200">{{ t('affiliate.rulesFormulaTitle') }}</p>
          <p class="mt-2 font-mono text-xs leading-6 text-emerald-700 dark:text-emerald-300">{{ t('affiliate.rulesFormulaCode') }}</p>
        </div>
        <div>
          <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('affiliate.rulesExampleTitle') }}</h3>
          <p class="mt-2 leading-6">{{ t('affiliate.rulesExample') }}</p>
        </div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-900"><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.rulesExampleIncome') }}</p><p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ t('affiliate.rulesExampleIncomeValue') }}</p></div>
          <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-900"><p class="text-xs text-gray-500 dark:text-dark-400">{{ t('affiliate.rulesExampleCost') }}</p><p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ t('affiliate.rulesExampleCostValue') }}</p></div>
        </div>
        <p class="rounded-lg border border-gray-200 px-4 py-3 text-xs leading-5 dark:border-dark-700">{{ t('affiliate.rulesNote') }}</p>
      </div>
      <template #footer><button class="btn btn-secondary" @click="showRules = false">{{ t('common.close') }}</button></template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatCurrencyPrecision, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import AffiliateNetworkArt from '@/components/affiliate/AffiliateNetworkArt.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const showRules = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedDisplayRebateRate = computed(() => {
  const v = detail.value?.display_rebate_rate_percent ?? detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})
function formatCount(value: number): string {
  return value.toLocaleString()
}

const leaderboardMaxRebate = computed(() => {
  const rows = detail.value?.leaderboard || []
  const firstPlace = rows.find((item) => item.rank === 1)?.history_rebate
  const maxValue = firstPlace ?? Math.max(...rows.map((item) => Number(item.history_rebate) || 0), 0)
  return Math.max(Number(maxValue) || 0, 0)
})

function leaderboardProgress(value: number | null | undefined): number {
  const max = leaderboardMaxRebate.value
  if (max <= 0) return 0
  return Math.min(100, Math.max(0, ((Number(value) || 0) / max) * 100))
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    // The local preview can be used without a backend. Keep the page useful
    // with deterministic sample data while leaving normal sessions unchanged.
    const localPreviewBuild = import.meta.env.DEV || import.meta.env.VITE_LOCAL_LOGIN_SHORTCUTS === 'true'
    const localPreviewHost =
      typeof window !== 'undefined' && ['localhost', '127.0.0.1', '::1'].includes(window.location.hostname)
    if (localPreviewBuild && localPreviewHost && authStore.user?.email.endsWith('@preview.local')) {
      detail.value = {
        user_id: authStore.user.id,
        aff_code: authStore.user.role === 'admin' ? 'PREVIEW-ADMIN' : 'PREVIEW-USER',
        inviter_id: null,
        aff_count: 12,
        aff_quota: 8.4,
        aff_frozen_quota: 1.2,
        aff_history_quota: 32.6,
        effective_rebate_rate_percent: 10,
        display_rebate_rate_percent: 10,
        invitees: [],
        leaderboard: [
          { rank: 1, email: 'top@example.com', invited_count: 28, history_rebate: 96 },
          { rank: 2, email: 'growth@example.com', invited_count: 19, history_rebate: 62.4 },
          { rank: 3, email: 'creator@example.com', invited_count: 12, history_rebate: 32.6 },
        ],
      }
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
