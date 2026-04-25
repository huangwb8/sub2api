import { describe, expect, it } from 'vitest'
import {
  normalizeNumberInput,
  normalizeOptionalNumberInput,
  optionalNumberInputValue
} from '../numericInput'

describe('numericInput', () => {
  it('keeps empty edits representable until a required field is normalized', () => {
    expect(normalizeNumberInput('', { min: 1, fallback: 1, integer: true })).toBe(1)
    expect(normalizeNumberInput('18', { min: 1, fallback: 1, integer: true })).toBe(18)
  })

  it('returns null for optional empty or below-min values', () => {
    expect(normalizeOptionalNumberInput('', { min: 1 })).toBeNull()
    expect(normalizeOptionalNumberInput('0', { min: 1 })).toBeNull()
    expect(normalizeOptionalNumberInput('1.5', { min: 1 })).toBe(1.5)
  })

  it('renders optional zero as blank when zero means unlimited', () => {
    expect(optionalNumberInputValue(0, { blankZero: true })).toBe('')
    expect(optionalNumberInputValue(50, { blankZero: true })).toBe(50)
  })
})
