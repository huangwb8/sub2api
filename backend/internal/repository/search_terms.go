package repository

import "strings"

var searchTermSeparatorReplacer = strings.NewReplacer(
	",", " ",
	"，", " ",
	"、", " ",
	";", " ",
	"；", " ",
	"\n", " ",
	"\r", " ",
	"\t", " ",
)

func splitSearchTerms(search string) []string {
	normalized := strings.TrimSpace(search)
	if normalized == "" {
		return nil
	}

	normalized = searchTermSeparatorReplacer.Replace(normalized)

	seen := make(map[string]struct{})
	terms := make([]string, 0)
	for _, raw := range strings.Fields(normalized) {
		term := strings.TrimSpace(raw)
		if term == "" {
			continue
		}
		key := strings.ToLower(term)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		terms = append(terms, term)
	}

	return terms
}
