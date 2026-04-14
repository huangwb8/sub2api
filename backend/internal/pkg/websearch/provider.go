package websearch

import "context"

type Provider interface {
	Name() string
	Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
}
