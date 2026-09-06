<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="flex items-center gap-3">
            <span class="inline-flex h-10 w-10 items-center justify-center rounded-xl bg-amber-50 text-amber-600 dark:bg-amber-500/15 dark:text-amber-400"><Icon name="chart" size="lg" /></span>
            <div>
              <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('userRanking.title') }}</h1>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('userRanking.subtitle') }}</p>
            </div>
          </div>
        </div>
        <div class="flex rounded-lg bg-gray-100 p-1 dark:bg-dark-800" role="tablist" :aria-label="t('userRanking.periodLabel')">
          <button type="button" class="min-h-10 rounded-md px-4 text-sm font-medium transition-colors" :class="period === 'day' ? 'bg-white text-emerald-600 shadow-sm dark:bg-dark-700 dark:text-emerald-400' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'" :aria-selected="period === 'day'" role="tab" @click="period = 'day'">{{ t('userRanking.day') }}</button>
          <button type="button" class="min-h-10 rounded-md px-4 text-sm font-medium transition-colors" :class="period === 'week' ? 'bg-white text-emerald-600 shadow-sm dark:bg-dark-700 dark:text-emerald-400' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'" :aria-selected="period === 'week'" role="tab" @click="period = 'week'">{{ t('userRanking.week') }}</button>
        </div>
      </header>

      <section class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <div>
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ period === 'day' ? t('userRanking.day') : t('userRanking.week') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ startDate }} {{ t('userRanking.to') }} {{ endDate }}</p>
          </div>
          <button type="button" class="btn btn-secondary min-h-10 px-3" :disabled="loading" :title="t('common.refresh')" @click="reload">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            <span class="ml-1">{{ t('common.refresh') }}</span>
          </button>
        </div>
        <UserTokenRanking ref="rankingRef" scope="user" :start-date="startDate" :end-date="endDate" :filters="{}" @loading="loading = $event" />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import UserTokenRanking from '@/components/admin/usage/UserTokenRanking.vue'

const { t } = useI18n()
const period = ref<'day' | 'week'>('day')
const rankingRef = ref<InstanceType<typeof UserTokenRanking> | null>(null)
const loading = ref(false)

function localDate(date: Date): string {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const endDate = computed(() => localDate(new Date()))
const startDate = computed(() => {
  const now = new Date()
  if (period.value === 'day') return localDate(now)
  const monday = new Date(now)
  const daysSinceMonday = (now.getDay() + 6) % 7
  monday.setDate(now.getDate() - daysSinceMonday)
  return localDate(monday)
})
function reload() { void rankingRef.value?.reload() }
</script>
