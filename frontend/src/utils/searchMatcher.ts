const SEARCH_TERM_SPLIT_REGEX = /[\s,，、;；]+/u

const normalizeSearchValue = (value: string | number | null | undefined): string => {
  return String(value ?? '').trim().toLowerCase()
}

export const splitSearchTerms = (query: string): string[] => {
  if (!query.trim()) return []

  const seen = new Set<string>()
  const terms: string[] = []

  for (const term of query.split(SEARCH_TERM_SPLIT_REGEX)) {
    const normalized = normalizeSearchValue(term)
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    terms.push(normalized)
  }

  return terms
}

export const matchesSearchTerms = (
  query: string,
  values: Array<string | number | null | undefined>
): boolean => {
  const terms = splitSearchTerms(query)
  if (terms.length === 0) return true

  const haystacks = values
    .map((value) => normalizeSearchValue(value))
    .filter((value) => value.length > 0)

  if (haystacks.length === 0) return false

  return terms.every((term) => haystacks.some((value) => value.includes(term)))
}
