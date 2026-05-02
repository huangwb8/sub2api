package service

import (
	"context"
	"os"
	"path/filepath"
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
		Name:        "api-prompt-custom",
		Type:        PluginTypeAPIPrompt,
		Description: "Prompt templates for focused API keys",
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Equal(t, "api-prompt-custom", plugin.Name)
	require.Equal(t, PluginTypeAPIPrompt, plugin.Type)
	require.True(t, plugin.Enabled)
	require.NotEmpty(t, plugin.APIPrompt)
	require.NotEmpty(t, plugin.APIPrompt.Templates)

	require.DirExists(t, rootDir+"/api-prompt-custom")
	require.FileExists(t, rootDir+"/api-prompt-custom/manifest.json")
	require.FileExists(t, rootDir+"/api-prompt-custom/config.json")

	manifestData, err := os.ReadFile(filepath.Join(rootDir, "api-prompt-custom", "manifest.json"))
	require.NoError(t, err)
	require.NotContains(t, string(manifestData), "base_url")
	require.NotContains(t, string(manifestData), "api_key")
}

func TestPluginService_NewPluginService_BootstrapsDefaultPluginWhenRootDirEmpty(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	svc, err := NewPluginService(rootDir)
	require.NoError(t, err)

	plugins, err := svc.ListPlugins(context.Background())
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.Equal(t, "api-prompt", plugins[0].Name)
	require.Equal(t, PluginTypeAPIPrompt, plugins[0].Type)
	require.True(t, plugins[0].Enabled)
	require.NotNil(t, plugins[0].APIPrompt)
	require.NotEmpty(t, plugins[0].APIPrompt.Templates)

	require.FileExists(t, filepath.Join(rootDir, "api-prompt", "manifest.json"))
	require.FileExists(t, filepath.Join(rootDir, "api-prompt", "config.json"))
}

func TestPluginService_NewPluginService_BootstrapsDefaultPluginInDataDirOutsideRepo(t *testing.T) {
	startDir := t.TempDir()
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)
	t.Chdir(startDir)

	svc, err := NewPluginService("")
	require.NoError(t, err)

	plugins, err := svc.ListPlugins(context.Background())
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	require.Equal(t, "api-prompt", plugins[0].Name)
	require.FileExists(t, filepath.Join(dataDir, "plugins", "api-prompt", "manifest.json"))
	require.FileExists(t, filepath.Join(dataDir, "plugins", "api-prompt", "config.json"))
	require.NoDirExists(t, filepath.Join(startDir, "plugins"))
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

func TestPluginService_TestPlugin_ValidatesLocalEnabledTemplates(t *testing.T) {
	t.Parallel()

	svc, err := NewPluginService(t.TempDir())
	require.NoError(t, err)
	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-local",
		Type:    PluginTypeAPIPrompt,
		Enabled: true,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{ID: "local-focus", Name: "Local Focus", Prompt: "Use local config.", Enabled: true},
			},
		},
	})
	require.NoError(t, err)

	result, err := svc.TestPlugin(context.Background(), "prompt-local")
	require.NoError(t, err)
	require.True(t, result.OK)
	require.Contains(t, result.Message, "local configuration")
}

func TestPluginService_LegacyRemoteManifestFieldsIgnoredAndCleanedOnSave(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	pluginDir := filepath.Join(rootDir, "api-prompt")
	require.NoError(t, os.MkdirAll(pluginDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "manifest.json"), []byte(`{
  "name": "api-prompt",
  "type": "api-prompt",
  "description": "legacy remote fields",
  "base_url": "https://plugin.example.com",
  "api_key": "secret",
  "enabled": true,
  "created_at": "2026-05-02T05:20:07Z",
  "updated_at": "2026-05-02T05:20:07Z"
}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "config.json"), []byte(`{
  "templates": [
    {"id": "focus", "name": "Focus", "prompt": "Use local prompt.", "enabled": true}
  ],
  "source": "remote",
  "last_synced_at": "2026-05-02T05:20:07Z",
  "last_sync_error": "legacy",
  "remote_template_count": 1
}`), 0o644))

	svc, err := NewPluginService(rootDir)
	require.NoError(t, err)
	plugin, err := svc.GetPlugin(context.Background(), "api-prompt")
	require.NoError(t, err)
	require.Equal(t, "api-prompt", plugin.Name)
	require.Equal(t, "local", plugin.APIPrompt.Source)

	_, err = svc.UpdatePlugin(context.Background(), "api-prompt", UpdatePluginRequest{Description: ptrStringPlugin("local only")})
	require.NoError(t, err)
	manifestData, err := os.ReadFile(filepath.Join(pluginDir, "manifest.json"))
	require.NoError(t, err)
	require.NotContains(t, string(manifestData), "base_url")
	require.NotContains(t, string(manifestData), "api_key")
}

func TestPluginService_ValidateAPIKeyPluginSettings_LocalTemplateRequired(t *testing.T) {
	t.Parallel()

	svc, err := NewPluginService(t.TempDir())
	require.NoError(t, err)
	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "prompt-local",
		Type:    PluginTypeAPIPrompt,
		Enabled: true,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{ID: "local-focus", Name: "Local Focus", Prompt: "Use local prompt.", Enabled: true},
			},
		},
	})
	require.NoError(t, err)

	binding := domain.APIKeyPluginSettings{
		APIPrompt: &domain.APIPromptKeyBinding{PluginName: "prompt-local", TemplateID: "local-focus"},
	}
	validated, err := svc.ValidateAPIKeyPluginSettings(context.Background(), binding)
	require.NoError(t, err)
	require.Equal(t, "local-focus", validated.APIPrompt.TemplateID)

	binding.APIPrompt.TemplateID = "missing"
	_, err = svc.ValidateAPIKeyPluginSettings(context.Background(), binding)
	require.ErrorIs(t, err, ErrInvalidPluginBinding)
}

func TestPluginService_CreatePlugin_RejectsInvalidLocalTemplateFields(t *testing.T) {
	t.Parallel()

	svc, err := NewPluginService(t.TempDir())
	require.NoError(t, err)

	_, err = svc.CreatePlugin(context.Background(), CreatePluginRequest{
		Name:    "bad-template",
		Type:    PluginTypeAPIPrompt,
		Enabled: true,
		APIPrompt: &APIPromptPluginConfig{
			Templates: []APIPromptTemplate{
				{ID: "", Name: "Missing ID", Prompt: "Prompt", Enabled: true},
			},
		},
	})
	require.ErrorIs(t, err, ErrInvalidPluginTemplate)
}

func TestResolveDefaultPluginRootDirFrom_PrefersRepoRootPluginsWhenStartedInBackend(t *testing.T) {
	repoRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "backend"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "frontend"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, "plugins"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "backend", "go.mod"), []byte("module example.com/sub2api\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "frontend", "package.json"), []byte("{\"name\":\"frontend\"}\n"), 0o644))

	resolved := resolveDefaultPluginRootDirFromWithDataDir(filepath.Join(repoRoot, "backend"), filepath.Join(t.TempDir(), "data"))
	require.Equal(t, filepath.Join(repoRoot, "plugins"), resolved)
}

func TestResolveDefaultPluginRootDirFrom_FallsBackToCurrentWorkingDirPluginsOutsideRepo(t *testing.T) {
	startDir := t.TempDir()
	require.Equal(t, filepath.Join(startDir, "plugins"), resolveDefaultPluginRootDirFromWithDataDir(startDir, "."))
}

func TestResolveDefaultPluginRootDirFrom_UsesDataDirPluginsOutsideRepo(t *testing.T) {
	startDir := t.TempDir()
	dataDir := t.TempDir()

	require.Equal(t, filepath.Join(dataDir, "plugins"), resolveDefaultPluginRootDirFromWithDataDir(startDir, dataDir))
}

func ptrStringPlugin(value string) *string {
	return &value
}
