export type NumericInputValue = number | string | null | undefined

export interface NormalizeNumberOptions {
  min?: number
  max?: number
  fallback: number
  integer?: boolean
}

export interface NormalizeOptionalNumberOptions {
  min?: number
  max?: number
  integer?: boolean
}

const toFiniteNumber = (value: NumericInputValue): number | null => {
  if (value === '' || value === null || value === undefined) {
    return null
  }

  const numericValue = Number(value)
  return Number.isFinite(numericValue) ? numericValue : null
}

const clampNumber = (
  value: number,
  options: Omit<NormalizeNumberOptions, 'fallback'>
): number => {
  let normalized = options.integer ? Math.floor(value) : value

  if (options.min !== undefined && normalized < options.min) {
    normalized = options.min
  }
  if (options.max !== undefined && normalized > options.max) {
    normalized = options.max
  }

  return normalized
}

export const normalizeNumberInput = (
  value: NumericInputValue,
  options: NormalizeNumberOptions
): number => {
  const numericValue = toFiniteNumber(value)

  if (numericValue === null) {
    return clampNumber(options.fallback, options)
  }

  return clampNumber(numericValue, options)
}

export const normalizeOptionalNumberInput = (
  value: NumericInputValue,
  options: NormalizeOptionalNumberOptions = {}
): number | null => {
  const numericValue = toFiniteNumber(value)

  if (numericValue === null) {
    return null
  }

  if (options.min !== undefined && numericValue < options.min) {
    return null
  }
  if (options.max !== undefined && numericValue > options.max) {
    return null
  }

  return options.integer ? Math.floor(numericValue) : numericValue
}

export const optionalNumberInputValue = (
  value: NumericInputValue,
  options: { blankZero?: boolean } = {}
): string | number => {
  if (value === '' || value === null || value === undefined) {
    return ''
  }

  if (options.blankZero && Number(value) === 0) {
    return ''
  }

  return value
}
