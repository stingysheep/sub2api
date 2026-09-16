import { describe, expect, it } from 'vitest'
import { serializePublicSettings } from '../publicSettingsSerialization'

describe('public settings HTML script serialization', () => {
  it('preserves data while preventing a closing script and attacker markup', () => {
    const value = { site_name: '</script><img src=x onerror="alert(1)">', text: '中文 & > < \u2028 \u2029' }
    const json = serializePublicSettings(value)
    expect(json).not.toMatch(/[<>&\u2028\u2029]/)
    expect(JSON.parse(json)).toEqual(value)
    const host = document.createElement('div')
    host.innerHTML = `<script>window.__APP_CONFIG__=${json};</script>`
    expect(host.querySelectorAll('script')).toHaveLength(1)
    expect(host.querySelector('img')).toBeNull()
  })
  it('keeps normal JSON and handles absent values explicitly', () => {
    expect(JSON.parse(serializePublicSettings({ enabled: true, count: 0 }))).toEqual({ enabled: true, count: 0 })
    expect(serializePublicSettings(undefined)).toBe('null')
  })
})
