export interface OperatorPendingAdjustmentRecord {
  actorId: number
  role: 'admin' | 'operator'
  targetId: number
  idempotencyKey: string
  createdAt: string
}

const STORAGE_PREFIX = 'operator.pending-adjustment.v1'

function storageKey(
  actorId: number,
  role: OperatorPendingAdjustmentRecord['role'],
) {
  return `${STORAGE_PREFIX}.${role}.${actorId}`
}

function isRecord(
  value: unknown,
  actorId: number,
  role: OperatorPendingAdjustmentRecord['role'],
): value is OperatorPendingAdjustmentRecord {
  if (!value || typeof value !== 'object') return false
  const record = value as Partial<OperatorPendingAdjustmentRecord>
  return (
    record.actorId === actorId &&
    record.role === role &&
    Number.isInteger(record.targetId) &&
    typeof record.idempotencyKey === 'string' &&
    record.idempotencyKey.length > 0 &&
    typeof record.createdAt === 'string'
  )
}

/**
 * Persists only a retry credential and identity binding. The adjustment body, especially
 * its free-form reason, is intentionally never written to browser storage.
 */
export function useOperatorPendingAdjustment() {
  function read(
    actorId: number,
    role: OperatorPendingAdjustmentRecord['role'],
  ): OperatorPendingAdjustmentRecord | null {
    try {
      const raw = window.localStorage.getItem(storageKey(actorId, role))
      if (!raw) return null
      const parsed: unknown = JSON.parse(raw)
      if (!isRecord(parsed, actorId, role))
        throw new Error('invalid pending adjustment record')
      return parsed
    } catch {
      // A malformed or inaccessible record must block another adjustment until the
      // operator checks the ledger; silently treating it as absent risks a duplicate.
      throw new Error('operator pending adjustment storage is unavailable')
    }
  }

  function save(record: OperatorPendingAdjustmentRecord): void {
    try {
      window.localStorage.setItem(
        storageKey(record.actorId, record.role),
        JSON.stringify(record),
      )
    } catch {
      throw new Error('operator pending adjustment storage is unavailable')
    }
  }

  function clear(
    actorId: number,
    role: OperatorPendingAdjustmentRecord['role'],
  ): void {
    try {
      window.localStorage.removeItem(storageKey(actorId, role))
    } catch {
      throw new Error('operator pending adjustment storage is unavailable')
    }
  }

  return { read, save, clear }
}
