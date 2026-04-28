package service

import "testing"

func TestNormalizeUserRiskControlConfigAppliesDefaults(t *testing.T) {
	t.Parallel()

	config := NormalizeUserRiskControlConfig(&UserRiskControlConfig{
		Mode:                        "  ",
		WarningThreshold:            0,
		LockThreshold:               0,
		OverlapWindowSeconds:        0,
		MaxDistinctPublicIPsPerDay:  0,
		HighRiskActiveHoursPerDay:   0,
		WarningEmailSubjectTemplate: " ",
		LockMessage:                 " ",
		DailyScorePenaltyCap:        0,
		DailyScoreRecovery:          0,
		RedisEvidenceRetentionDays:  0,
	})

	defaults := DefaultUserRiskControlConfig()
	if config.Mode != defaults.Mode {
		t.Fatalf("Mode = %q, want %q", config.Mode, defaults.Mode)
	}
	if config.WarningThreshold != defaults.WarningThreshold {
		t.Fatalf("WarningThreshold = %v, want %v", config.WarningThreshold, defaults.WarningThreshold)
	}
	if config.LockThreshold != defaults.LockThreshold {
		t.Fatalf("LockThreshold = %v, want %v", config.LockThreshold, defaults.LockThreshold)
	}
	if config.WarningEmailSubjectTemplate != defaults.WarningEmailSubjectTemplate {
		t.Fatalf("WarningEmailSubjectTemplate = %q, want %q", config.WarningEmailSubjectTemplate, defaults.WarningEmailSubjectTemplate)
	}
	if config.LockMessage != defaults.LockMessage {
		t.Fatalf("LockMessage = %q, want %q", config.LockMessage, defaults.LockMessage)
	}
	if config.RedisEvidenceRetentionDays != defaults.RedisEvidenceRetentionDays {
		t.Fatalf("RedisEvidenceRetentionDays = %d, want %d", config.RedisEvidenceRetentionDays, defaults.RedisEvidenceRetentionDays)
	}
}

func TestValidateUserRiskControlConfig(t *testing.T) {
	t.Parallel()

	valid := DefaultUserRiskControlConfig()
	valid.Mode = UserRiskControlModeAutoLock
	if err := ValidateUserRiskControlConfig(valid); err != nil {
		t.Fatalf("ValidateUserRiskControlConfig(valid) returned error: %v", err)
	}

	invalidOrder := DefaultUserRiskControlConfig()
	invalidOrder.WarningThreshold = 2
	invalidOrder.LockThreshold = 3
	if err := ValidateUserRiskControlConfig(invalidOrder); err == nil {
		t.Fatal("expected threshold order validation error, got nil")
	}

	invalidMode := DefaultUserRiskControlConfig()
	invalidMode.Mode = "panic"
	if err := ValidateUserRiskControlConfig(invalidMode); err == nil {
		t.Fatal("expected invalid mode validation error, got nil")
	}
}
