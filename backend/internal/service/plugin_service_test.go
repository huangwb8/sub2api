package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginService_CreatePlugin_PersistsAPIPromptInstance(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	svc, err := NewPluginService(rootDir)
	require.NoError(t, err)

	plugin, err := svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:        "api-prompt",
		Type:        PluginTypeAPIPrompt,
		Description: "Prompt templates for focused API keys",
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Equal(t, "api-prompt", plugin.Name)
	require.Equal(t, PluginTypeAPIPrompt, plugin.Type)
	require.True(t, plugin.Enabled)
	require.NotEmpty(t, plugin.APIPrompt)
	require.NotEmpty(t, plugin.APIPrompt.Templates)

	require.DirExists(t, rootDir+"/api-prompt")
	require.FileExists(t, rootDir+"/api-prompt/manifest.json")
	require.FileExists(t, rootDir+"/api-prompt/config.json")
}

func TestPluginService_ListAPIPromptTemplateOptions_OnlyEnabledPlugins(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	svc, err := NewPluginService(rootDir)
	require.NoError(t, err)

	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-a",
		Type:    PluginTypeAPIPrompt,
		Enabled: true,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{
					ID:          "focus",
					Name:        "Focus",
					Description: "Keep answers concise",
					Prompt:      "Always answer concisely.",
					Enabled:     true,
					SortOrder:   20,
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-b",
		Type:    PluginTypeAPIPrompt,
		Enabled: false,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{
					ID:        "disabled-plugin-template",
					Name:      "Disabled Plugin Template",
					Prompt:    "This should never be exposed.",
					Enabled:   true,
					SortOrder: 10,
				},
			},
		},
	})
	require.NoError(t, err)

	options, err := svc.ListAPIPromptTemplateOptions(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, options)

	var found bool
	for _, option := range options {
		require.NotEqual(t, "prompt-b", option.PluginName)
		if option.PluginName == "prompt-a" && option.TemplateID == "focus" {
			found = true
			require.Equal(t, "Focus", option.Name)
			require.Equal(t, "Always answer concisely.", option.Prompt)
		}
	}
	require.True(t, found, "expected enabled prompt template to be listed")
}
