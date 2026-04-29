package apicompat

// ImageGenerationsRequest represents an OpenAI Images generation request.
type ImageGenerationsRequest struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	N                 int    `json:"n,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	Background        string `json:"background,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression int    `json:"output_compression,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	Stream            bool   `json:"stream,omitempty"`
	PartialImages     int    `json:"partial_images,omitempty"`
	ResponseFormat    string `json:"response_format,omitempty"`
	Style             string `json:"style,omitempty"`
	User              string `json:"user,omitempty"`
	PromptCacheKey    string `json:"prompt_cache_key,omitempty"`
}

// ImageData represents one image item in an OpenAI Images response.
type ImageData struct {
	B64JSON       string `json:"b64_json,omitempty"`
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// ImageUsage mirrors the token usage object returned by OpenAI Images APIs.
type ImageUsage struct {
	TotalTokens         int                     `json:"total_tokens,omitempty"`
	InputTokens         int                     `json:"input_tokens,omitempty"`
	OutputTokens        int                     `json:"output_tokens,omitempty"`
	InputTokensDetails  *ImageUsageTokenDetails `json:"input_tokens_details,omitempty"`
	OutputTokensDetails *ImageUsageTokenDetails `json:"output_tokens_details,omitempty"`
}

// ImageUsageTokenDetails contains text/image token details for image usage.
type ImageUsageTokenDetails struct {
	TextTokens  int `json:"text_tokens,omitempty"`
	ImageTokens int `json:"image_tokens,omitempty"`
}

// ImageGenerationsResponse represents an OpenAI Images generation response.
type ImageGenerationsResponse struct {
	Created      int64       `json:"created"`
	Data         []ImageData `json:"data"`
	Usage        *ImageUsage `json:"usage,omitempty"`
	OutputFormat string      `json:"output_format,omitempty"`
	Quality      string      `json:"quality,omitempty"`
	Size         string      `json:"size,omitempty"`
	Background   string      `json:"background,omitempty"`
}
