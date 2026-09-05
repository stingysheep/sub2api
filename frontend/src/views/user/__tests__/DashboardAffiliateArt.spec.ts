import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(resolve('src/views/user/DashboardView.vue'), 'utf8')

describe('user dashboard affiliate artwork', () => {
  it('shows the animated artwork at tablet-width dashboard viewports', () => {
    expect(source).toContain('md:grid-cols-[1.5fr_1fr]')
    expect(source).toContain('hidden justify-center md:flex')
    expect(source).not.toContain('hidden justify-center lg:flex')
    expect(source.indexOf('v-if="affiliateDetail"')).toBeLessThan(source.indexOf('v-if="loading"'))
  })

  it('keeps the artwork motion visible without shortening its smooth cycles', () => {
    const artSource = readFileSync(resolve('src/components/affiliate/AffiliateNetworkArt.vue'), 'utf8')
    expect(artSource).toContain('translate3d(0,-6px,0)')
    expect(artSource).toContain('scale(1.04)')
    expect(artSource).toContain('animation:float 7.2s')
    expect(artSource).toContain('animation:pulse 6.6s')
  })
})
