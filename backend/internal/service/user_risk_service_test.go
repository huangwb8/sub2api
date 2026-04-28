package service

import (
	"strings"
	"testing"
)

func TestScoreUserRiskDayCapsPenaltyAndEmitsReasons(t *testing.T) {
	t.Parallel()

	reasons, penalty := scoreUserRiskDay(DefaultUserRiskControlConfig(), &UserRiskSignalSnapshot{
		OverlapEvents:      2,
		DistinctPublicIPs:  6,
		DistinctUserAgents: 4,
	}, &UserRiskUsageSummary{
		DistinctAPIKeys: 3,
		ActiveHours:     20,
	})

	if penalty != 1.0 {
		t.Fatalf("penalty = %v, want 1.0", penalty)
	}
	if len(reasons) == 0 {
		t.Fatal("expected non-empty reasons")
	}
	if !strings.Contains(strings.Join(reasons, " "), "multi-IP overlap") {
		t.Fatalf("expected overlap reason, got %v", reasons)
	}
}

func TestShouldAutoLockUserRiskProfileRequiresModeThresholdAndEvidence(t *testing.T) {
	t.Parallel()

	config := DefaultUserRiskControlConfig()
	config.Mode = UserRiskControlModeAutoLock
	config.RequireTrustedProxyForAutoLock = true

	profile := &UserRiskProfile{
		Score:              1.5,
		ConsecutiveBadDays: 3,
	}

	if shouldAutoLockUserRiskProfile(config, profile, nil) {
		t.Fatal("expected auto lock to require trusted evidence")
	}

	if !shouldAutoLockUserRiskProfile(config, profile, &UserRiskSignalSnapshot{DistinctPublicIPs: 2}) {
		t.Fatal("expected auto lock when evidence is present")
	}

	config.Mode = UserRiskControlModeWarnOnly
	if shouldAutoLockUserRiskProfile(config, profile, &UserRiskSignalSnapshot{DistinctPublicIPs: 2}) {
		t.Fatal("did not expect auto lock outside auto_lock mode")
	}
}
