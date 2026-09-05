import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve('src/components/common/DashboardAnnouncementBar.vue'), 'utf8')

describe('dashboard notice separation', () => {
  it('uses the dedicated dashboard_notice setting instead of the announcement store', () => {
    expect(source).toContain('dashboard_notice')
    expect(source).toContain('getLocalDashboardNotice')
    expect(source).not.toContain('useAnnouncementStore')
    expect(source).not.toContain('announcementsAPI')
  })

  it('uses dashboard-specific copy so the editor cannot be confused with announcements', () => {
    expect(source).toContain("admin.dashboard.notice.label")
    expect(source).toContain("admin.dashboard.notice.create")
    expect(source).toContain("admin.dashboard.notice.edit")
    expect(source).not.toContain("admin.announcements.createAnnouncement")
    expect(source).not.toContain("admin.announcements.editAnnouncement")
  })

  it('shows the dashboard notice label only to administrators', () => {
    expect(source).toContain('<span v-if="isAdmin" class="dashboard-announcement__title">')
    expect(source).toContain('<span v-if="isAdmin" class="dashboard-announcement__separator">')
  })
})
