import { describe, expect, it } from 'vitest'

import {
  normalizePlanFeatures,
  normalizePlanValidityUnit,
  normalizeSubscriptionPlan,
  sortSubscriptionPlans,
} from '@/utils/subscriptionPlan'

describe('subscriptionPlan utils', () => {
  it('normalizes legacy plural validity units to canonical singular values', () => {
    expect(normalizePlanValidityUnit('days')).toBe('day')
    expect(normalizePlanValidityUnit('weeks')).toBe('week')
    expect(normalizePlanValidityUnit('months')).toBe('month')
    expect(normalizePlanValidityUnit('years')).toBe('year')
  })

  it('normalizes mixed feature payloads into clean arrays', () => {
    expect(normalizePlanFeatures('A\n\n B ')).toEqual(['A', 'B'])
    expect(normalizePlanFeatures([' A ', '', 'B'])).toEqual(['A', 'B'])
  })

  it('normalizes plan payloads and sorts deterministically', () => {
    const older = normalizeSubscriptionPlan({
      id: 2,
      group_id: 1,
      name: 'Older',
      description: '',
      price: 19.9,
      validity_days: 1,
      validity_unit: 'days',
      features: 'A',
      for_sale: true,
      sort_order: 1,
    })

    const newer = normalizeSubscriptionPlan({
      id: 1,
      group_id: 1,
      name: 'Newer',
      description: '',
      price: 29.9,
      validity_days: 1,
      validity_unit: 'months',
      features: ['B'],
      for_sale: true,
      sort_order: 0,
    })

    expect(older.validity_unit).toBe('day')
    expect(newer.validity_unit).toBe('month')
    expect(sortSubscriptionPlans([older, newer]).map(plan => plan.id)).toEqual([1, 2])
  })
})
