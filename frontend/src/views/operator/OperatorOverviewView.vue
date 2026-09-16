<template>
  <AppLayout>
    <div class="space-y-4">
      <header class="flex items-center justify-between">
        <h1 class="text-xl font-semibold">
          {{ t('operator.overview.title') }}
        </h1>
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          @click="load"
        >
          {{
            loading
              ? t('operator.common.loading')
              : t('operator.common.refresh')
          }}
        </button>
      </header>
      <div class="grid gap-4 lg:grid-cols-2">
        <section class="card p-4" :aria-busy="groupsLoading">
          <div class="mb-3 flex items-center justify-between gap-3">
            <h2 class="font-semibold">{{ t('operator.overview.groups') }}</h2>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="groupsLoading"
              @click="loadGroups"
            >
              {{ t('operator.common.retry') }}
            </button>
          </div>
          <p v-if="groupsLoading" class="text-sm text-gray-500" role="status">
            {{ t('operator.common.loading') }}
          </p>
          <p v-else-if="groupsError" class="text-sm text-red-600" role="alert">
            {{ groupsError }}
          </p>
          <p v-else-if="groups.length === 0" class="text-sm text-gray-500">
            {{ t('operator.overview.groupsEmpty') }}
          </p>
          <div
            v-for="group in groups"
            :key="group.id"
            class="border-b py-2 text-sm break-words"
          >
            {{ group.name }} · {{ group.platform }} · {{ group.status }} ·
            {{
              t('operator.overview.modelCount', { count: group.model_count })
            }}
          </div>
        </section>
        <section class="card p-4" :aria-busy="channelsLoading">
          <div class="mb-3 flex items-center justify-between gap-3">
            <h2 class="font-semibold">{{ t('operator.overview.channels') }}</h2>
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="channelsLoading"
              @click="loadChannels"
            >
              {{ t('operator.common.retry') }}
            </button>
          </div>
          <p v-if="channelsLoading" class="text-sm text-gray-500" role="status">
            {{ t('operator.common.loading') }}
          </p>
          <p
            v-else-if="channelsError"
            class="text-sm text-red-600"
            role="alert"
          >
            {{ channelsError }}
          </p>
          <p v-else-if="channels.length === 0" class="text-sm text-gray-500">
            {{ t('operator.overview.channelsEmpty') }}
          </p>
          <div
            v-for="channel in channels"
            :key="channel.id"
            class="border-b py-2 text-sm break-words"
          >
            {{ channel.name }} · {{ channel.status
            }}<span v-if="channel.latency_ms !== null">
              · {{ channel.latency_ms }}
              {{ t('operator.overview.latency') }}</span
            >
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  operatorAPI,
  type OperatorChannelStatus,
  type OperatorGroup,
} from '@/api/operator'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()
const router = useRouter()
let actorGeneration = 0
let groupRequest = 0
let channelRequest = 0
function actorKey() {
  const user = authStore.user
  return user && (user.role === 'admin' || user.role === 'operator')
    ? `${user.role}:${user.id}`
    : null
}
function captureActor() {
  const key = actorKey()
  const generation = actorGeneration
  return () =>
    key !== null && key === actorKey() && generation === actorGeneration
}
const groups = ref<OperatorGroup[]>([])
const channels = ref<OperatorChannelStatus[]>([])
const groupsLoading = ref(false)
const channelsLoading = ref(false)
const groupsError = ref('')
const channelsError = ref('')
const loading = computed(() => groupsLoading.value || channelsLoading.value)
async function loadGroups() {
  if (!actorKey()) return
  const current = captureActor()
  const request = ++groupRequest
  const valid = () => current() && request === groupRequest
  groupsLoading.value = true
  groupsError.value = ''
  try {
    const response = await operatorAPI.listGroups()
    if (valid()) groups.value = response.items
  } catch (err) {
    if (valid())
      groupsError.value = extractApiErrorMessage(
        err,
        t('operator.overview.loadError'),
      )
  } finally {
    if (valid()) groupsLoading.value = false
  }
}
async function loadChannels() {
  if (!actorKey()) return
  const current = captureActor()
  const request = ++channelRequest
  const valid = () => current() && request === channelRequest
  channelsLoading.value = true
  channelsError.value = ''
  try {
    const response = await operatorAPI.getChannelStatus()
    if (valid()) channels.value = response.items
  } catch (err) {
    if (valid())
      channelsError.value = extractApiErrorMessage(
        err,
        t('operator.overview.loadError'),
      )
  } finally {
    if (valid()) channelsLoading.value = false
  }
}
function load() {
  void loadGroups()
  void loadChannels()
}
watch(
  () => `${authStore.user?.role}:${authStore.user?.id}`,
  () => {
    actorGeneration += 1
    groups.value = []
    channels.value = []
    groupsError.value = ''
    channelsError.value = ''
    groupsLoading.value = false
    channelsLoading.value = false
    if (!actorKey()) {
      void router.replace(authStore.user ? '/dashboard' : '/login')
      return
    }
    load()
  },
  { immediate: true, flush: 'sync' },
)
onUnmounted(() => {
  actorGeneration += 1
})
</script>
