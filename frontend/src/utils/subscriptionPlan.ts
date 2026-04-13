import type { SubscriptionPlan } from '@/types/payment'

export type PlanValidityUnit = 'day' | 'week' | 'month' | 'year'

const VALIDITY_UNIT_ALIASES: Record<string, PlanValidityUnit> = {
  day: 'day',
  days: 'day',
  week: 'week',
  weeks: 'week',
  month: 'month',
  months: 'month',
  year: 'year',
  years: 'year',
}

export function normalizePlanValidityUnit(unit?: string | null): PlanValidityUnit {
  if (!unit) return 'day'
  return VALIDITY_UNIT_ALIASES[unit.trim().toLowerCase()] || 'day'
}

export function normalizePlanFeatures(features?: string[] | string | null): string[] {
  if (Array.isArray(features)) {
    return features.map(feature => feature.trim()).filter(Boolean)
  }
  if (typeof features === 'string') {
    return features.split('\n').map(feature => feature.trim()).filter(Boolean)
  }
  return []
}

export function normalizeSubscriptionPlan(
  plan: Omit<SubscriptionPlan, 'features'> & { features?: string[] | string | null }
): SubscriptionPlan {
  return {
    ...plan,
    validity_unit: normalizePlanValidityUnit(plan.validity_unit),
    features: normalizePlanFeatures(plan.features),
  }
}

export function sortSubscriptionPlans(plans: SubscriptionPlan[]): SubscriptionPlan[] {
  return [...plans].sort((left, right) => {
    if (left.sort_order !== right.sort_order) {
      return left.sort_order - right.sort_order
    }
    return left.id - right.id
  })
}
