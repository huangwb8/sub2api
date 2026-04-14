package websearch

type SearchResult struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	PageAge string `json:"page_age,omitempty"`
}

type SearchRequest struct {
	Query      string
	MaxResults int
	ProxyURL   string
}

type SearchResponse struct {
	Results []SearchResult
	Query   string
}

const defaultMaxResults = 5

const (
	ProviderTypeBrave  = "brave"
	ProviderTypeTavily = "tavily"
)
