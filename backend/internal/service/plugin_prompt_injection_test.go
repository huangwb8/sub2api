package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newPromptTestService(t *testing.T) *PluginService {
	t.Helper()

	svc, err := NewPluginService(t.TempDir())
	require.NoError(t, err)
	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-a",
		Type:    PluginTypeAPIPrompt,
		Enabled: true,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{
					ID:        "focus",
					Name:      "Focus",
					Prompt:    "Always answer like a senior backend engineer.",
					Enabled:   true,
					SortOrder: 10,
				},
			},
		},
	})
	require.NoError(t, err)
	return svc
}

func testPromptBinding() domain.APIKeyPluginSettings {
	return domain.APIKeyPluginSettings{
		APIPrompt: &domain.APIPromptKeyBinding{
			PluginName: "prompt-a",
			TemplateID: "focus",
		},
	}
}

func TestPluginService_ApplyBoundPromptTemplate_AnthropicMessagesPrependsSystemBlock(t *testing.T) {
	t.Parallel()

	svc := newPromptTestService(t)
	body := []byte(`{"model":"claude-sonnet-4-5","messages":[{"role":"user","content":"hello"}],"system":"Existing system"}`)

	updated, resolved, err := svc.ApplyBoundPromptTemplate(context.Background(), body, PluginPromptTargetAnthropicMessages, testPromptBinding())
	require.NoError(t, err)
	require.Equal(t, "prompt-a", resolved.PluginName)
	require.Equal(t, "focus", resolved.TemplateID)
	require.Equal(t, "Always answer like a senior backend engineer.", gjson.GetBytes(updated, "system.0.text").String())
	require.Equal(t, "Existing system", gjson.GetBytes(updated, "system.1.text").String())
}

func TestPluginService_ApplyBoundPromptTemplate_OpenAIChatCompletionsPrependsSystemMessage(t *testing.T) {
	t.Parallel()

	svc := newPromptTestService(t)
	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`)

	updated, _, err := svc.ApplyBoundPromptTemplate(context.Background(), body, PluginPromptTargetOpenAIChatCompletions, testPromptBinding())
	require.NoError(t, err)
	require.Equal(t, "system", gjson.GetBytes(updated, "messages.0.role").String())
	require.Equal(t, "Always answer like a senior backend engineer.", gjson.GetBytes(updated, "messages.0.content").String())
	require.Equal(t, "user", gjson.GetBytes(updated, "messages.1.role").String())
}

func TestPluginService_ApplyBoundPromptTemplate_OpenAIResponsesPrependsInstructions(t *testing.T) {
	t.Parallel()

	svc := newPromptTestService(t)
	body := []byte(`{"model":"gpt-5","instructions":"Existing instructions","input":"hello"}`)

	updated, _, err := svc.ApplyBoundPromptTemplate(context.Background(), body, PluginPromptTargetOpenAIResponses, testPromptBinding())
	require.NoError(t, err)
	require.Equal(t, "Always answer like a senior backend engineer.\n\nExisting instructions", gjson.GetBytes(updated, "instructions").String())
}

func TestPluginService_ApplyBoundPromptTemplate_GeminiPrependsSystemInstruction(t *testing.T) {
	t.Parallel()

	svc := newPromptTestService(t)
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}],"systemInstruction":{"parts":[{"text":"Existing system"}]}}`)

	updated, _, err := svc.ApplyBoundPromptTemplate(context.Background(), body, PluginPromptTargetGeminiGenerateContent, testPromptBinding())
	require.NoError(t, err)
	require.Equal(t, "Always answer like a senior backend engineer.", gjson.GetBytes(updated, "systemInstruction.parts.0.text").String())
	require.Equal(t, "Existing system", gjson.GetBytes(updated, "systemInstruction.parts.1.text").String())
}

func TestPluginService_ApplyBoundPromptTemplate_DisabledPluginReturnsOriginalBody(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	svc, err := NewPluginService(rootDir)
	require.NoError(t, err)
	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-a",
		Type:    PluginTypeAPIPrompt,
		Enabled: false,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{ID: "focus", Name: "Focus", Prompt: "Always answer concisely.", Enabled: true},
			},
		},
	})
	require.NoError(t, err)

	body := []byte(`{"model":"gpt-5","messages":[{"role":"user","content":"hello"}]}`)
	updated, resolved, err := svc.ApplyBoundPromptTemplate(context.Background(), body, PluginPromptTargetOpenAIChatCompletions, testPromptBinding())
	require.NoError(t, err)
	require.Nil(t, resolved)
	require.JSONEq(t, string(body), string(updated))
}
