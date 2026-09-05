<template>
  <div
    v-if="notice || isAdmin"
    class="dashboard-announcement"
    role="status"
    aria-live="polite"
  >
    <template v-if="notice">
      <button
        type="button"
        class="dashboard-announcement__content"
        :class="{ 'dashboard-announcement__content--editable': isAdmin }"
        :aria-label="isAdmin ? t('admin.dashboard.notice.edit') : undefined"
        @click="isAdmin && openEditor()"
      >
        <span v-if="isAdmin" class="dashboard-announcement__title">{{ t('admin.dashboard.notice.label') }}</span>
        <span v-if="isAdmin" class="dashboard-announcement__separator">:</span>
        <span class="dashboard-announcement__message">{{ notice }}</span>
      </button>

      <button
        v-if="isAdmin"
        type="button"
        class="dashboard-announcement__edit"
        :aria-label="t('admin.dashboard.notice.edit')"
        :title="t('admin.dashboard.notice.edit')"
        @click="openEditor"
      >
        <Icon name="edit" size="xs" />
      </button>
    </template>

    <button
      v-else
      type="button"
      class="dashboard-announcement__empty-link"
      @click="openEditor"
    >
      {{ t('admin.dashboard.notice.create') }}
    </button>

    <BaseDialog
      :show="editorOpen"
      :title="notice ? t('admin.dashboard.notice.edit') : t('admin.dashboard.notice.create')"
      width="normal"
      @close="closeEditor"
    >
      <form id="dashboard-announcement-form" class="space-y-4" @submit.prevent="saveEditor">
        <div>
          <label class="input-label" for="dashboard-announcement-content">
            {{ t('admin.dashboard.notice.contentLabel') }}
          </label>
          <textarea
            id="dashboard-announcement-content"
            v-model="contentDraft"
            class="input min-h-32 resize-y"
            maxlength="10000"
            required
          ></textarea>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditor">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="dashboard-announcement-form"
            class="btn btn-primary"
            :disabled="saving"
          >
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { isLocalPreviewSession, getLocalDashboardNotice, setLocalDashboardNotice } from '@/utils/localPreview'

const props = defineProps<{
  isAdmin: boolean
}>()

const { t } = useI18n()
const appStore = useAppStore()
const localNotice = ref(getLocalDashboardNotice())
const notice = computed(() => {
  if (isLocalPreviewSession()) return localNotice.value
  return appStore.cachedPublicSettings?.dashboard_notice?.trim() ?? ''
})

const editorOpen = ref(false)
const saving = ref(false)
const contentDraft = ref('')

function openEditor() {
  if (!props.isAdmin) return
  contentDraft.value = notice.value
  editorOpen.value = true
}

function closeEditor() {
  if (saving.value) return
  editorOpen.value = false
}

async function saveEditor() {
  if (!props.isAdmin || !contentDraft.value.trim()) return

  saving.value = true
  try {
    if (isLocalPreviewSession()) {
      setLocalDashboardNotice(contentDraft.value.trim())
      localNotice.value = contentDraft.value.trim()
    } else {
      await adminAPI.settings.updateSettings({ dashboard_notice: contentDraft.value.trim() })
      await appStore.fetchPublicSettings(true)
    }
    editorOpen.value = false
    appStore.showSuccess(t('common.saved'))
  } catch (error: any) {
    appStore.showError(error?.message || t('common.unknownError'))
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.dashboard-announcement {
  display: flex;
  min-width: 0;
  flex: 1 1 auto;
  align-items: center;
  justify-content: center;
  gap: 0.45rem;
  overflow: hidden;
  padding: 0 1rem;
  color: rgb(71 85 105);
  font-size: 0.75rem;
  line-height: 1.25rem;
}

.dashboard-announcement__content {
  display: flex;
  min-width: 0;
  overflow: hidden;
  align-items: baseline;
  gap: 0.3rem;
  border: 0;
  padding: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
}

.dashboard-announcement__content--editable {
  cursor: pointer;
}

.dashboard-announcement__title {
  flex: 0 0 auto;
  max-width: 11rem;
  overflow: hidden;
  color: rgb(30 64 175);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-announcement__separator {
  flex: 0 0 auto;
  color: rgb(148 163 184);
}

.dashboard-announcement__message {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-announcement__edit {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  padding: 0.25rem;
  color: rgb(100 116 139);
  border-radius: 0.375rem;
  transition: color 150ms ease, background-color 150ms ease;
}

.dashboard-announcement__edit:hover {
  color: rgb(30 64 175);
  background: rgb(239 246 255);
}

.dashboard-announcement__empty-link {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgb(37 99 235);
  font-weight: 600;
}

@media (max-width: 1023px) {
  .dashboard-announcement {
    justify-content: flex-start;
    padding: 0 0.25rem;
  }
}

@media (max-width: 639px) {
  .dashboard-announcement__title,
  .dashboard-announcement__separator {
    display: none;
  }
}

:global(.dark) .dashboard-announcement {
  color: rgb(148 163 184);
}

:global(.dark) .dashboard-announcement__title {
  color: rgb(147 197 253);
}

:global(.dark) .dashboard-announcement__edit:hover {
  color: rgb(191 219 254);
  background: rgb(30 58 138 / 0.25);
}
</style>
