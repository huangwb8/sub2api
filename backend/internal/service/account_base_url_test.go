//go:build unit

package service

import (
	"testing"
)

func TestGetBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "non-apikey type returns empty",
			account: Account{
				Type:     AccountTypeOAuth,
				Platform: PlatformAnthropic,
			},
			expected: "",
		},
		{
			name: "apikey without base_url returns default anthropic",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAnthropic,
				Credentials: map[string]any{},
			},
			expected: "https://api.anthropic.com",
		},
		{
			name: "apikey with custom base_url",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAnthropic,
				Credentials: map[string]any{"base_url": "https://custom.example.com"},
			},
			expected: "https://custom.example.com",
		},
		{
			name: "antigravity apikey auto-appends /antigravity",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity apikey trims trailing slash before appending",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com/"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity non-apikey returns empty",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.account.GetBaseURL()
			if result != tt.expected {
				t.Errorf("GetBaseURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetGeminiBaseURL(t *testing.T) {
	const defaultGeminiURL = "https://generativelanguage.googleapis.com"

	tests := []struct {
		name     string
		account  Account
		expected string
	}{
		{
			name: "apikey without base_url returns default",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformGemini,
				Credentials: map[string]any{},
			},
			expected: defaultGeminiURL,
		},
		{
			name: "apikey with custom base_url",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformGemini,
				Credentials: map[string]any{"base_url": "https://custom-gemini.example.com"},
			},
			expected: "https://custom-gemini.example.com",
		},
		{
			name: "antigravity apikey auto-appends /antigravity",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity apikey trims trailing slash",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com/"},
			},
			expected: "https://upstream.example.com/antigravity",
		},
		{
			name: "antigravity oauth does NOT append /antigravity",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"base_url": "https://upstream.example.com"},
			},
			expected: "https://upstream.example.com",
		},
		{
			name: "oauth without base_url returns default",
			account: Account{
				Type:        AccountTypeOAuth,
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{},
			},
			expected: defaultGeminiURL,
		},
		{
			name: "nil credentials returns default",
			account: Account{
				Type:     AccountTypeAPIKey,
				Platform: PlatformGemini,
			},
			expected: defaultGeminiURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.account.GetGeminiBaseURL(defaultGeminiURL)
			if result != tt.expected {
				t.Errorf("GetGeminiBaseURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestOpenAIChatAPIHelpers(t *testing.T) {
	tests := []struct {
		name        string
		account     Account
		wantBaseURL string
		wantAPIKey  string
		wantChatAPI bool
		wantFormat  OpenAIAPIFormat
	}{
		{
			name: "chatapi uses default openai base url",
			account: Account{
				Type:        AccountTypeChatAPI,
				Platform:    PlatformOpenAI,
				Credentials: map[string]any{"api_key": "sk-chat"},
			},
			wantBaseURL: "https://api.openai.com",
			wantAPIKey:  "sk-chat",
			wantChatAPI: true,
			wantFormat:  OpenAIAPIFormatChat,
		},
		{
			name: "chatapi keeps custom base url",
			account: Account{
				Type:        AccountTypeChatAPI,
				Platform:    PlatformOpenAI,
				Credentials: map[string]any{"api_key": "sk-chat", "base_url": "https://chat.example.com/v1"},
			},
			wantBaseURL: "https://chat.example.com/v1",
			wantAPIKey:  "sk-chat",
			wantChatAPI: true,
			wantFormat:  OpenAIAPIFormatChat,
		},
		{
			name: "responses api key stays responses format",
			account: Account{
				Type:        AccountTypeAPIKey,
				Platform:    PlatformOpenAI,
				Credentials: map[string]any{"api_key": "sk-responses", "base_url": "https://api.openai.com"},
			},
			wantBaseURL: "https://api.openai.com",
			wantAPIKey:  "sk-responses",
			wantChatAPI: false,
			wantFormat:  OpenAIAPIFormatResponses,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.GetOpenAIBaseURL(); got != tt.wantBaseURL {
				t.Fatalf("GetOpenAIBaseURL() = %q, want %q", got, tt.wantBaseURL)
			}
			if got := tt.account.GetOpenAIApiKey(); got != tt.wantAPIKey {
				t.Fatalf("GetOpenAIApiKey() = %q, want %q", got, tt.wantAPIKey)
			}
			if got := tt.account.IsOpenAIChatAPI(); got != tt.wantChatAPI {
				t.Fatalf("IsOpenAIChatAPI() = %v, want %v", got, tt.wantChatAPI)
			}
			if got := tt.account.OpenAIAPIFormat(); got != tt.wantFormat {
				t.Fatalf("OpenAIAPIFormat() = %q, want %q", got, tt.wantFormat)
			}
		})
	}
}
