package service

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

const (
	SchedulingMechanismPlatformAll       = "all"
	SchedulingMechanismAccountTypeAll    = "all"
	defaultProxyFailoverProbeIntervalMin = 5
	defaultProxyFailoverFailureThreshold = 3
	defaultProxyFailoverFailureWindowMin = 10
	defaultProxyFailoverCooldownMin      = 15
	defaultProxyFailoverHalfOpenAccounts = 2
	defaultProxyFailoverBackoffFactor    = 2
	defaultProxyFailoverMaxCooldownMin   = 120
	defaultProxyFailoverMaxPerProxy      = 6
	defaultProxyFailoverMaxMigrateBatch  = 12
	defaultProxyFailoverTempUnschedMin   = 10
)

type SchedulingMechanism struct {
	ID                       string                  `json:"id"`
	Name                     string                  `json:"name"`
	Platform                 string                  `json:"platform"`
	AccountType              string                  `json:"account_type"`
	Enabled                  bool                    `json:"enabled"`
	Hidden                   bool                    `json:"hidden"`
	Description              string                  `json:"description,omitempty"`
	TempUnschedulableEnabled bool                    `json:"temp_unschedulable_enabled"`
	TempUnschedulableRules   []TempUnschedulableRule `json:"temp_unschedulable_rules,omitempty"`
	UpdatedAtUnix            int64                   `json:"updated_at_unix,omitempty"`
}

type ProxyFailoverSettings struct {
	Enabled               bool `json:"enabled"`
	AutoTestEnabled       bool `json:"auto_test_enabled"`
	ProbeIntervalMinutes  int  `json:"probe_interval_minutes"`
	FailureThreshold      int  `json:"failure_threshold"`
	FailureWindowMinutes  int  `json:"failure_window_minutes"`
	CooldownMinutes       int  `json:"cooldown_minutes"`
	HalfOpenProbeAccounts int  `json:"half_open_probe_accounts"`
	CooldownBackoffFactor int  `json:"cooldown_backoff_factor"`
	MaxCooldownMinutes    int  `json:"max_cooldown_minutes"`
	MaxAccountsPerProxy   int  `json:"max_accounts_per_proxy"`
	MaxMigrationsPerCycle int  `json:"max_migrations_per_cycle"`
	PreferSameCountry     bool `json:"prefer_same_country"`
	OnlyOpenAIOAuth       bool `json:"only_openai_oauth"`
	TempUnschedMinutes    int  `json:"temp_unsched_minutes"`
}

type SchedulingMechanismSettings struct {
	Mechanisms    []SchedulingMechanism `json:"mechanisms"`
	ProxyFailover ProxyFailoverSettings `json:"proxy_failover"`
}

func DefaultProxyFailoverSettings() ProxyFailoverSettings {
	return ProxyFailoverSettings{
		Enabled:               true,
		AutoTestEnabled:       true,
		ProbeIntervalMinutes:  defaultProxyFailoverProbeIntervalMin,
		FailureThreshold:      defaultProxyFailoverFailureThreshold,
		FailureWindowMinutes:  defaultProxyFailoverFailureWindowMin,
		CooldownMinutes:       defaultProxyFailoverCooldownMin,
		HalfOpenProbeAccounts: defaultProxyFailoverHalfOpenAccounts,
		CooldownBackoffFactor: defaultProxyFailoverBackoffFactor,
		MaxCooldownMinutes:    defaultProxyFailoverMaxCooldownMin,
		MaxAccountsPerProxy:   defaultProxyFailoverMaxPerProxy,
		MaxMigrationsPerCycle: defaultProxyFailoverMaxMigrateBatch,
		PreferSameCountry:     true,
		OnlyOpenAIOAuth:       false,
		TempUnschedMinutes:    defaultProxyFailoverTempUnschedMin,
	}
}

func DefaultSchedulingMechanismSettings() *SchedulingMechanismSettings {
	return &SchedulingMechanismSettings{
		Mechanisms:    []SchedulingMechanism{},
		ProxyFailover: DefaultProxyFailoverSettings(),
	}
}

func normalizeSchedulingMechanismSettings(settings *SchedulingMechanismSettings) *SchedulingMechanismSettings {
	if settings == nil {
		return DefaultSchedulingMechanismSettings()
	}

	normalized := &SchedulingMechanismSettings{
		Mechanisms:    make([]SchedulingMechanism, 0, len(settings.Mechanisms)),
		ProxyFailover: settings.ProxyFailover,
	}

	for _, mechanism := range settings.Mechanisms {
		mechanism.ID = strings.TrimSpace(mechanism.ID)
		mechanism.Name = strings.TrimSpace(mechanism.Name)
		mechanism.Platform = normalizeSchedulingMechanismPlatform(mechanism.Platform)
		mechanism.AccountType = normalizeSchedulingMechanismAccountType(mechanism.AccountType)
		mechanism.Description = strings.TrimSpace(mechanism.Description)
		if mechanism.Name == "" {
			continue
		}
		mechanism.TempUnschedulableRules = normalizeTempUnschedulableRules(mechanism.TempUnschedulableRules)
		if mechanism.TempUnschedulableEnabled && len(mechanism.TempUnschedulableRules) == 0 {
			continue
		}
		if mechanism.UpdatedAtUnix <= 0 {
			mechanism.UpdatedAtUnix = time.Now().Unix()
		}
		normalized.Mechanisms = append(normalized.Mechanisms, mechanism)
	}

	normalized.ProxyFailover = normalizeProxyFailoverSettings(normalized.ProxyFailover)
	return normalized
}

func normalizeSchedulingMechanismPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "", SchedulingMechanismPlatformAll:
		return SchedulingMechanismPlatformAll
	case PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity:
		return strings.ToLower(strings.TrimSpace(platform))
	default:
		return SchedulingMechanismPlatformAll
	}
}

func normalizeSchedulingMechanismAccountType(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "", SchedulingMechanismAccountTypeAll:
		return SchedulingMechanismAccountTypeAll
	case AccountTypeOAuth, AccountTypeAPIKey, AccountTypeChatAPI, AccountTypeSetupToken, AccountTypeUpstream, AccountTypeBedrock:
		return strings.ToLower(strings.TrimSpace(accountType))
	default:
		return SchedulingMechanismAccountTypeAll
	}
}

func normalizeTempUnschedulableRules(rules []TempUnschedulableRule) []TempUnschedulableRule {
	if len(rules) == 0 {
		return nil
	}
	normalized := make([]TempUnschedulableRule, 0, len(rules))
	for idx, rule := range rules {
		if rule.ErrorCode < 100 || rule.ErrorCode > 599 {
			continue
		}
		if rule.DurationMinutes < 1 {
			continue
		}
		keywords := make([]string, 0, len(rule.Keywords))
		for _, keyword := range rule.Keywords {
			trimmed := strings.TrimSpace(keyword)
			if trimmed == "" {
				continue
			}
			keywords = append(keywords, trimmed)
		}
		if len(keywords) == 0 {
			continue
		}
		rule.Keywords = keywords
		rule.Description = strings.TrimSpace(rule.Description)
		rule.ID = strings.TrimSpace(rule.ID)
		if rule.ID == "" {
			rule.ID = generatedTempUnschedulableRuleID(rule, idx)
		}
		normalized = append(normalized, rule)
	}
	return normalized
}

func generatedTempUnschedulableRuleID(rule TempUnschedulableRule, index int) string {
	h := sha1.New()
	_, _ = h.Write([]byte(strconv.Itoa(index)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.Itoa(rule.ErrorCode)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.Itoa(rule.DurationMinutes)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strings.Join(rule.Keywords, "\x00")))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(rule.Description))
	sum := h.Sum(nil)
	return "rule_" + hex.EncodeToString(sum[:6])
}

func normalizeProxyFailoverSettings(settings ProxyFailoverSettings) ProxyFailoverSettings {
	settings.PreferSameCountry = true
	if settings.ProbeIntervalMinutes < 1 {
		settings.ProbeIntervalMinutes = defaultProxyFailoverProbeIntervalMin
	}
	if settings.ProbeIntervalMinutes > 120 {
		settings.ProbeIntervalMinutes = 120
	}
	if settings.FailureThreshold < 1 {
		settings.FailureThreshold = defaultProxyFailoverFailureThreshold
	}
	if settings.FailureThreshold > 10 {
		settings.FailureThreshold = 10
	}
	if settings.FailureWindowMinutes < 1 {
		settings.FailureWindowMinutes = defaultProxyFailoverFailureWindowMin
	}
	if settings.FailureWindowMinutes > 120 {
		settings.FailureWindowMinutes = 120
	}
	if settings.CooldownMinutes < 1 {
		settings.CooldownMinutes = defaultProxyFailoverCooldownMin
	}
	if settings.CooldownMinutes > 240 {
		settings.CooldownMinutes = 240
	}
	if settings.HalfOpenProbeAccounts < 1 {
		settings.HalfOpenProbeAccounts = defaultProxyFailoverHalfOpenAccounts
	}
	if settings.HalfOpenProbeAccounts > 10 {
		settings.HalfOpenProbeAccounts = 10
	}
	if settings.CooldownBackoffFactor < 1 {
		settings.CooldownBackoffFactor = defaultProxyFailoverBackoffFactor
	}
	if settings.CooldownBackoffFactor > 4 {
		settings.CooldownBackoffFactor = 4
	}
	if settings.MaxCooldownMinutes < 1 {
		settings.MaxCooldownMinutes = defaultProxyFailoverMaxCooldownMin
	}
	if settings.MaxCooldownMinutes > 240 {
		settings.MaxCooldownMinutes = 240
	}
	if settings.MaxAccountsPerProxy < 1 {
		settings.MaxAccountsPerProxy = defaultProxyFailoverMaxPerProxy
	}
	if settings.MaxAccountsPerProxy > 100 {
		settings.MaxAccountsPerProxy = 100
	}
	if settings.MaxMigrationsPerCycle < 1 {
		settings.MaxMigrationsPerCycle = defaultProxyFailoverMaxMigrateBatch
	}
	if settings.MaxMigrationsPerCycle > 200 {
		settings.MaxMigrationsPerCycle = 200
	}
	if settings.TempUnschedMinutes < 1 {
		settings.TempUnschedMinutes = defaultProxyFailoverTempUnschedMin
	}
	if settings.TempUnschedMinutes > 240 {
		settings.TempUnschedMinutes = 240
	}
	return settings
}

func mechanismMatchesAccount(mechanism SchedulingMechanism, account *Account) bool {
	if !mechanismSelectableByAccount(mechanism, account) {
		return false
	}
	return !mechanism.Hidden
}

func mechanismSelectableByAccount(mechanism SchedulingMechanism, account *Account) bool {
	if account == nil {
		return false
	}
	if !mechanism.Enabled || !mechanism.TempUnschedulableEnabled {
		return false
	}
	if mechanism.Platform != SchedulingMechanismPlatformAll && mechanism.Platform != account.Platform {
		return false
	}
	if mechanism.AccountType != SchedulingMechanismAccountTypeAll && mechanism.AccountType != account.Type {
		return false
	}
	return len(mechanism.TempUnschedulableRules) > 0
}
