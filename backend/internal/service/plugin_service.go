package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type PluginType string

const (
	PluginTypeAPIPrompt PluginType = "api-prompt"
)

type PluginPromptTarget string

const (
	PluginPromptTargetAnthropicMessages     PluginPromptTarget = "anthropic_messages"
	PluginPromptTargetOpenAIChatCompletions PluginPromptTarget = "openai_chat_completions"
	PluginPromptTargetOpenAIResponses       PluginPromptTarget = "openai_responses"
	PluginPromptTargetGeminiGenerateContent PluginPromptTarget = "gemini_generate_content"
)

var (
	ErrPluginNotFound          = infraerrors.NotFound("PLUGIN_NOT_FOUND", "plugin not found")
	ErrPluginExists            = infraerrors.Conflict("PLUGIN_ALREADY_EXISTS", "plugin already exists")
	ErrInvalidPluginName       = infraerrors.BadRequest("INVALID_PLUGIN_NAME", "plugin name can only contain letters, numbers, underscores, and hyphens")
	ErrInvalidPluginType       = infraerrors.BadRequest("INVALID_PLUGIN_TYPE", "plugin type is not supported")
	ErrInvalidPluginTemplate   = infraerrors.BadRequest("INVALID_PLUGIN_TEMPLATE", "plugin template configuration is invalid")
	ErrInvalidPluginBinding    = infraerrors.BadRequest("INVALID_PLUGIN_BINDING", "plugin binding is invalid")
	pluginNamePattern          = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	defaultPluginHTTPTimeout   = 5 * time.Second
	defaultPluginManifestPerm  = os.FileMode(0o644)
	defaultPluginDirectoryPerm = os.FileMode(0o755)
)

type APIPromptTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Prompt      string `json:"prompt"`
	Enabled     bool   `json:"enabled"`
	Builtin     bool   `json:"builtin"`
	SortOrder   int    `json:"sort_order"`
}

type APIPromptPluginConfig struct {
	Templates           []APIPromptTemplate `json:"templates"`
	Source              string              `json:"source,omitempty"`
	LastSyncedAt        *time.Time          `json:"last_synced_at,omitempty"`
	LastSyncError       string              `json:"last_sync_error,omitempty"`
	RemoteTemplateCount int                 `json:"remote_template_count,omitempty"`
}

type Plugin struct {
	Name             string                 `json:"name"`
	Type             PluginType             `json:"type"`
	Description      string                 `json:"description,omitempty"`
	BaseURL          string                 `json:"base_url,omitempty"`
	Enabled          bool                   `json:"enabled"`
	APIKeyConfigured bool                   `json:"api_key_configured"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	APIPrompt        *APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type CreatePluginRequest struct {
	Name        string                 `json:"name"`
	Type        PluginType             `json:"type"`
	Description string                 `json:"description"`
	BaseURL     string                 `json:"base_url"`
	APIKey      string                 `json:"api_key"`
	Enabled     bool                   `json:"enabled"`
	APIPrompt   *APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type UpdatePluginRequest struct {
	Description *string                `json:"description,omitempty"`
	BaseURL     *string                `json:"base_url,omitempty"`
	APIKey      *string                `json:"api_key,omitempty"`
	Enabled     *bool                  `json:"enabled,omitempty"`
	APIPrompt   *APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type PluginTestResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	CheckedAt string `json:"checked_at"`
}

type APIPromptTemplateOption struct {
	PluginName    string     `json:"plugin_name"`
	TemplateID    string     `json:"template_id"`
	Name          string     `json:"name"`
	Description   string     `json:"description,omitempty"`
	Prompt        string     `json:"prompt"`
	Builtin       bool       `json:"builtin"`
	SortOrder     int        `json:"sort_order"`
	Source        string     `json:"source,omitempty"`
	Status        string     `json:"status,omitempty"`
	LastSyncedAt  *time.Time `json:"last_synced_at,omitempty"`
	LastSyncError string     `json:"last_sync_error,omitempty"`
}

type PluginDriver interface {
	Type() PluginType
	Test(ctx context.Context, record *pluginDiskRecord) (*PluginTestResult, error)
	ListAPIPromptTemplateOptions(ctx context.Context, record *pluginDiskRecord) ([]APIPromptTemplateOption, error)
	ValidateAPIPromptBinding(ctx context.Context, record *pluginDiskRecord, templateID string) error
	RenderAPIPrompt(ctx context.Context, record *pluginDiskRecord, templateID string, target PluginPromptTarget) (*APIPromptTemplateOption, error)
}

type pluginManifest struct {
	Name        string     `json:"name"`
	Type        PluginType `json:"type"`
	Description string     `json:"description,omitempty"`
	BaseURL     string     `json:"base_url,omitempty"`
	APIKey      string     `json:"api_key,omitempty"`
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type pluginDiskRecord struct {
	Manifest pluginManifest
	Config   *APIPromptPluginConfig
}

type PluginService struct {
	rootDir    string
	httpClient *http.Client
	drivers    map[PluginType]PluginDriver

	mu      sync.RWMutex
	plugins map[string]*pluginDiskRecord
}

func NewPluginService(rootDir string) (*PluginService, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		rootDir = filepath.Join(".", "plugins")
	}
	if err := os.MkdirAll(rootDir, defaultPluginDirectoryPerm); err != nil {
		return nil, fmt.Errorf("create plugin root dir: %w", err)
	}
	svc := &PluginService{
		rootDir: rootDir,
		httpClient: &http.Client{
			Timeout: defaultPluginHTTPTimeout,
		},
		plugins: make(map[string]*pluginDiskRecord),
	}
	svc.drivers = map[PluginType]PluginDriver{
		PluginTypeAPIPrompt: &apiPromptDriver{service: svc},
	}
	if err := svc.reloadFromDisk(); err != nil {
		return nil, err
	}
	return svc, nil
}

func ProvidePluginService() (*PluginService, error) {
	return NewPluginService(filepath.Join(".", "plugins"))
}

func (s *PluginService) ListPlugins(ctx context.Context) ([]Plugin, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.plugins))
	for name := range s.plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	plugins := make([]Plugin, 0, len(names))
	for _, name := range names {
		plugins = append(plugins, s.toPublicPlugin(s.plugins[name]))
	}
	return plugins, nil
}

func (s *PluginService) GetPlugin(ctx context.Context, name string) (*Plugin, error) {
	_ = ctx
	name, err := normalizePluginName(name)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.plugins[name]
	if !ok {
		return nil, ErrPluginNotFound
	}
	plugin := s.toPublicPlugin(record)
	return &plugin, nil
}

func (s *PluginService) CreatePlugin(ctx context.Context, req CreatePluginRequest) (*Plugin, error) {
	_ = ctx
	name, err := normalizePluginName(req.Name)
	if err != nil {
		return nil, err
	}
	if !isSupportedPluginType(req.Type) {
		return nil, ErrInvalidPluginType
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.plugins[name]; exists {
		return nil, ErrPluginExists
	}

	now := time.Now()
	record, err := s.buildRecord(name, req.Type, pluginManifest{
		Name:        name,
		Type:        req.Type,
		Description: strings.TrimSpace(req.Description),
		BaseURL:     normalizePluginBaseURL(req.BaseURL),
		APIKey:      strings.TrimSpace(req.APIKey),
		Enabled:     req.Enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, req.APIPrompt, nil)
	if err != nil {
		return nil, err
	}
	if err := s.writeRecord(record); err != nil {
		return nil, err
	}
	s.plugins[name] = record
	plugin := s.toPublicPlugin(record)
	return &plugin, nil
}

func (s *PluginService) UpdatePlugin(ctx context.Context, name string, req UpdatePluginRequest) (*Plugin, error) {
	_ = ctx
	name, err := normalizePluginName(name)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.plugins[name]
	if !ok {
		return nil, ErrPluginNotFound
	}

	nextManifest := record.Manifest
	if req.Description != nil {
		nextManifest.Description = strings.TrimSpace(*req.Description)
	}
	if req.BaseURL != nil {
		nextManifest.BaseURL = normalizePluginBaseURL(*req.BaseURL)
	}
	if req.APIKey != nil {
		nextManifest.APIKey = strings.TrimSpace(*req.APIKey)
	}
	if req.Enabled != nil {
		nextManifest.Enabled = *req.Enabled
	}
	nextManifest.UpdatedAt = time.Now()

	nextRecord, err := s.buildRecord(name, nextManifest.Type, nextManifest, req.APIPrompt, record.Config)
	if err != nil {
		return nil, err
	}
	if err := s.writeRecord(nextRecord); err != nil {
		return nil, err
	}
	s.plugins[name] = nextRecord
	plugin := s.toPublicPlugin(nextRecord)
	return &plugin, nil
}

func (s *PluginService) SetPluginEnabled(ctx context.Context, name string, enabled bool) (*Plugin, error) {
	return s.UpdatePlugin(ctx, name, UpdatePluginRequest{Enabled: &enabled})
}

func (s *PluginService) TestPlugin(ctx context.Context, name string) (*PluginTestResult, error) {
	name, err := normalizePluginName(name)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	record, ok := s.plugins[name]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrPluginNotFound
	}
	record = clonePluginRecord(record)

	driver, ok := s.drivers[record.Manifest.Type]
	if !ok {
		return nil, ErrInvalidPluginType
	}
	return driver.Test(ctx, record)
}

func (s *PluginService) ListAPIPromptTemplateOptions(ctx context.Context) ([]APIPromptTemplateOption, error) {
	s.mu.RLock()
	records := make([]*pluginDiskRecord, 0, len(s.plugins))
	for _, record := range s.plugins {
		if record.Manifest.Type == PluginTypeAPIPrompt && record.Manifest.Enabled && record.Config != nil {
			records = append(records, clonePluginRecord(record))
		}
	}
	s.mu.RUnlock()

	options := make([]APIPromptTemplateOption, 0)
	driver, ok := s.drivers[PluginTypeAPIPrompt]
	if !ok {
		return nil, ErrInvalidPluginType
	}
	for _, record := range records {
		recordOptions, err := driver.ListAPIPromptTemplateOptions(ctx, record)
		if err != nil {
			continue
		}
		options = append(options, recordOptions...)
	}

	sort.Slice(options, func(i, j int) bool {
		if options[i].PluginName != options[j].PluginName {
			return options[i].PluginName < options[j].PluginName
		}
		if options[i].SortOrder != options[j].SortOrder {
			return options[i].SortOrder < options[j].SortOrder
		}
		if options[i].Name != options[j].Name {
			return options[i].Name < options[j].Name
		}
		return options[i].TemplateID < options[j].TemplateID
	})
	return options, nil
}

func (s *PluginService) ValidateAPIKeyPluginSettings(ctx context.Context, settings domain.APIKeyPluginSettings) (domain.APIKeyPluginSettings, error) {
	_ = ctx
	if settings.APIPrompt == nil {
		return domain.APIKeyPluginSettings{}, nil
	}

	pluginName, err := normalizePluginName(settings.APIPrompt.PluginName)
	if err != nil {
		return domain.APIKeyPluginSettings{}, ErrInvalidPluginBinding
	}
	templateID := strings.TrimSpace(settings.APIPrompt.TemplateID)
	if templateID == "" {
		return domain.APIKeyPluginSettings{}, ErrInvalidPluginBinding
	}

	s.mu.RLock()
	record, ok := s.plugins[pluginName]
	if !ok || !record.Manifest.Enabled || record.Manifest.Type != PluginTypeAPIPrompt || record.Config == nil {
		s.mu.RUnlock()
		return domain.APIKeyPluginSettings{}, ErrInvalidPluginBinding
	}
	record = clonePluginRecord(record)
	s.mu.RUnlock()

	driver, ok := s.drivers[PluginTypeAPIPrompt]
	if !ok {
		return domain.APIKeyPluginSettings{}, ErrInvalidPluginType
	}
	if err := driver.ValidateAPIPromptBinding(ctx, record, templateID); err != nil {
		return domain.APIKeyPluginSettings{}, err
	}
	return domain.APIKeyPluginSettings{
		APIPrompt: &domain.APIPromptKeyBinding{
			PluginName: pluginName,
			TemplateID: templateID,
		},
	}, nil
}

func (s *PluginService) ApplyBoundPromptTemplate(ctx context.Context, body []byte, target PluginPromptTarget, settings domain.APIKeyPluginSettings) ([]byte, *APIPromptTemplateOption, error) {
	if len(body) == 0 || settings.APIPrompt == nil {
		return body, nil, nil
	}

	resolved := s.resolvePromptTemplate(ctx, settings, target)
	if resolved == nil {
		return body, nil, nil
	}

	switch target {
	case PluginPromptTargetAnthropicMessages:
		updated, err := prependAnthropicSystemPrompt(body, resolved.Prompt)
		return updated, resolved, err
	case PluginPromptTargetOpenAIChatCompletions:
		updated, err := prependOpenAIChatSystemPrompt(body, resolved.Prompt)
		return updated, resolved, err
	case PluginPromptTargetOpenAIResponses:
		updated, err := prependOpenAIResponsesInstructions(body, resolved.Prompt)
		return updated, resolved, err
	case PluginPromptTargetGeminiGenerateContent:
		updated, err := prependGeminiSystemInstruction(body, resolved.Prompt)
		return updated, resolved, err
	default:
		return body, nil, nil
	}
}

func (s *PluginService) resolvePromptTemplate(ctx context.Context, settings domain.APIKeyPluginSettings, target PluginPromptTarget) *APIPromptTemplateOption {
	if settings.APIPrompt == nil {
		return nil
	}
	pluginName, err := normalizePluginName(settings.APIPrompt.PluginName)
	if err != nil {
		return nil
	}
	templateID := strings.TrimSpace(settings.APIPrompt.TemplateID)
	if templateID == "" {
		return nil
	}

	s.mu.RLock()
	record, ok := s.plugins[pluginName]
	if !ok || !record.Manifest.Enabled || record.Manifest.Type != PluginTypeAPIPrompt || record.Config == nil {
		s.mu.RUnlock()
		return nil
	}
	record = clonePluginRecord(record)
	s.mu.RUnlock()

	driver, ok := s.drivers[PluginTypeAPIPrompt]
	if !ok {
		return nil
	}
	resolved, err := driver.RenderAPIPrompt(ctx, record, templateID, target)
	if err != nil {
		return nil
	}
	return resolved
}

func (s *PluginService) reloadFromDisk() error {
	entries, err := os.ReadDir(s.rootDir)
	if err != nil {
		return fmt.Errorf("read plugin root dir: %w", err)
	}

	plugins := make(map[string]*pluginDiskRecord)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := s.readRecord(filepath.Join(s.rootDir, entry.Name()))
		if err != nil {
			return err
		}
		plugins[record.Manifest.Name] = record
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.plugins = plugins
	return nil
}

func (s *PluginService) readRecord(pluginDir string) (*pluginDiskRecord, error) {
	manifestPath := filepath.Join(pluginDir, "manifest.json")
	configPath := filepath.Join(pluginDir, "config.json")

	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read plugin manifest %q: %w", manifestPath, err)
	}
	var manifest pluginManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("decode plugin manifest %q: %w", manifestPath, err)
	}
	name, err := normalizePluginName(manifest.Name)
	if err != nil {
		return nil, err
	}
	if !isSupportedPluginType(manifest.Type) {
		return nil, ErrInvalidPluginType
	}
	manifest.Name = name
	manifest.BaseURL = normalizePluginBaseURL(manifest.BaseURL)

	record := &pluginDiskRecord{Manifest: manifest}
	if manifest.Type == PluginTypeAPIPrompt {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("read plugin config %q: %w", configPath, err)
		}
		var config APIPromptPluginConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("decode plugin config %q: %w", configPath, err)
		}
		normalizedConfig, err := normalizeAPIPromptConfigWithPromptRequirement(&config, false, !hasRemoteManifestEndpoint(manifest))
		if err != nil {
			return nil, err
		}
		record.Config = normalizedConfig
	}
	return record, nil
}

func (s *PluginService) writeRecord(record *pluginDiskRecord) error {
	pluginDir := filepath.Join(s.rootDir, record.Manifest.Name)
	if err := os.MkdirAll(pluginDir, defaultPluginDirectoryPerm); err != nil {
		return fmt.Errorf("create plugin dir %q: %w", pluginDir, err)
	}

	manifestPath := filepath.Join(pluginDir, "manifest.json")
	manifestData, err := json.MarshalIndent(record.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), defaultPluginManifestPerm); err != nil {
		return fmt.Errorf("write plugin manifest %q: %w", manifestPath, err)
	}

	configPath := filepath.Join(pluginDir, "config.json")
	switch record.Manifest.Type {
	case PluginTypeAPIPrompt:
		configData, err := json.MarshalIndent(record.Config, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal plugin config: %w", err)
		}
		if err := os.WriteFile(configPath, append(configData, '\n'), defaultPluginManifestPerm); err != nil {
			return fmt.Errorf("write plugin config %q: %w", configPath, err)
		}
	default:
		return ErrInvalidPluginType
	}
	return nil
}

func (s *PluginService) buildRecord(name string, pluginType PluginType, manifest pluginManifest, nextConfig *APIPromptPluginConfig, currentConfig *APIPromptPluginConfig) (*pluginDiskRecord, error) {
	record := &pluginDiskRecord{Manifest: manifest}
	switch pluginType {
	case PluginTypeAPIPrompt:
		cfg := nextConfig
		if cfg == nil {
			cfg = currentConfig
		}
		normalizedConfig, err := normalizeAPIPromptConfigWithPromptRequirement(cfg, currentConfig == nil, !hasRemoteManifestEndpoint(manifest))
		if err != nil {
			return nil, err
		}
		record.Config = normalizedConfig
	default:
		return nil, ErrInvalidPluginType
	}
	record.Manifest.Name = name
	record.Manifest.Type = pluginType
	return record, nil
}

func (s *PluginService) toPublicPlugin(record *pluginDiskRecord) Plugin {
	plugin := Plugin{
		Name:             record.Manifest.Name,
		Type:             record.Manifest.Type,
		Description:      record.Manifest.Description,
		BaseURL:          record.Manifest.BaseURL,
		Enabled:          record.Manifest.Enabled,
		APIKeyConfigured: strings.TrimSpace(record.Manifest.APIKey) != "",
		CreatedAt:        record.Manifest.CreatedAt,
		UpdatedAt:        record.Manifest.UpdatedAt,
	}
	if record.Config != nil {
		plugin.APIPrompt = cloneAPIPromptConfig(record.Config)
	}
	return plugin
}

type apiPromptDriver struct {
	service *PluginService
}

func (d *apiPromptDriver) Type() PluginType {
	return PluginTypeAPIPrompt
}

func (d *apiPromptDriver) Test(ctx context.Context, record *pluginDiskRecord) (*PluginTestResult, error) {
	result := &PluginTestResult{
		CheckedAt: time.Now().Format(time.RFC3339),
	}

	if hasRemotePluginEndpoint(record) {
		if err := d.checkRemoteHealth(ctx, record); err != nil {
			result.OK = false
			result.Message = "External api-prompt health check failed: " + err.Error()
			return result, nil
		}
		templates, err := d.fetchRemoteTemplates(ctx, record)
		if err != nil {
			_ = d.service.markAPIPromptSyncError(record.Manifest.Name, err)
			result.OK = false
			result.Message = "External api-prompt template sync failed: " + err.Error()
			return result, nil
		}
		cfg := remoteAPIPromptConfig(templates, time.Now(), "")
		_ = d.service.saveAPIPromptConfig(record.Manifest.Name, cfg)
		templateCount := countEnabledAPIPromptTemplates(templates, false)
		if templateCount == 0 {
			result.OK = false
			result.Message = "External api-prompt has no enabled templates"
			return result, nil
		}
		result.OK = true
		result.Message = fmt.Sprintf("External api-prompt is ready with %d remote templates", templateCount)
		return result, nil
	}

	templateCount := countEnabledAPIPromptTemplates(record.Config.Templates, true)
	if templateCount == 0 {
		result.OK = false
		result.Message = "api-prompt has no enabled templates"
		return result, nil
	}
	result.OK = true
	result.Message = fmt.Sprintf("api-prompt is ready with %d enabled templates", templateCount)
	return result, nil
}

func (d *apiPromptDriver) ListAPIPromptTemplateOptions(ctx context.Context, record *pluginDiskRecord) ([]APIPromptTemplateOption, error) {
	if hasRemotePluginEndpoint(record) {
		templates, err := d.fetchRemoteTemplates(ctx, record)
		if err == nil {
			now := time.Now()
			cfg := remoteAPIPromptConfig(templates, now, "")
			_ = d.service.saveAPIPromptConfig(record.Manifest.Name, cfg)
			return buildAPIPromptTemplateOptions(record.Manifest.Name, cfg, "remote", "available", false), nil
		}
		_ = d.service.markAPIPromptSyncError(record.Manifest.Name, err)
		cfg := cloneAPIPromptConfig(record.Config)
		cfg.Source = "cache"
		cfg.LastSyncError = err.Error()
		return buildAPIPromptTemplateOptions(record.Manifest.Name, cfg, "cache", "degraded", false), nil
	}

	cfg := cloneAPIPromptConfig(record.Config)
	cfg.Source = "local"
	return buildAPIPromptTemplateOptions(record.Manifest.Name, cfg, "local", "available", true), nil
}

func (d *apiPromptDriver) ValidateAPIPromptBinding(ctx context.Context, record *pluginDiskRecord, templateID string) error {
	if hasRemotePluginEndpoint(record) {
		templates, err := d.fetchRemoteTemplates(ctx, record)
		if err != nil {
			_ = d.service.markAPIPromptSyncError(record.Manifest.Name, err)
			return ErrInvalidPluginBinding
		}
		cfg := remoteAPIPromptConfig(templates, time.Now(), "")
		_ = d.service.saveAPIPromptConfig(record.Manifest.Name, cfg)
		if findAPIPromptTemplate(cfg.Templates, templateID, false) != nil {
			return nil
		}
		return ErrInvalidPluginBinding
	}

	if findAPIPromptTemplate(record.Config.Templates, templateID, true) == nil {
		return ErrInvalidPluginBinding
	}
	return nil
}

func (d *apiPromptDriver) RenderAPIPrompt(ctx context.Context, record *pluginDiskRecord, templateID string, target PluginPromptTarget) (*APIPromptTemplateOption, error) {
	if hasRemotePluginEndpoint(record) {
		rendered, err := d.renderRemotePrompt(ctx, record, templateID, target)
		if err == nil {
			cached := findAPIPromptTemplate(record.Config.Templates, templateID, false)
			option := APIPromptTemplateOption{
				PluginName: record.Manifest.Name,
				TemplateID: templateID,
				Name:       templateID,
				Prompt:     rendered,
				Source:     "remote",
				Status:     "available",
			}
			if cached != nil {
				option.Name = cached.Name
				option.Description = cached.Description
				option.Builtin = cached.Builtin
				option.SortOrder = cached.SortOrder
			}
			return &option, nil
		}
		_ = d.service.markAPIPromptSyncError(record.Manifest.Name, err)
	}

	tpl := findAPIPromptTemplate(record.Config.Templates, templateID, true)
	if tpl == nil {
		return nil, ErrInvalidPluginBinding
	}
	source := strings.TrimSpace(record.Config.Source)
	if source == "" {
		source = "local"
	}
	if hasRemotePluginEndpoint(record) {
		source = "cache"
	}
	return &APIPromptTemplateOption{
		PluginName:    record.Manifest.Name,
		TemplateID:    templateID,
		Name:          tpl.Name,
		Description:   tpl.Description,
		Prompt:        tpl.Prompt,
		Builtin:       tpl.Builtin,
		SortOrder:     tpl.SortOrder,
		Source:        source,
		Status:        "available",
		LastSyncedAt:  record.Config.LastSyncedAt,
		LastSyncError: record.Config.LastSyncError,
	}, nil
}

func (d *apiPromptDriver) checkRemoteHealth(ctx context.Context, record *pluginDiskRecord) error {
	req, err := d.newRemoteRequest(ctx, record, http.MethodGet, "/health", nil)
	if err != nil {
		return fmt.Errorf("build plugin health request: %w", err)
	}
	resp, err := d.service.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func (d *apiPromptDriver) fetchRemoteTemplates(ctx context.Context, record *pluginDiskRecord) ([]APIPromptTemplate, error) {
	req, err := d.newRemoteRequest(ctx, record, http.MethodGet, "/v1/templates", nil)
	if err != nil {
		return nil, fmt.Errorf("build api-prompt templates request: %w", err)
	}
	data, err := d.doRemoteJSON(req)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Templates []APIPromptTemplate `json:"templates"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Templates != nil {
		cfg, err := normalizeAPIPromptConfigWithPromptRequirement(&APIPromptPluginConfig{Templates: envelope.Templates}, false, false)
		if err != nil {
			return nil, err
		}
		return cfg.Templates, nil
	}

	var templates []APIPromptTemplate
	if err := json.Unmarshal(data, &templates); err != nil {
		return nil, fmt.Errorf("decode api-prompt templates response: %w", err)
	}
	cfg, err := normalizeAPIPromptConfigWithPromptRequirement(&APIPromptPluginConfig{Templates: templates}, false, false)
	if err != nil {
		return nil, err
	}
	return cfg.Templates, nil
}

func (d *apiPromptDriver) renderRemotePrompt(ctx context.Context, record *pluginDiskRecord, templateID string, target PluginPromptTarget) (string, error) {
	payload := map[string]any{
		"plugin_name": record.Manifest.Name,
		"template_id": templateID,
		"target":      target,
		"context": map[string]any{
			"plugin_type": record.Manifest.Type,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := d.newRemoteRequest(ctx, record, http.MethodPost, "/v1/render", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build api-prompt render request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	data, err := d.doRemoteJSON(req)
	if err != nil {
		return "", err
	}
	var resp struct {
		Prompt            string `json:"prompt"`
		SystemInstruction string `json:"system_instruction"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("decode api-prompt render response: %w", err)
	}
	prompt := strings.TrimSpace(resp.Prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(resp.SystemInstruction)
	}
	if prompt == "" {
		return "", fmt.Errorf("api-prompt render response has no prompt")
	}
	return prompt, nil
}

func (d *apiPromptDriver) newRemoteRequest(ctx context.Context, record *pluginDiskRecord, method string, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(record.Manifest.BaseURL, "/")+path, body)
	if err != nil {
		return nil, err
	}
	if apiKey := strings.TrimSpace(record.Manifest.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("x-api-key", apiKey)
	}
	return req, nil
}

func (d *apiPromptDriver) doRemoteJSON(req *http.Request) ([]byte, error) {
	resp, err := d.service.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func (s *PluginService) saveAPIPromptConfig(name string, cfg *APIPromptPluginConfig) error {
	name, err := normalizePluginName(name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.plugins[name]
	if !ok || record.Manifest.Type != PluginTypeAPIPrompt {
		return ErrPluginNotFound
	}
	nextRecord := clonePluginRecord(record)
	nextRecord.Config = cloneAPIPromptConfig(cfg)
	if err := s.writeRecord(nextRecord); err != nil {
		return err
	}
	s.plugins[name] = nextRecord
	return nil
}

func (s *PluginService) markAPIPromptSyncError(name string, syncErr error) error {
	name, err := normalizePluginName(name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.plugins[name]
	if !ok || record.Manifest.Type != PluginTypeAPIPrompt || record.Config == nil {
		return ErrPluginNotFound
	}
	nextRecord := clonePluginRecord(record)
	nextRecord.Config.Source = "cache"
	if syncErr != nil {
		nextRecord.Config.LastSyncError = syncErr.Error()
	}
	if err := s.writeRecord(nextRecord); err != nil {
		return err
	}
	s.plugins[name] = nextRecord
	return nil
}

func hasRemotePluginEndpoint(record *pluginDiskRecord) bool {
	return record != nil && strings.TrimSpace(record.Manifest.BaseURL) != ""
}

func hasRemoteManifestEndpoint(manifest pluginManifest) bool {
	return strings.TrimSpace(manifest.BaseURL) != ""
}

func remoteAPIPromptConfig(templates []APIPromptTemplate, syncedAt time.Time, syncErr string) *APIPromptPluginConfig {
	return &APIPromptPluginConfig{
		Templates:           templates,
		Source:              "remote",
		LastSyncedAt:        &syncedAt,
		LastSyncError:       strings.TrimSpace(syncErr),
		RemoteTemplateCount: len(templates),
	}
}

func buildAPIPromptTemplateOptions(pluginName string, cfg *APIPromptPluginConfig, source string, status string, requirePrompt bool) []APIPromptTemplateOption {
	if cfg == nil {
		return nil
	}
	options := make([]APIPromptTemplateOption, 0, len(cfg.Templates))
	for _, tpl := range cfg.Templates {
		if !tpl.Enabled {
			continue
		}
		if requirePrompt && strings.TrimSpace(tpl.Prompt) == "" {
			continue
		}
		options = append(options, APIPromptTemplateOption{
			PluginName:    pluginName,
			TemplateID:    tpl.ID,
			Name:          tpl.Name,
			Description:   tpl.Description,
			Prompt:        tpl.Prompt,
			Builtin:       tpl.Builtin,
			SortOrder:     tpl.SortOrder,
			Source:        source,
			Status:        status,
			LastSyncedAt:  cfg.LastSyncedAt,
			LastSyncError: cfg.LastSyncError,
		})
	}
	return options
}

func countEnabledAPIPromptTemplates(templates []APIPromptTemplate, requirePrompt bool) int {
	count := 0
	for _, tpl := range templates {
		if !tpl.Enabled {
			continue
		}
		if requirePrompt && strings.TrimSpace(tpl.Prompt) == "" {
			continue
		}
		count++
	}
	return count
}

func findAPIPromptTemplate(templates []APIPromptTemplate, templateID string, requirePrompt bool) *APIPromptTemplate {
	templateID = strings.TrimSpace(templateID)
	for idx := range templates {
		tpl := &templates[idx]
		if tpl.ID != templateID || !tpl.Enabled {
			continue
		}
		if requirePrompt && strings.TrimSpace(tpl.Prompt) == "" {
			continue
		}
		return tpl
	}
	return nil
}

func clonePluginRecord(record *pluginDiskRecord) *pluginDiskRecord {
	if record == nil {
		return nil
	}
	return &pluginDiskRecord{
		Manifest: record.Manifest,
		Config:   cloneAPIPromptConfig(record.Config),
	}
}

func normalizePluginName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !pluginNamePattern.MatchString(name) {
		return "", ErrInvalidPluginName
	}
	return name, nil
}

func isSupportedPluginType(pluginType PluginType) bool {
	switch pluginType {
	case PluginTypeAPIPrompt:
		return true
	default:
		return false
	}
}

func normalizePluginBaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func normalizeAPIPromptConfig(cfg *APIPromptPluginConfig, useDefaults bool) (*APIPromptPluginConfig, error) {
	return normalizeAPIPromptConfigWithPromptRequirement(cfg, useDefaults, true)
}

func normalizeAPIPromptConfigWithPromptRequirement(cfg *APIPromptPluginConfig, useDefaults bool, requirePrompt bool) (*APIPromptPluginConfig, error) {
	if cfg == nil {
		if useDefaults {
			return &APIPromptPluginConfig{Templates: defaultAPIPromptTemplates()}, nil
		}
		return &APIPromptPluginConfig{Templates: []APIPromptTemplate{}}, nil
	}

	templates := cfg.Templates
	if len(templates) == 0 && useDefaults {
		templates = defaultAPIPromptTemplates()
	}

	normalized := make([]APIPromptTemplate, 0, len(templates))
	seenIDs := make(map[string]struct{}, len(templates))
	for idx, tpl := range templates {
		tpl.Name = strings.TrimSpace(tpl.Name)
		tpl.Description = strings.TrimSpace(tpl.Description)
		tpl.Prompt = strings.TrimSpace(tpl.Prompt)
		if tpl.Name == "" || (requirePrompt && tpl.Prompt == "") {
			return nil, ErrInvalidPluginTemplate
		}
		tpl.ID = strings.TrimSpace(tpl.ID)
		if tpl.ID == "" {
			tpl.ID = slugifyPromptTemplateID(tpl.Name)
		}
		if tpl.ID == "" {
			return nil, ErrInvalidPluginTemplate
		}
		if _, exists := seenIDs[tpl.ID]; exists {
			return nil, ErrInvalidPluginTemplate
		}
		seenIDs[tpl.ID] = struct{}{}
		if tpl.SortOrder == 0 {
			tpl.SortOrder = (idx + 1) * 10
		}
		normalized = append(normalized, tpl)
	}

	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].SortOrder != normalized[j].SortOrder {
			return normalized[i].SortOrder < normalized[j].SortOrder
		}
		if normalized[i].Name != normalized[j].Name {
			return normalized[i].Name < normalized[j].Name
		}
		return normalized[i].ID < normalized[j].ID
	})
	return &APIPromptPluginConfig{
		Templates:           normalized,
		Source:              strings.TrimSpace(cfg.Source),
		LastSyncedAt:        cfg.LastSyncedAt,
		LastSyncError:       strings.TrimSpace(cfg.LastSyncError),
		RemoteTemplateCount: cfg.RemoteTemplateCount,
	}, nil
}

func defaultAPIPromptTemplates() []APIPromptTemplate {
	return []APIPromptTemplate{
		{
			ID:          "general-writing",
			Name:        "通用写作助手",
			Description: "提升结构化表达、条理和收束能力。",
			Prompt:      "你是一位结构清晰、表达克制、擅长把复杂问题讲明白的专业助手。优先给出可执行、准确、分层清晰的答案。",
			Enabled:     true,
			Builtin:     true,
			SortOrder:   10,
		},
		{
			ID:          "engineering-review",
			Name:        "工程审查助手",
			Description: "更强调正确性、边界条件与风险识别。",
			Prompt:      "你是一位严谨的工程审查助手。回答时优先关注正确性、边界条件、异常路径、兼容性和可维护性，并明确指出潜在风险。",
			Enabled:     true,
			Builtin:     true,
			SortOrder:   20,
		},
		{
			ID:          "product-ops",
			Name:        "产品运营助手",
			Description: "更强调用户视角、落地步骤和沟通措辞。",
			Prompt:      "你是一位兼顾产品与运营的助手。回答时优先从用户目标、执行步骤、沟通话术和结果衡量角度组织内容。",
			Enabled:     true,
			Builtin:     true,
			SortOrder:   30,
		},
	}
}

func cloneAPIPromptConfig(cfg *APIPromptPluginConfig) *APIPromptPluginConfig {
	if cfg == nil {
		return nil
	}
	templates := make([]APIPromptTemplate, len(cfg.Templates))
	copy(templates, cfg.Templates)
	return &APIPromptPluginConfig{
		Templates:           templates,
		Source:              cfg.Source,
		LastSyncedAt:        cfg.LastSyncedAt,
		LastSyncError:       cfg.LastSyncError,
		RemoteTemplateCount: cfg.RemoteTemplateCount,
	}
}

func slugifyPromptTemplateID(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == ' ':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func prependAnthropicSystemPrompt(body []byte, prompt string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	systemItems := []any{
		map[string]any{
			"type": "text",
			"text": prompt,
		},
	}
	switch current := payload["system"].(type) {
	case string:
		if strings.TrimSpace(current) != "" {
			systemItems = append(systemItems, map[string]any{"type": "text", "text": current})
		}
	case []any:
		systemItems = append(systemItems, current...)
	case nil:
	}
	payload["system"] = systemItems
	return json.Marshal(payload)
}

func prependOpenAIChatSystemPrompt(body []byte, prompt string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	systemMessage := map[string]any{
		"role":    "system",
		"content": prompt,
	}
	messages, _ := payload["messages"].([]any)
	payload["messages"] = append([]any{systemMessage}, messages...)
	return json.Marshal(payload)
}

func prependOpenAIResponsesInstructions(body []byte, prompt string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	if current, ok := payload["instructions"].(string); ok && strings.TrimSpace(current) != "" {
		payload["instructions"] = prompt + "\n\n" + current
	} else {
		payload["instructions"] = prompt
	}
	return json.Marshal(payload)
}

func prependGeminiSystemInstruction(body []byte, prompt string) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	parts := []any{map[string]any{"text": prompt}}
	if current, ok := payload["systemInstruction"].(map[string]any); ok {
		if currentParts, ok := current["parts"].([]any); ok {
			parts = append(parts, currentParts...)
		}
	}
	payload["systemInstruction"] = map[string]any{"parts": parts}
	return json.Marshal(payload)
}

func jsonEqualBytes(a, b []byte) bool {
	return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
}
