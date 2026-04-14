package websearch

const (
	maxResponseSize   = 1 << 20
	errorBodyTruncLen = 200
)

func truncateBody(body []byte) string {
	if len(body) <= errorBodyTruncLen {
		return string(body)
	}
	return string(body[:errorBodyTruncLen]) + "...(truncated)"
}
