package service

import (
	"context"
	"encoding/json"
	"fmt"
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
	Templates []APIPromptTemplate `json:"templates"`
	Source    string              `json:"source,omitempty"`
}

type Plugin struct {
	Name        string                 `json:"name"`
	Type        PluginType             `json:"type"`
	Description string                 `json:"description,omitempty"`
	Enabled     bool                   `json:"enabled"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	APIPrompt   *APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type CreatePluginRequest struct {
	Name        string                 `json:"name"`
	Type        PluginType             `json:"type"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
	APIPrompt   *APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type UpdatePluginRequest struct {
	Description *string                `json:"description,omitempty"`
	Enabled     *bool                  `json:"enabled,omitempty"`
	APIPrompt   *APIPromptPluginConfig `json:"api_prompt,omitempty"`
}

type PluginTestResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	CheckedAt string `json:"checked_at"`
}

type APIPromptTemplateOption struct {
	PluginName  string `json:"plugin_name"`
	TemplateID  string `json:"template_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Prompt      string `json:"prompt"`
	Builtin     bool   `json:"builtin"`
	SortOrder   int    `json:"sort_order"`
	Source      string `json:"source,omitempty"`
	Status      string `json:"status,omitempty"`
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
	Enabled     bool       `json:"enabled"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type pluginDiskRecord struct {
	Manifest pluginManifest
	Config   *APIPromptPluginConfig
}

type PluginService struct {
	rootDir string
	drivers map[PluginType]PluginDriver

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
		plugins: make(map[string]*pluginDiskRecord),
	}
	svc.drivers = map[PluginType]PluginDriver{
		PluginTypeAPIPrompt: &apiPromptDriver{},
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
		normalizedConfig, err := normalizeAPIPromptConfigWithPromptRequirement(&config, false, true)
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
		normalizedConfig, err := normalizeAPIPromptConfigWithPromptRequirement(cfg, currentConfig == nil, true)
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
		Name:        record.Manifest.Name,
		Type:        record.Manifest.Type,
		Description: record.Manifest.Description,
		Enabled:     record.Manifest.Enabled,
		CreatedAt:   record.Manifest.CreatedAt,
		UpdatedAt:   record.Manifest.UpdatedAt,
	}
	if record.Config != nil {
		plugin.APIPrompt = cloneAPIPromptConfig(record.Config)
	}
	return plugin
}

type apiPromptDriver struct{}

func (d *apiPromptDriver) Type() PluginType {
	return PluginTypeAPIPrompt
}

func (d *apiPromptDriver) Test(ctx context.Context, record *pluginDiskRecord) (*PluginTestResult, error) {
	_ = ctx
	result := &PluginTestResult{
		CheckedAt: time.Now().Format(time.RFC3339),
	}

	templateCount := countEnabledAPIPromptTemplates(record.Config.Templates, true)
	if templateCount == 0 {
		result.OK = false
		result.Message = "api-prompt has no enabled local templates"
		return result, nil
	}
	result.OK = true
	result.Message = fmt.Sprintf("api-prompt local configuration is ready with %d enabled templates", templateCount)
	return result, nil
}

func (d *apiPromptDriver) ListAPIPromptTemplateOptions(ctx context.Context, record *pluginDiskRecord) ([]APIPromptTemplateOption, error) {
	_ = ctx
	cfg := cloneAPIPromptConfig(record.Config)
	cfg.Source = "local"
	return buildAPIPromptTemplateOptions(record.Manifest.Name, cfg, "local", "available", true), nil
}

func (d *apiPromptDriver) ValidateAPIPromptBinding(ctx context.Context, record *pluginDiskRecord, templateID string) error {
	_ = ctx
	if findAPIPromptTemplate(record.Config.Templates, templateID, true) == nil {
		return ErrInvalidPluginBinding
	}
	return nil
}

func (d *apiPromptDriver) RenderAPIPrompt(ctx context.Context, record *pluginDiskRecord, templateID string, target PluginPromptTarget) (*APIPromptTemplateOption, error) {
	_ = ctx
	_ = target
	tpl := findAPIPromptTemplate(record.Config.Templates, templateID, true)
	if tpl == nil {
		return nil, ErrInvalidPluginBinding
	}
	source := strings.TrimSpace(record.Config.Source)
	if source == "" {
		source = "local"
	}
	return &APIPromptTemplateOption{
		PluginName:  record.Manifest.Name,
		TemplateID:  templateID,
		Name:        tpl.Name,
		Description: tpl.Description,
		Prompt:      tpl.Prompt,
		Builtin:     tpl.Builtin,
		SortOrder:   tpl.SortOrder,
		Source:      source,
		Status:      "available",
	}, nil
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
			PluginName:  pluginName,
			TemplateID:  tpl.ID,
			Name:        tpl.Name,
			Description: tpl.Description,
			Prompt:      tpl.Prompt,
			Builtin:     tpl.Builtin,
			SortOrder:   tpl.SortOrder,
			Source:      source,
			Status:      status,
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
		Templates: normalized,
		Source:    "local",
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
		Templates: templates,
		Source:    cfg.Source,
	}
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
