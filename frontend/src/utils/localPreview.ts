import type { UserAnnouncement } from '@/types'

const LOCAL_ANNOUNCEMENT_KEY = 'sub2api_local_preview_announcement'
const LOCAL_DASHBOARD_NOTICE_KEY = 'sub2api_local_preview_dashboard_notice'

/** True only for the loopback-only Vite preview session. */
export function isLocalPreviewSession(): boolean {
  if (typeof window === 'undefined') return false
  const hostname = window.location.hostname
  if (!['localhost', '127.0.0.1', '::1'].includes(hostname)) return false
  if (!(import.meta.env.DEV || import.meta.env.VITE_LOCAL_LOGIN_SHORTCUTS === 'true')) return false
  return localStorage.getItem('auth_token')?.startsWith('local-preview-') === true
}

export function getLocalPreviewAnnouncement(): UserAnnouncement | null {
  try {
    const raw = localStorage.getItem(LOCAL_ANNOUNCEMENT_KEY)
    if (!raw) return null
    const value = JSON.parse(raw) as UserAnnouncement
    if (!value || typeof value.title !== 'string' || typeof value.content !== 'string') return null
    return value
  } catch {
    return null
  }
}

export function setLocalPreviewAnnouncement(title: string, content: string): UserAnnouncement {
  const now = new Date().toISOString()
  const previous = getLocalPreviewAnnouncement()
  const announcement: UserAnnouncement = {
    id: previous?.id ?? 1,
    title,
    content,
    notify_mode: 'silent',
    created_at: previous?.created_at ?? now,
    updated_at: now,
  }
  localStorage.setItem(LOCAL_ANNOUNCEMENT_KEY, JSON.stringify(announcement))
  return announcement
}

export function getLocalDashboardNotice(): string {
  try {
    return localStorage.getItem(LOCAL_DASHBOARD_NOTICE_KEY) ?? ''
  } catch {
    return ''
  }
}

export function setLocalDashboardNotice(value: string): void {
  localStorage.setItem(LOCAL_DASHBOARD_NOTICE_KEY, value)
}
