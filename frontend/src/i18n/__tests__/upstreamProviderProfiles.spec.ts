import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import en from '../locales/en'
import zh from '../locales/zh'

describe('upstream provider profile translations', () => {
  const files = [
    'components/admin/account/UpstreamProviderProfilesPanel.vue',
    'components/account/CreateAccountModal.vue',
    'components/account/EditAccountModal.vue',
    'views/admin/AccountsView.vue',
  ]
  const keys = new Set(files.flatMap(file => [...readFileSync(resolve(__dirname, '../../', file), 'utf8')
    .matchAll(/admin\.accounts\.upstreamProfiles\.([A-Za-z]+)/g)].map(match => match[1])))
  for (const [locale, messages] of Object.entries({ en, zh })) {
    it(`${locale} resolves every profile label from the assembled runtime messages`, () => {
      const profile = (messages.admin.accounts as unknown as Record<string, Record<string, string>>).upstreamProfiles
      expect(profile).toBeDefined()
      for (const key of keys) expect(profile[key], `${locale}.${key}`).toBeTruthy()
    })
  }
})
