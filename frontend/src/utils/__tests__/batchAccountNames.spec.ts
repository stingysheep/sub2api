import { describe, expect, it } from 'vitest'
import { buildBatchAccountNames, parseMultilineValues } from '../batchAccountNames'

describe('batch account names', () => {
  it('parses one API key per non-empty line', () => {
    expect(parseMultilineValues(' first\n\nsecond\r\n third ')).toEqual(['first', 'second', 'third'])
  })

  it('adds numbered names and skips occupied numbers', () => {
    expect(buildBatchAccountNames('coco-gpt-007', 3, ['coco-gpt-007-1'])).toEqual([
      'coco-gpt-007-2',
      'coco-gpt-007-3',
      'coco-gpt-007-4'
    ])
  })
})
