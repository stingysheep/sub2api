<template>
  <AppLayout>
    <div class="space-y-4">
      <header class="flex flex-wrap items-center justify-between gap-3">
        <h1 class="text-xl font-semibold">{{ t('operator.users.title') }}</h1>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="usersLoading"
          @click="loadUsers"
        >
          {{
            usersLoading
              ? t('operator.common.loading')
              : t('operator.common.refresh')
          }}
        </button>
      </header>
      <form class="card flex gap-3 p-4" @submit.prevent="searchUsers">
        <label class="sr-only" for="operator-user-search">{{
          t('operator.users.search')
        }}</label
        ><input
          id="operator-user-search"
          v-model="search"
          class="input flex-1"
          :placeholder="t('operator.users.searchPlaceholder')"
          autocomplete="off"
        /><button
          type="submit"
          class="btn btn-primary"
          :disabled="usersLoading"
        >
          {{ t('operator.users.search') }}
        </button>
      </form>
      <div class="card overflow-x-auto">
        <p v-if="usersLoading" class="p-4 text-sm text-gray-500" role="status">
          {{ t('operator.common.loading') }}
        </p>
        <p v-else-if="usersError" class="p-4 text-sm text-red-600" role="alert">
          {{ usersError }}
        </p>
        <p v-else-if="users.length === 0" class="p-4 text-sm text-gray-500">
          {{ t('operator.users.empty') }}
        </p>
        <table v-else class="w-full text-sm">
          <thead>
            <tr>
              <th scope="col">{{ t('operator.users.user') }}</th>
              <th scope="col">{{ t('operator.users.status') }}</th>
              <th scope="col">{{ t('operator.users.balance') }}</th>
              <th scope="col">
                <span class="sr-only">{{ t('operator.users.details') }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="user in users" :key="user.id">
              <td class="break-all">
                {{ user.email }}<small class="block">{{ user.username }}</small>
              </td>
              <td>{{ t(`operator.users.statuses.${user.status}`) }}</td>
              <td>{{ formatAmount(user.balance) }}</td>
              <td>
                <button
                  type="button"
                  class="btn btn-secondary"
                  @click="openUser(user.id)"
                >
                  {{ t('operator.users.details') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Pagination
        v-if="userPagination.total > 0"
        :page="userPagination.page"
        :total="userPagination.total"
        :page-size="userPagination.page_size"
        :show-page-size-selector="false"
        @update:page="changeUserPage"
      />
      <section v-if="selected" class="card space-y-4 p-4">
        <div>
          <h2 class="text-lg font-semibold break-all">{{ selected.email }}</h2>
          <p class="text-sm text-gray-500">{{ selected.username }}</p>
          <div class="mt-2 grid gap-2 text-sm sm:grid-cols-3">
            <p>
              <span class="text-gray-500"
                >{{ t('operator.users.totalBalance') }}:</span
              >
              {{ formatAmount(selected.balance) }}
            </p>
            <p>
              <span class="text-gray-500"
                >{{ t('operator.users.freeBalance') }}:</span
              >
              {{ formatAmount(selected.free_balance) }}
            </p>
            <p>
              <span class="text-gray-500"
                >{{ t('operator.users.paidBalance') }}:</span
              >
              {{ formatAmount(selected.paid_balance) }}
            </p>
          </div>
        </div>
        <p
          v-if="identityError"
          class="rounded bg-red-50 p-3 text-sm text-red-700"
          role="alert"
        >
          {{ identityError }}
        </p>
        <div
          class="tabs flex flex-wrap"
          role="tablist"
          :aria-label="t('operator.users.detailTabs')"
        >
          <button
            v-for="(tab, index) in tabs"
            :id="`operator-tab-${tab.value}`"
            :key="tab.value"
            type="button"
            role="tab"
            class="tab"
            :class="activeTab === tab.value ? 'tab-active' : ''"
            :aria-selected="activeTab === tab.value"
            :aria-controls="`operator-panel-${tab.value}`"
            :tabindex="activeTab === tab.value ? 0 : -1"
            @click="selectTab(tab.value)"
            @keydown="onTabKeydown($event, index)"
          >
            {{ t(tab.label) }}
          </button>
        </div>
        <section
          v-if="activeTab === 'ledger'"
          id="operator-panel-ledger"
          role="tabpanel"
          aria-labelledby="operator-tab-ledger"
        >
          <div class="flex justify-between gap-3">
            <h3 class="font-medium">{{ t('operator.users.tabs.ledger') }}</h3>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="ledgerState.loading"
              @click="loadLedger"
            >
              {{ t('operator.common.retry') }}
            </button>
          </div>
          <p
            v-if="ledgerState.loading"
            class="mt-3 text-sm text-gray-500"
            role="status"
          >
            {{ t('operator.common.loading') }}
          </p>
          <p
            v-else-if="ledgerState.error"
            class="mt-3 text-sm text-red-600"
            role="alert"
          >
            {{ ledgerState.error }}
          </p>
          <p v-else-if="ledger.length === 0" class="mt-3 text-sm text-gray-500">
            {{ t('operator.users.ledgerEmpty') }}
          </p>
          <div v-else class="mt-3 space-y-2 text-sm">
            <p
              v-for="item in ledger"
              :key="item.id"
              class="whitespace-pre-wrap break-words"
            >
              {{ formatDateTime(item.time) }} · {{ item.operation }}
              {{ formatAmount(item.amount) }} · {{ item.reason }}
            </p>
          </div>
          <Pagination
            v-if="ledgerPagination.total > 0"
            class="mt-3"
            :page="ledgerPagination.page"
            :total="ledgerPagination.total"
            :page-size="ledgerPagination.page_size"
            :show-page-size-selector="false"
            @update:page="changeLedgerPage"
          />
        </section>
        <section
          v-else-if="activeTab === 'orders'"
          id="operator-panel-orders"
          role="tabpanel"
          aria-labelledby="operator-tab-orders"
        >
          <div class="flex justify-between gap-3">
            <h3 class="font-medium">{{ t('operator.users.tabs.orders') }}</h3>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="ordersState.loading"
              @click="loadOrders"
            >
              {{ t('operator.common.retry') }}
            </button>
          </div>
          <p
            v-if="ordersState.loading"
            class="mt-3 text-sm text-gray-500"
            role="status"
          >
            {{ t('operator.common.loading') }}
          </p>
          <p
            v-else-if="ordersState.error"
            class="mt-3 text-sm text-red-600"
            role="alert"
          >
            {{ ordersState.error }}
          </p>
          <p v-else-if="orders.length === 0" class="mt-3 text-sm text-gray-500">
            {{ t('operator.users.ordersEmpty') }}
          </p>
          <div v-else class="mt-3 space-y-2 text-sm">
            <p v-for="item in orders" :key="item.id">
              #{{ item.id }} {{ formatAmount(item.amount) }}
              {{ item.currency }} · {{ item.status }} ·
              {{ formatDateTime(item.created_at) }}
            </p>
          </div>
          <Pagination
            v-if="ordersPagination.total > 0"
            class="mt-3"
            :page="ordersPagination.page"
            :total="ordersPagination.total"
            :page-size="ordersPagination.page_size"
            :show-page-size-selector="false"
            @update:page="changeOrdersPage"
          />
        </section>
        <section
          v-else-if="activeTab === 'usage'"
          id="operator-panel-usage"
          role="tabpanel"
          aria-labelledby="operator-tab-usage"
        >
          <label class="input-label" for="operator-usage-period">{{
            t('operator.users.usagePeriod')
          }}</label
          ><select
            id="operator-usage-period"
            v-model="usagePeriod"
            class="input max-w-xs"
            @change="loadUsage"
          >
            <option
              v-for="period in usagePeriods"
              :key="period"
              :value="period"
            >
              {{ t(`operator.users.periods.${period}`) }}
            </option></select
          ><button
            type="button"
            class="btn btn-secondary ml-2"
            :disabled="usageState.loading"
            @click="loadUsage"
          >
            {{ t('operator.common.retry') }}
          </button>
          <p
            v-if="usageState.loading"
            class="mt-3 text-sm text-gray-500"
            role="status"
          >
            {{ t('operator.common.loading') }}
          </p>
          <p
            v-else-if="usageState.error"
            class="mt-3 text-sm text-red-600"
            role="alert"
          >
            {{ usageState.error }}
          </p>
          <p
            v-else-if="usage"
            class="mt-3 text-sm whitespace-pre-wrap break-words"
          >
            {{
              t('operator.users.usageSummary', {
                period: usage.period,
                requests: usage.request_count,
                tokens: usage.total_tokens,
                amount: formatAmount(usage.usage_amount),
                start: formatDateTime(usage.start_at),
                end: formatDateTime(usage.end_at),
              })
            }}
          </p>
          <p v-else class="mt-3 text-sm text-gray-500">
            {{ t('operator.users.usageEmpty') }}
          </p>
        </section>
        <form
          v-else
          id="operator-panel-adjust"
          class="grid max-w-md gap-2"
          role="tabpanel"
          aria-labelledby="operator-tab-adjust"
          @submit.prevent="adjust"
        >
          <p
            v-if="recoveredPending"
            class="rounded bg-amber-50 p-3 text-sm text-amber-800"
            role="alert"
          >
            {{ t('operator.users.recoveredPending')
            }}<button
              type="button"
              class="mt-2 btn btn-secondary"
              :disabled="selected.id !== recoveredPending.targetId"
              @click="acknowledgeRecoveredPending"
            >
              {{ t('operator.users.acknowledgeLedger') }}
            </button>
          </p>
          <p
            v-if="pendingAdjustment"
            class="rounded bg-amber-50 p-3 text-sm text-amber-800"
            role="status"
          >
            {{ pendingNotice }}
          </p>
          <p
            v-if="adjustmentResult"
            class="rounded bg-blue-50 p-3 text-sm text-blue-800"
            role="status"
          >
            {{ adjustmentResult }}
          </p>
          <label class="input-label" for="operator-adjust-operation">{{
            t('operator.users.operation')
          }}</label
          ><select
            id="operator-adjust-operation"
            v-model="adjustment.operation"
            class="input"
            :disabled="adjustmentLocked"
          >
            <option value="add">{{ t('operator.users.adjustAdd') }}</option>
            <option value="subtract">
              {{ t('operator.users.adjustSubtract') }}
            </option></select
          ><label
            v-if="adjustment.operation === 'add'"
            class="input-label"
            for="operator-adjust-source"
            >{{ t('operator.users.source') }}</label
          ><select
            v-if="adjustment.operation === 'add'"
            id="operator-adjust-source"
            v-model="adjustment.source"
            class="input"
            :disabled="adjustmentLocked"
          >
            <option value="free">{{ t('operator.users.freeBalance') }}</option>
            <option value="paid">
              {{ t('operator.users.paidBalance') }}
            </option></select
          ><label class="input-label" for="operator-adjust-amount">{{
            t('operator.users.amount')
          }}</label
          ><input
            id="operator-adjust-amount"
            v-model="adjustment.amount"
            class="input"
            inputmode="decimal"
            :placeholder="t('operator.users.amountPlaceholder')"
            :disabled="adjustmentLocked"
          /><label class="input-label" for="operator-adjust-reason">{{
            t('operator.users.reason')
          }}</label
          ><textarea
            id="operator-adjust-reason"
            v-model="adjustment.reason"
            class="input min-h-24"
            :placeholder="t('operator.users.reasonPlaceholder')"
            :disabled="adjustmentLocked"
          />
          <p v-if="adjustmentError" class="text-sm text-red-600" role="alert">
            {{ adjustmentError }}
          </p>
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="
              saving || Boolean(recoveredPending) || Boolean(identityError)
            "
          >
            {{
              saving
                ? t('operator.common.loading')
                : pendingAdjustment
                  ? t('operator.users.retryAdjustment')
                  : t('operator.users.submitAdjustment')
            }}
          </button>
        </form>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import {
  operatorAPI,
  type OperatorBalanceAdjustmentRequest,
  type OperatorBalanceAdjustmentResponse,
} from '@/api/operator'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAuthStore } from '@/stores/auth'
import {
  useOperatorPendingAdjustment,
  type OperatorPendingAdjustmentRecord,
} from '@/composables/useOperatorPendingAdjustment'
import { useOperatorUserReads } from '@/composables/useOperatorUserReads'

type DetailTab = 'ledger' | 'orders' | 'usage' | 'adjust'
type PendingAdjustment = {
  targetId: number
  payload: OperatorBalanceAdjustmentRequest
  idempotencyKey: string
  cacheSynced: boolean
}

const { t, locale } = useI18n()
const authStore = useAuthStore()
const router = useRouter()
const pendingStorage = useOperatorPendingAdjustment()
const reads = useOperatorUserReads(() => {
  const actor = currentActor()
  return actor ? `${actor.role}:${actor.id}` : null
}, t)
const {
  users,
  search,
  selected,
  ledger,
  orders,
  usage,
  usersLoading,
  usersError,
  ledgerState,
  ordersState,
  usageState,
  usagePeriod,
  userPagination,
  ledgerPagination,
  ordersPagination,
  loadUsers,
  loadLedger,
  loadOrders,
  loadUsage,
  searchUsers,
  changeUserPage,
  changeLedgerPage,
  changeOrdersPage,
} = reads
const activeTab = ref<DetailTab>('ledger')
const saving = ref(false)
const adjustmentError = ref('')
const identityError = ref('')
const pendingAdjustment = ref<PendingAdjustment | null>(null)
const recoveredPending = ref<OperatorPendingAdjustmentRecord | null>(null)
const lastAdjustment = ref<OperatorBalanceAdjustmentResponse | null>(null)
const adjustment = reactive({
  operation: 'add' as 'add' | 'subtract',
  amount: '',
  source: 'free' as 'free' | 'paid',
  reason: '',
})
const usagePeriods = ['today', '7d', '30d', '90d'] as const
const tabs: Array<{ value: DetailTab; label: string }> = [
  { value: 'ledger', label: 'operator.users.tabs.ledger' },
  { value: 'orders', label: 'operator.users.tabs.orders' },
  { value: 'usage', label: 'operator.users.tabs.usage' },
  { value: 'adjust', label: 'operator.users.tabs.adjust' },
]
const adjustmentLocked = computed(
  () => Boolean(pendingAdjustment.value) || saving.value,
)
const pendingNotice = computed(() =>
  pendingAdjustment.value?.cacheSynced === false
    ? t('operator.users.cachePending')
    : t('operator.users.retryPending'),
)
const adjustmentResult = computed(() =>
  lastAdjustment.value
    ? t('operator.users.adjustmentResult', {
        before: formatAmount(lastAdjustment.value.before_balance),
        after: formatAmount(lastAdjustment.value.after_balance),
      })
    : '',
)

function currentActor(): { id: number; role: 'operator' | 'admin' } | null {
  const user = authStore.user
  return user && (user.role === 'operator' || user.role === 'admin')
    ? { id: user.id, role: user.role }
    : null
}

function formatAmount(value: string | number) {
  const raw = String(value)
  const match = raw.match(/^(-?)(\d+)(?:\.(\d{1,8}))?$/)
  if (!match) return raw
  const separators = new Intl.NumberFormat(
    locale?.value || undefined,
  ).formatToParts(1000.1)
  const group = separators.find((part) => part.type === 'group')?.value ?? ','
  const decimal =
    separators.find((part) => part.type === 'decimal')?.value ?? '.'
  return `${match[1]}${match[2].replace(/\B(?=(\d{3})+(?!\d))/g, group)}${decimal}${(match[3] ?? '').padEnd(8, '0')}`
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale?.value || undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(date)
}

function actorOrFail() {
  const actor = currentActor()
  if (!actor) identityError.value = t('operator.users.authorizationChanged')
  return actor
}

function restorePending() {
  reads.reset()
  pendingAdjustment.value = null
  recoveredPending.value = null
  lastAdjustment.value = null
  saving.value = false
  adjustmentError.value = ''
  identityError.value = ''
  Object.assign(adjustment, {
    operation: 'add',
    amount: '',
    source: 'free',
    reason: '',
  })
  const actor = actorOrFail()
  if (!actor) return
  try {
    recoveredPending.value = pendingStorage.read(actor.id, actor.role)
  } catch {
    identityError.value = t('operator.users.pendingStorageUnavailable')
  }
}

async function openUser(id: number) {
  lastAdjustment.value = null
  adjustmentError.value = ''
  activeTab.value = 'ledger'
  await reads.openUser(id)
}

function selectTab(tab: DetailTab) {
  activeTab.value = tab
}
function onTabKeydown(event: KeyboardEvent, index: number) {
  if (!['ArrowRight', 'ArrowLeft', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next =
    event.key === 'Home'
      ? 0
      : event.key === 'End'
        ? tabs.length - 1
        : (index + (event.key === 'ArrowRight' ? 1 : tabs.length - 1)) %
          tabs.length
  selectTab(tabs[next].value)
  document.getElementById(`operator-tab-${tabs[next].value}`)?.focus()
}

function validAmount(value: string) {
  return /^\d{1,12}(\.\d{1,8})?$/.test(value) && /[1-9]/.test(value)
}
function makePayload(): OperatorBalanceAdjustmentRequest | null {
  const amount = adjustment.amount.trim()
  const reason = adjustment.reason.trim()
  if (
    !validAmount(amount) ||
    !reason ||
    new TextEncoder().encode(reason).length > 1000
  )
    return null
  return adjustment.operation === 'subtract'
    ? { operation: 'subtract', amount, reason }
    : { operation: 'add', amount, source: adjustment.source, reason }
}
function newKey() {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random()}`
}

async function adjust() {
  if (saving.value || recoveredPending.value || identityError.value) return
  adjustmentError.value = ''
  const actor = actorOrFail()
  if (!actor || !selected.value) return
  const currentActor = reads.capture()
  const currentSelection = reads.captureSelection()
  const current = pendingAdjustment.value
  const payload = current?.payload ?? makePayload()
  if (!payload) {
    adjustmentError.value = t('operator.users.invalidAdjustment')
    return
  }
  const pending = current ?? {
    targetId: selected.value.id,
    payload,
    idempotencyKey: newKey(),
    cacheSynced: true,
  }
  if (pending.targetId !== selected.value.id) {
    adjustmentError.value = t('operator.users.resolvePending')
    return
  }
  if (!current) {
    try {
      pendingStorage.save({
        actorId: actor.id,
        role: actor.role,
        targetId: pending.targetId,
        idempotencyKey: pending.idempotencyKey,
        createdAt: new Date().toISOString(),
      })
    } catch {
      identityError.value = t('operator.users.pendingStorageUnavailable')
      return
    }
  }
  pendingAdjustment.value = pending
  saving.value = true
  try {
    const response = await operatorAPI.adjustBalance(
      pending.targetId,
      pending.payload,
      pending.idempotencyKey,
    )
    // Retain the old actor's recovery credential if authorization/selection changed.
    if (!currentSelection()) return
    lastAdjustment.value = response
    pendingAdjustment.value = { ...pending, cacheSynced: response.cache_synced }
    if (response.cache_synced) {
      pendingStorage.clear(actor.id, actor.role)
      pendingAdjustment.value = null
      // Refresh only this still-current selection; never reopen an old target.
      try {
        const refreshed = await operatorAPI.getUser(pending.targetId)
        if (currentSelection()) {
          selected.value = refreshed
          void loadLedger()
        }
      } catch (error) {
        if (currentSelection())
          usersError.value = extractApiErrorMessage(
            error,
            t('operator.users.detailLoadError'),
          )
      }
    }
  } catch (error) {
    if (!currentSelection()) return
    const status =
      typeof error === 'object' && error !== null && 'status' in error
        ? (error as { status?: unknown }).status
        : undefined
    if (
      !current &&
      typeof status === 'number' &&
      [400, 401, 403, 404, 413, 422, 423, 429].includes(status)
    ) {
      try {
        pendingStorage.clear(actor.id, actor.role)
        pendingAdjustment.value = null
      } catch {
        identityError.value = t('operator.users.pendingStorageUnavailable')
      }
    }
    adjustmentError.value = extractApiErrorMessage(
      error,
      t('operator.users.adjustmentFailed'),
    )
  } finally {
    if (currentActor()) saving.value = false
  }
}

function acknowledgeRecoveredPending() {
  const actor = actorOrFail()
  if (
    !actor ||
    !recoveredPending.value ||
    selected.value?.id !== recoveredPending.value.targetId ||
    !window.confirm(t('operator.users.acknowledgeLedgerConfirm'))
  )
    return
  try {
    pendingStorage.clear(actor.id, actor.role)
    recoveredPending.value = null
  } catch {
    identityError.value = t('operator.users.pendingStorageUnavailable')
  }
}
function warnBeforeUnload(event: BeforeUnloadEvent) {
  if (!saving.value && !pendingAdjustment.value) return
  event.preventDefault()
  event.returnValue = ''
}
onBeforeRouteLeave(() => {
  if (!saving.value && !pendingAdjustment.value) return true
  return window.confirm(t('operator.users.leavePendingWarning'))
})
watch(
  () => `${authStore.user?.id ?? 'none'}:${authStore.user?.role ?? 'none'}`,
  () => {
    restorePending()
    if (!currentActor()) {
      void router.replace(authStore.user ? '/dashboard' : '/login')
      return
    }
    void loadUsers()
  },
  { immediate: true, flush: 'sync' },
)
onMounted(() => window.addEventListener('beforeunload', warnBeforeUnload))
onUnmounted(() => {
  reads.reset()
  window.removeEventListener('beforeunload', warnBeforeUnload)
})
</script>
