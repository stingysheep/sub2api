<template>
  <BaseDialog :show="show" :title="t('admin.accounts.upstreamBalance.title')" width="normal" @close="emit('close')">
    <div class="space-y-4">
      <div
        v-if="account"
        class="flex items-center justify-between rounded-xl border border-emerald-200 bg-gradient-to-r from-emerald-50 to-white p-3 dark:border-emerald-800/40 dark:from-emerald-900/15 dark:to-dark-700"
      >
        <div class="min-w-0">
          <p class="truncate font-semibold text-gray-900 dark:text-gray-100">{{ account.name }}</p>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm shrink-0" :disabled="loading" @click="loadBalance">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.accounts.upstreamBalance.refresh') }}
        </button>
      </div>

      <div v-if="loading" class="flex flex-col items-center justify-center gap-3 py-10">
        <LoadingSpinner />
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.loading') }}</p>
      </div>

      <div v-else-if="errorMessage" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
        {{ errorMessage }}
      </div>

      <template v-else-if="balance">
        <div class="grid gap-3 sm:grid-cols-3">
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.provider') }}</p>
            <p class="mt-1 break-all font-medium text-gray-900 dark:text-gray-100">{{ balance.provider || '-' }}</p>
          </div>
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.fetchedAt') }}</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatDate(balance.fetched_at) }}</p>
          </div>
          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.statusCode') }}</p>
            <p class="mt-1 font-medium text-gray-900 dark:text-gray-100">{{ balance.status_code ?? '-' }}</p>
          </div>
        </div>

        <div v-if="balance.entries?.length" class="space-y-3">
          <article
            v-for="(entry, index) in balance.entries"
            :key="`${entry.plan_name || 'default'}-${index}`"
            class="rounded-xl border border-gray-200 p-4 dark:border-dark-500"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <h3 class="font-medium text-gray-900 dark:text-gray-100">{{ entry.plan_name || t('admin.accounts.upstreamBalance.unnamedPlan') }}</h3>
                <p v-if="entry.is_valid === false && entry.invalid_message" class="mt-1 text-xs text-red-600 dark:text-red-400">
                  {{ entry.invalid_message }}
                </p>
              </div>
              <span
                v-if="entry.is_valid === false"
                class="rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-950/40 dark:text-red-300"
              >{{ t('admin.accounts.upstreamBalance.invalid') }}</span>
            </div>
            <dl class="mt-4 grid grid-cols-3 gap-3">
              <div>
                <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.remaining') }}</dt>
                <dd class="mt-1 font-mono text-sm font-semibold text-emerald-700 dark:text-emerald-300">{{ formatAmount(entry.remaining, entry.unit) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.used') }}</dt>
                <dd class="mt-1 font-mono text-sm text-gray-900 dark:text-gray-100">{{ formatAmount(entry.used, entry.unit) }}</dd>
              </div>
              <div>
                <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamBalance.total') }}</dt>
                <dd class="mt-1 font-mono text-sm text-gray-900 dark:text-gray-100">{{ formatAmount(entry.total, entry.unit) }}</dd>
              </div>
            </dl>
          </article>
        </div>
        <EmptyState v-else icon="chart" :title="t('admin.accounts.upstreamBalance.empty')" />
      </template>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Account } from '@/types'
import type { UpstreamBalanceResult } from '@/api/admin/accounts'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
const loading = ref(false)
const errorMessage = ref('')
const balance = ref<UpstreamBalanceResult | null>(null)

const formatDate = (value?: string) => {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString()
}

const formatAmount = (value?: number | null, unit?: string) => {
  if (typeof value !== 'number' || !Number.isFinite(value)) return '-'
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 6 })}${unit ? ` ${unit}` : ''}`
}

const loadBalance = async () => {
  if (!props.account || loading.value) return
  loading.value = true
  errorMessage.value = ''
  try {
    balance.value = await adminAPI.accounts.getUpstreamBalance(props.account.id)
  } catch (error) {
    balance.value = null
    errorMessage.value = extractApiErrorMessage(error, t('admin.accounts.upstreamBalance.queryFailed'))
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (visible) => {
    if (visible) void loadBalance()
    else {
      balance.value = null
      errorMessage.value = ''
    }
  },
  { immediate: true }
)
</script>
