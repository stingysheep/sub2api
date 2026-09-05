<template>
  <aside class="upstream-profile-nav flex min-h-0 w-full shrink-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
    <header class="flex items-start justify-between gap-3 border-b border-gray-100 px-3 py-3 dark:border-dark-700">
      <div class="min-w-0">
        <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.accounts.upstreamProfiles.title') }}</h2>
        <p class="mt-1 text-xs leading-4 text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamProfiles.description') }}</p>
      </div>
      <div class="flex shrink-0 items-center gap-1">
        <button
          type="button"
          class="icon-btn"
          :class="reorderMode ? 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-300' : 'text-gray-500'"
          :aria-pressed="reorderMode"
          :title="reorderMode ? t('admin.accounts.upstreamProfiles.finishReorder') : t('admin.accounts.upstreamProfiles.reorder')"
          @click="reorderMode = !reorderMode"
        >
          <Icon :name="reorderMode ? 'check' : 'menu'" size="sm" />
        </button>
        <button type="button" class="icon-btn text-gray-500" :title="t('admin.accounts.upstreamProfiles.manage')" @click="openEditor">
          <Icon name="cog" size="sm" />
        </button>
      </div>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-2">
      <button
        type="button"
        class="profile-nav-item"
        :class="activeProfileId === 'all' && 'profile-nav-item-active'"
        @click="selectProfile('all')"
      >
        <Icon name="grid" size="sm" class="shrink-0" />
        <span class="min-w-0 flex-1 truncate">{{ t('admin.accounts.upstreamProfiles.allAccounts') }}</span>
        <span class="profile-count">{{ props.accounts?.length || 0 }}</span>
      </button>

      <VueDraggable
        v-model="orderedProfiles"
        class="mt-1 space-y-1"
        handle=".profile-drag-handle"
        :animation="180"
        :disabled="!reorderMode || savingOrder"
        data-testid="upstream-profile-order-list"
        @end="handleProfileDragEnd"
      >
        <div v-for="profile in orderedProfiles" :key="profile.id" class="profile-nav-group">
          <div class="profile-nav-row">
            <button
              type="button"
              class="profile-nav-item min-w-0 flex-1"
              :class="isProfileActive(profile.id) && 'profile-nav-item-active'"
              @click="selectProfile(profile.id)"
            >
              <Icon v-if="reorderMode" name="gripVertical" size="sm" class="profile-drag-handle shrink-0 cursor-grab text-gray-400 active:cursor-grabbing" :title="t('admin.accounts.upstreamProfiles.dragProfile')" />
              <Icon v-else name="server" size="sm" class="shrink-0 text-gray-400" />
              <span class="min-w-0 flex-1 truncate text-left">{{ profile.name }}</span>
              <span class="profile-count">{{ profileAccountCount(profile.id) }}</span>
              <span v-if="!profile.enabled" class="profile-disabled-dot" :title="t('common.disabled')" />
            </button>
            <button
              v-if="profilePlatformItems(profile.id).length"
              type="button"
              class="profile-expand-btn"
              :aria-label="isExpanded(profile.id) ? t('admin.accounts.upstreamProfiles.collapsePlatforms') : t('admin.accounts.upstreamProfiles.expandPlatforms')"
              :title="isExpanded(profile.id) ? t('admin.accounts.upstreamProfiles.collapsePlatforms') : t('admin.accounts.upstreamProfiles.expandPlatforms')"
              @click="toggleExpanded(profile.id)"
            >
              <Icon :name="isExpanded(profile.id) ? 'chevronDown' : 'chevronRight'" size="xs" />
            </button>
          </div>
          <div v-if="isExpanded(profile.id)" class="profile-platform-list">
            <button
              v-for="item in profilePlatformItems(profile.id)"
              :key="item.platform"
              type="button"
              class="profile-platform-item"
              :class="isPlatformActive(profile.id, item.platform) && 'profile-platform-item-active'"
              @click="selectPlatform(profile.id, item.platform)"
            >
              <PlatformIcon :platform="item.platform" size="xs" class="shrink-0" />
              <span class="min-w-0 flex-1 truncate text-left">{{ item.label }}</span>
              <span class="profile-count">{{ item.count }}</span>
            </button>
          </div>
        </div>
      </VueDraggable>

      <div class="profile-nav-group mt-1 border-t border-dashed border-gray-200 pt-2 dark:border-dark-700">
        <div class="profile-nav-row">
          <button
            type="button"
            class="profile-nav-item min-w-0 flex-1"
            :class="isProfileActive('unassigned') && 'profile-nav-item-active'"
            @click="selectProfile('unassigned')"
          >
            <Icon name="inbox" size="sm" class="shrink-0 text-gray-400" />
            <span class="min-w-0 flex-1 truncate text-left">{{ t('admin.accounts.upstreamProfiles.unassigned') }}</span>
            <span class="profile-count">{{ unassignedCount }}</span>
          </button>
          <button
            v-if="profilePlatformItems('unassigned').length"
            type="button"
            class="profile-expand-btn"
            :aria-label="isExpanded('unassigned') ? t('admin.accounts.upstreamProfiles.collapsePlatforms') : t('admin.accounts.upstreamProfiles.expandPlatforms')"
            :title="isExpanded('unassigned') ? t('admin.accounts.upstreamProfiles.collapsePlatforms') : t('admin.accounts.upstreamProfiles.expandPlatforms')"
            @click="toggleExpanded('unassigned')"
          >
            <Icon :name="isExpanded('unassigned') ? 'chevronDown' : 'chevronRight'" size="xs" />
          </button>
        </div>
        <div v-if="isExpanded('unassigned')" class="profile-platform-list">
          <button
            v-for="item in profilePlatformItems('unassigned')"
            :key="item.platform"
            type="button"
            class="profile-platform-item"
            :class="isPlatformActive('unassigned', item.platform) && 'profile-platform-item-active'"
            @click="selectPlatform('unassigned', item.platform)"
          >
            <PlatformIcon :platform="item.platform" size="xs" class="shrink-0" />
            <span class="min-w-0 flex-1 truncate text-left">{{ item.label }}</span>
            <span class="profile-count">{{ item.count }}</span>
          </button>
        </div>
      </div>
    </div>
  </aside>

  <BaseDialog :show="showEditor" :title="t('admin.accounts.upstreamProfiles.manage')" width="wide" @close="closeEditor">
    <div class="space-y-3">
      <div v-for="(profile, index) in draftProfiles" :key="profile.id || `new-${index}`" class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
        <div class="grid gap-3 md:grid-cols-2">
          <label class="block">
            <span class="input-label">{{ t('admin.accounts.upstreamProfiles.name') }}</span>
            <input v-model="profile.name" class="input" :placeholder="t('admin.accounts.upstreamProfiles.namePlaceholder')" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.accounts.upstreamProfiles.namePrefix') }}</span>
            <input v-model="profile.name_prefix" class="input font-mono" :placeholder="t('admin.accounts.upstreamProfiles.namePrefixPlaceholder')" />
          </label>
          <label class="block">
            <span class="input-label">{{ t('admin.accounts.upstreamProfiles.baseUrl') }}</span>
            <input v-model="profile.base_url" class="input font-mono" :placeholder="t('admin.accounts.upstreamProfiles.baseUrlPlaceholder')" />
          </label>
        </div>
        <div class="mt-3 flex items-center justify-between gap-3">
          <label class="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="profile.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
            {{ t('admin.accounts.upstreamProfiles.enabled') }}
          </label>
          <button type="button" class="p-1.5 text-gray-400 hover:text-red-600" :title="t('admin.accounts.upstreamProfiles.remove')" @click="removeProfile(index)">
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>
      <button type="button" class="w-full rounded-md border border-dashed border-gray-300 px-3 py-2 text-sm text-gray-500 hover:border-primary-400 hover:text-primary-600 dark:border-dark-600 dark:text-gray-400" @click="addProfile">
        <Icon name="plus" size="sm" class="mr-1" />
        {{ t('admin.accounts.upstreamProfiles.add') }}
      </button>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" @click="closeEditor">{{ t('common.cancel') }}</button>
      <button type="button" class="btn btn-primary" :disabled="saving" @click="saveProfiles">
        {{ saving ? t('common.loading') : t('common.save') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { VueDraggable } from 'vue-draggable-plus'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { UpstreamProviderProfile } from '@/api/admin/settings'
import type { Account, AccountPlatform } from '@/types'

export type ProfileSelection = 'all' | 'unassigned' | number | {
  profileId: number | 'unassigned'
  platform: AccountPlatform
}

const PLATFORM_ORDER: AccountPlatform[] = [
  'openai',
  'anthropic',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek'
]
const PLATFORM_LABELS: Record<AccountPlatform, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu GLM',
  deepseek: 'DeepSeek'
}

const props = defineProps<{
  profiles: UpstreamProviderProfile[]
  accounts?: Account[]
  /** Full filtered account set used for stable profile counts. */
  profileAccounts?: Account[]
  activeProfileId: ProfileSelection
}>()
const emit = defineEmits<{
  updated: [profiles: UpstreamProviderProfile[]]
  select: [profileId: ProfileSelection]
}>()
const { t } = useI18n()
const appStore = useAppStore()
const showEditor = ref(false)
const saving = ref(false)
const savingOrder = ref(false)
const reorderMode = ref(false)
const draftProfiles = ref<UpstreamProviderProfile[]>([])
const orderedProfiles = ref<UpstreamProviderProfile[]>([])
const expandedProfileIds = ref<Set<number | 'unassigned'>>(new Set())

const profileIDFor = (account: Account) => {
  const accountWithProfile = account as Account & { upstream_provider_profile_id?: number | string | null }
  const value = accountWithProfile.upstream_provider_profile_id ?? account.extra?.upstream_provider_profile_id
  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : null
}

const profilePlatformItems = (profileID: number | 'unassigned') => {
  const items = new Map<AccountPlatform, number>()
  for (const account of props.profileAccounts || props.accounts || []) {
    const accountProfileID = profileIDFor(account)
    const matchesProfile = profileID === 'unassigned' ? accountProfileID === null : accountProfileID === profileID
    if (!matchesProfile || !PLATFORM_ORDER.includes(account.platform)) continue
    items.set(account.platform, (items.get(account.platform) || 0) + 1)
  }
  return PLATFORM_ORDER
    .filter(platform => items.has(platform))
    .map(platform => ({ platform, label: PLATFORM_LABELS[platform], count: items.get(platform) || 0 }))
}

const isPlatformSelection = (selection: ProfileSelection): selection is Exclude<ProfileSelection, 'all' | 'unassigned' | number> =>
  typeof selection === 'object' && selection !== null

const isProfileActive = (profileID: number | 'unassigned') => {
  if (props.activeProfileId === profileID) return true
  return isPlatformSelection(props.activeProfileId) && props.activeProfileId.profileId === profileID
}

const isPlatformActive = (profileID: number | 'unassigned', platform: AccountPlatform) =>
  isPlatformSelection(props.activeProfileId) &&
  props.activeProfileId.profileId === profileID &&
  props.activeProfileId.platform === platform

const isExpanded = (profileID: number | 'unassigned') => expandedProfileIds.value.has(profileID)

const toggleExpanded = (profileID: number | 'unassigned') => {
  const next = new Set(expandedProfileIds.value)
  if (next.has(profileID)) next.delete(profileID)
  else next.add(profileID)
  expandedProfileIds.value = next
}

const unassignedCount = computed(() => (props.profileAccounts || props.accounts || []).filter(account => !profileIDFor(account)).length)
const profileAccountCount = (profileID: number) => (props.profileAccounts || props.accounts || []).filter(account => profileIDFor(account) === profileID).length

watch(() => props.profiles, (profiles) => {
  orderedProfiles.value = [...profiles].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id).map(profile => ({ ...profile }))
}, { immediate: true, deep: true })

const selectProfile = (profileID: ProfileSelection) => emit('select', profileID)
const selectPlatform = (profileID: number | 'unassigned', platform: AccountPlatform) => {
  expandedProfileIds.value = new Set([...expandedProfileIds.value, profileID])
  emit('select', { profileId: profileID, platform })
}

watch(() => props.activeProfileId, (selection) => {
  if (selection === 'unassigned') {
    expandedProfileIds.value = new Set([...expandedProfileIds.value, 'unassigned'])
  } else if (typeof selection === 'number') {
    expandedProfileIds.value = new Set([...expandedProfileIds.value, selection])
  } else if (isPlatformSelection(selection)) {
    expandedProfileIds.value = new Set([...expandedProfileIds.value, selection.profileId])
  }
}, { immediate: true, deep: true })

const handleProfileDragEnd = async () => {
  if (!reorderMode.value || savingOrder.value) return
  const previous = [...props.profiles]
  savingOrder.value = true
  try {
    const updates = orderedProfiles.value.map((profile, index) => ({ ...profile, sort_order: index * 10 }))
    const saved = await adminAPI.settings.updateUpstreamProviderProfiles(updates)
    orderedProfiles.value = saved.map(profile => ({ ...profile }))
    emit('updated', saved)
    appStore.showSuccess(t('admin.accounts.upstreamProfiles.orderSaved'))
  } catch (error) {
    orderedProfiles.value = [...previous].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id).map(profile => ({ ...profile }))
    console.error('Failed to save upstream provider profile order:', error)
    appStore.showError(t('admin.accounts.upstreamProfiles.orderSaveFailed'))
  } finally {
    savingOrder.value = false
  }
}

const openEditor = () => {
  draftProfiles.value = orderedProfiles.value.map(profile => ({ ...profile }))
  showEditor.value = true
}
const closeEditor = () => { showEditor.value = false }
const addProfile = () => {
  const maxOrder = Math.max(-10, ...draftProfiles.value.map(profile => profile.sort_order ?? 0))
  const nextID = Math.max(0, ...draftProfiles.value.map(profile => profile.id)) + 1
  draftProfiles.value.push({ id: nextID, sort_order: maxOrder + 10, name: '', name_prefix: '', base_url: '', enabled: true })
}
const removeProfile = (index: number) => { draftProfiles.value.splice(index, 1) }
const saveProfiles = async () => {
  const profiles = draftProfiles.value.map((profile, index) => ({ ...profile, sort_order: index * 10 }))
  if (profiles.some(profile => !profile.name.trim())) {
    appStore.showError(t('admin.accounts.upstreamProfiles.nameRequired'))
    return
  }
  saving.value = true
  try {
    const saved = await adminAPI.settings.updateUpstreamProviderProfiles(profiles)
    orderedProfiles.value = saved.map(profile => ({ ...profile }))
    emit('updated', saved)
    showEditor.value = false
    appStore.showSuccess(t('admin.accounts.upstreamProfiles.saved'))
  } catch (error) {
    console.error('Failed to save upstream provider profiles:', error)
    appStore.showError(t('admin.accounts.upstreamProfiles.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.profile-nav-row { display: flex; min-width: 0; }
.profile-expand-btn {
  display: inline-flex;
  width: 2rem;
  height: 2.5rem;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: rgb(156 163 175);
}
.profile-expand-btn:hover { color: rgb(75 85 99); }
.profile-platform-list { margin: 0.125rem 0 0.25rem 1.5rem; border-left: 1px solid rgb(229 231 235); padding-left: 0.375rem; }
.profile-platform-item {
  display: flex;
  min-height: 2.125rem;
  width: 100%;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.375rem;
  padding: 0.375rem 0.5rem;
  color: rgb(107 114 128);
  font-size: 0.75rem;
}
.profile-platform-item:hover { background: rgb(249 250 251); color: rgb(17 24 39); }
.profile-platform-item-active { background: rgb(240 253 250); color: rgb(4 120 87); font-weight: 600; }
.profile-nav-item {
  display: flex;
  min-height: 2.5rem;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.375rem;
  padding: 0.5rem 0.625rem;
  color: rgb(75 85 99);
  font-size: 0.8125rem;
  transition: background-color 180ms ease, color 180ms ease;
}
.profile-nav-item:hover { background: rgb(249 250 251); color: rgb(17 24 39); }
.profile-nav-item-active { background: rgb(236 253 245); color: rgb(4 120 87); font-weight: 600; }
.profile-count { flex-shrink: 0; color: rgb(156 163 175); font-size: 0.6875rem; font-variant-numeric: tabular-nums; }
.profile-disabled-dot { width: 0.375rem; height: 0.375rem; flex-shrink: 0; border-radius: 999px; background: rgb(156 163 175); }
@media (prefers-reduced-motion: reduce) { .profile-nav-item { transition: none; } }
@media (prefers-color-scheme: dark) {
  .profile-nav-item { color: rgb(209 213 219); }
  .profile-nav-item:hover { background: rgb(31 41 55); color: rgb(243 244 246); }
  .profile-nav-item-active { background: rgb(6 78 59 / 0.3); color: rgb(110 231 183); }
  .profile-expand-btn:hover { color: rgb(229 231 235); }
  .profile-platform-list { border-color: rgb(75 85 99); }
  .profile-platform-item { color: rgb(156 163 175); }
  .profile-platform-item:hover { background: rgb(31 41 55); color: rgb(243 244 246); }
  .profile-platform-item-active { background: rgb(6 78 59 / 0.3); color: rgb(110 231 183); }
}
</style>
