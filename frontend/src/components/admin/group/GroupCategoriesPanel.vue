<template>
  <aside class="group-category-nav flex min-h-0 w-full shrink-0 flex-col overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
    <header class="flex items-start justify-between gap-3 border-b border-gray-100 px-3 py-3 dark:border-dark-700">
      <div class="min-w-0">
        <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.groups.categories.title') }}</h2>
        <p class="mt-1 text-xs leading-4 text-gray-500 dark:text-gray-400">{{ t('admin.groups.categories.description') }}</p>
      </div>
      <button type="button" class="icon-btn text-gray-500" :disabled="saving || loadingEditor" :title="t('admin.groups.categories.manage')" @click="openEditor">
        <Icon name="cog" size="sm" />
      </button>
    </header>

    <div class="min-h-0 flex-1 overflow-y-auto p-2">
      <button type="button" class="category-nav-item" :class="activeCategoryId === 'all' && 'category-nav-item-active'" @click="emit('select', 'all')">
        <Icon name="grid" size="sm" class="shrink-0" />
        <span class="min-w-0 flex-1 truncate text-left">{{ t('admin.groups.categories.allGroups') }}</span>
        <span class="category-count">{{ groups.length }}</span>
      </button>
      <button type="button" class="category-nav-item mt-1" :class="activeCategoryId === 'uncategorized' && 'category-nav-item-active'" @click="emit('select', 'uncategorized')">
        <Icon name="inbox" size="sm" class="shrink-0 text-gray-400" />
        <span class="min-w-0 flex-1 truncate text-left">{{ t('admin.groups.categories.uncategorized') }}</span>
        <span class="category-count">{{ uncategorizedCount }}</span>
      </button>

      <VueDraggable v-model="orderedCategories" class="mt-1 space-y-1" handle=".category-drag-handle" :animation="180" :disabled="savingOrder" data-testid="group-category-order-list" @end="handleDragEnd">
        <div v-for="category in orderedCategories" :key="category.id" class="category-nav-row">
          <button type="button" class="category-nav-item min-w-0 flex-1" :class="activeCategoryId === category.id && 'category-nav-item-active'" @click="emit('select', category.id)">
            <Icon name="server" size="sm" class="category-drag-handle shrink-0 cursor-grab text-gray-400" />
            <span class="min-w-0 flex-1 truncate text-left">{{ category.name }}</span>
            <span class="category-count">{{ categoryCount(category.id) }}</span>
          </button>
        </div>
      </VueDraggable>
      <p v-if="loadingEditor" role="status" class="px-2 py-1 text-xs text-gray-500">{{ t('common.loading') }}</p>
    </div>
  </aside>

  <BaseDialog :show="showEditor" :title="t('admin.groups.categories.manage')" width="normal" @close="closeEditor">
    <div class="space-y-3">
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.categories.manageHint') }}</p>
      <div v-for="(category, index) in draftCategories" :key="category.id" class="rounded-lg border border-gray-200 p-2 dark:border-dark-600">
        <div class="flex items-center gap-2">
          <Icon name="gripVertical" size="sm" class="shrink-0 text-gray-400" />
          <input v-model="category.name" class="input min-w-0 flex-1" :placeholder="t('admin.groups.categories.namePlaceholder')" />
          <button type="button" class="rounded p-2 text-gray-400 hover:text-red-500" :title="t('common.delete')" @click="removeCategory(index)">
            <Icon name="trash" size="sm" />
          </button>
        </div>
        <div class="mt-2 max-h-36 space-y-1 overflow-y-auto border-t border-gray-100 pt-2 dark:border-dark-700">
          <label v-for="group in groups" :key="group.id" class="flex items-center gap-2 px-1 py-1 text-xs text-gray-600 dark:text-gray-300">
            <input type="checkbox" :checked="category.group_ids.includes(group.id)" :disabled="isGroupAssignedElsewhere(category.id, group.id)" @change="toggleGroup(category, group.id)" />
            <span class="min-w-0 truncate">{{ group.name }}</span>
          </label>
          <p v-if="groups.length === 0" class="px-1 text-xs text-gray-400">{{ t('admin.groups.categories.noGroups') }}</p>
        </div>
      </div>
      <button type="button" class="btn btn-secondary w-full" @click="addCategory">
        <Icon name="plus" size="sm" class="mr-1" />{{ t('admin.groups.categories.add') }}
      </button>
    </div>
    <template #footer>
      <button type="button" class="btn btn-secondary" :disabled="saving" @click="closeEditor">{{ t('common.cancel') }}</button>
      <button type="button" class="btn btn-primary" :disabled="saving" @click="saveCategories">{{ saving ? t('common.saving') : t('common.save') }}</button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import type { GroupCategory } from '@/api/admin/groups'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

export type GroupCategorySelection = 'all' | 'uncategorized' | number

const props = defineProps<{
  categories: GroupCategory[]
  groups: AdminGroup[]
  activeCategoryId: GroupCategorySelection
}>()
const emit = defineEmits<{
  select: [selection: GroupCategorySelection]
  updated: [categories: GroupCategory[]]
}>()
const { t } = useI18n()
const appStore = useAppStore()
const showEditor = ref(false)
const loadingEditor = ref(false)
const saving = ref(false)
const savingOrder = ref(false)
const editorSnapshot = ref<GroupCategory[]>([])
const draftCategories = ref<GroupCategory[]>([])
const orderedCategories = ref<GroupCategory[]>([])

watch(() => props.categories, (value) => {
  orderedCategories.value = [...value].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id).map(category => ({ ...category, group_ids: [...category.group_ids] }))
}, { immediate: true, deep: true })

const categoryByGroupID = computed(() => {
  const map = new Map<number, number>()
  for (const category of props.categories) for (const groupID of category.group_ids) map.set(groupID, category.id)
  return map
})
const uncategorizedCount = computed(() => props.groups.filter(group => !categoryByGroupID.value.has(group.id)).length)
const categoryCount = (id: number) => props.groups.filter(group => categoryByGroupID.value.get(group.id) === id).length

const openEditor = async () => {
  if (loadingEditor.value || saving.value) return
  loadingEditor.value = true
  try {
    const current = await adminAPI.groups.getCategories()
    editorSnapshot.value = current.map(category => ({ ...category, group_ids: [...category.group_ids] }))
    draftCategories.value = current.map(category => ({ ...category, group_ids: [...category.group_ids] }))
    emit('updated', current)
    showEditor.value = true
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.loadFailed')))
  } finally {
    loadingEditor.value = false
  }
}
const closeEditor = () => { if (!saving.value) showEditor.value = false }
const addCategory = () => {
  const nextID = Math.max(0, ...draftCategories.value.map(category => category.id), ...props.categories.map(category => category.id)) + 1
  draftCategories.value.push({ id: nextID, sort_order: draftCategories.value.length * 10, name: '', group_ids: [] })
}
const isGroupAssignedElsewhere = (categoryID: number, groupID: number) =>
  draftCategories.value.some(category => category.id !== categoryID && category.group_ids.includes(groupID))
const toggleGroup = (category: GroupCategory, groupID: number) => {
  const index = category.group_ids.indexOf(groupID)
  if (index >= 0) category.group_ids.splice(index, 1)
  else category.group_ids.push(groupID)
}
const removeCategory = (index: number) => {
  if (saving.value) return
  const category = draftCategories.value[index]
  if (category?.group_ids.length) {
    appStore.showError(t('admin.groups.categories.inUse'))
    return
  }
  draftCategories.value.splice(index, 1)
}
const saveCategories = async () => {
  if (saving.value) return
  if (draftCategories.value.some(category => !category.name.trim())) {
    appStore.showError(t('admin.groups.categories.nameRequired'))
    return
  }
  saving.value = true
  try {
    const payload = draftCategories.value.map((category, index) => ({ ...category, sort_order: index * 10, name: category.name.trim() }))
    const saved = await adminAPI.groups.updateCategories(payload, editorSnapshot.value)
    emit('updated', saved)
    showEditor.value = false
    appStore.showSuccess(t('admin.groups.categories.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.saveFailed')))
  } finally {
    saving.value = false
  }
}
const handleDragEnd = async () => {
  if (savingOrder.value || orderedCategories.value.length === 0) return
  const previous = props.categories.map(category => ({ ...category, group_ids: [...category.group_ids] }))
  savingOrder.value = true
  try {
    const payload = orderedCategories.value.map((category, index) => ({ ...category, sort_order: index * 10 }))
    const saved = await adminAPI.groups.updateCategories(payload, previous)
    emit('updated', saved)
    appStore.showSuccess(t('admin.groups.categories.orderSaved'))
  } catch (error) {
    orderedCategories.value = [...previous]
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.categories.saveFailed')))
  } finally {
    savingOrder.value = false
  }
}
</script>

<style scoped>
.category-nav-row { display: flex; min-width: 0; }
.category-nav-item { display: flex; min-width: 0; width: 100%; align-items: center; gap: .5rem; border-radius: .5rem; padding: .625rem .5rem; text-align: left; font-size: .8125rem; color: rgb(75 85 99); transition: background-color .15s, color .15s; }
.category-nav-item:hover { background: rgb(249 250 251); color: rgb(17 24 39); }
.category-nav-item-active { background: rgb(236 253 245); color: rgb(5 150 105); font-weight: 600; }
.category-count { flex-shrink: 0; min-width: 1.5rem; text-align: right; font-size: .6875rem; color: rgb(156 163 175); }
.dark .category-nav-item { color: rgb(209 213 219); }
.dark .category-nav-item:hover { background: rgb(55 65 81); color: rgb(255 255 255); }
.dark .category-nav-item-active { background: rgb(6 78 59 / .35); color: rgb(110 231 183); }
</style>
