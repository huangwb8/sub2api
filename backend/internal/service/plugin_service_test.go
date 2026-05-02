package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
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

func TestPluginService_ListAPIPromptTemplateOptions_RemoteSyncsAndCachesTemplates(t *testing.T) {
	t.Parallel()

	var sawAuth bool
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer secret" && r.Header.Get("x-api-key") == "secret" {
			sawAuth = true
		}
		require.Equal(t, "/v1/templates", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"templates": []map[string]any{
				{
					"id":          "remote-focus",
					"name":        "Remote Focus",
					"description": "Remote directory template",
					"enabled":     true,
					"sort_order":  5,
				},
			},
		})
	}))
	defer remote.Close()

	svc, err := NewPluginService(t.TempDir())
	require.NoError(t, err)
	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-remote",
		Type:    PluginTypeAPIPrompt,
		BaseURL: remote.URL,
		APIKey:  "secret",
		Enabled: true,
	})
	require.NoError(t, err)

	options, err := svc.ListAPIPromptTemplateOptions(context.Background())
	require.NoError(t, err)
	require.Len(t, options, 1)
	require.True(t, sawAuth)
	require.Equal(t, "remote-focus", options[0].TemplateID)
	require.Equal(t, "remote", options[0].Source)
	require.Equal(t, "available", options[0].Status)
	require.NotNil(t, options[0].LastSyncedAt)
	require.Empty(t, options[0].Prompt)

	plugin, err := svc.GetPlugin(context.Background(), "prompt-remote")
	require.NoError(t, err)
	require.Equal(t, "remote", plugin.APIPrompt.Source)
	require.Equal(t, 1, plugin.APIPrompt.RemoteTemplateCount)
	require.NotNil(t, plugin.APIPrompt.LastSyncedAt)
}

func TestPluginService_ValidateAPIKeyPluginSettings_RemoteRequiresFreshTemplates(t *testing.T) {
	t.Parallel()

	remoteOK := true
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !remoteOK {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"templates": []map[string]any{
				{"id": "remote-focus", "name": "Remote Focus", "enabled": true},
			},
		})
	}))
	defer remote.Close()

	svc, err := NewPluginService(t.TempDir())
	require.NoError(t, err)
	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-remote",
		Type:    PluginTypeAPIPrompt,
		BaseURL: remote.URL,
		Enabled: true,
	})
	require.NoError(t, err)

	binding := domain.APIKeyPluginSettings{
		APIPrompt: &domain.APIPromptKeyBinding{PluginName: "prompt-remote", TemplateID: "remote-focus"},
	}
	validated, err := svc.ValidateAPIKeyPluginSettings(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "remote-focus", validated.APIPrompt.TemplateID)

	remoteOK = false
	_, err = svc.ValidateAPIKeyPluginSettings(context.Background(), binding)
	require.ErrorIs(t, err, ErrInvalidPluginBinding)
}
