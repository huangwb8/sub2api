import { describe, expect, it } from 'vitest'

import { matchesSearchTerms, splitSearchTerms } from '../searchMatcher'

describe('searchMatcher', () => {
  it('splits search terms by spaces and punctuation and deduplicates them', () => {
    expect(splitSearchTerms(' ab, dy；ab  ')).toEqual(['ab', 'dy'])
  })

  it('matches multiple keywords across the same field', () => {
    expect(matchesSearchTerms('ab dy', ['abdkdkdidddy'])).toBe(true)
  })

  it('matches multiple keywords across different fields', () => {
    expect(matchesSearchTerms('jp 127', ['Decodo-JP', '127.0.0.1'])).toBe(true)
  })

  it('returns false when any keyword is missing', () => {
    expect(matchesSearchTerms('ab zz', ['abdkdkdidddy'])).toBe(false)
  })
})
