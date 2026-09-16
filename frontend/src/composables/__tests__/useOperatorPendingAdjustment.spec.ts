import { describe, expect, it, beforeEach } from 'vitest'
import { useOperatorPendingAdjustment } from '../useOperatorPendingAdjustment'

describe('useOperatorPendingAdjustment', () => {
  beforeEach(() => localStorage.clear())

  it('binds the record to the current actor and role', () => {
    const storage = useOperatorPendingAdjustment()
    storage.save({ actorId: 7, role: 'operator', targetId: 9, idempotencyKey: 'key-1', createdAt: '2026-01-01T00:00:00.000Z' })
    expect(storage.read(7, 'operator')?.targetId).toBe(9)
    expect(storage.read(8, 'operator')).toBeNull()
    expect(storage.read(7, 'admin')).toBeNull()
  })

  it('fails closed when a same-actor record is malformed', () => {
    localStorage.setItem('operator.pending-adjustment.v1.operator.7', '{')
    expect(() => useOperatorPendingAdjustment().read(7, 'operator')).toThrow('unavailable')
  })
})
