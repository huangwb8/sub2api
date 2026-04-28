package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *UserRiskService) sendWarningEmail(ctx context.Context, user *User, profile *UserRiskProfile, config *UserRiskControlConfig, reasons []string) error {
	if s == nil || s.emailQueue == nil || user == nil || strings.TrimSpace(user.Email) == "" || config == nil || !config.WarningEmailEnabled {
		return nil
	}
	subject := strings.TrimSpace(config.WarningEmailSubjectTemplate)
	if subject == "" {
		subject = DefaultUserRiskControlConfig().WarningEmailSubjectTemplate
	}
	body := buildUserRiskWarningEmailBody(user, profile, config, reasons)
	return s.emailQueue.EnqueueCustomEmail(user.Email, subject, body)
}

func buildUserRiskWarningEmailBody(user *User, profile *UserRiskProfile, config *UserRiskControlConfig, reasons []string) string {
	score := 5.0
	status := UserRiskStatusWarned
	if profile != nil {
		score = profile.Score
		status = profile.Status
	}
	if config == nil {
		config = DefaultUserRiskControlConfig()
	}

	reasonLines := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		reasonLines = append(reasonLines, fmt.Sprintf("- %s", reason))
	}
	if len(reasonLines) == 0 {
		reasonLines = append(reasonLines, "- Suspicious concurrent or multi-location usage was detected.")
	}

	return strings.TrimSpace(fmt.Sprintf(`
Hello %s,

We detected account activity that looks inconsistent with normal single-user subscription usage.

Current score: %.1f / 5.0
Current status: %s
Warning threshold: %.1f
Lock threshold: %.1f

Main evidence:
%s

Suggested actions:
- Stop sharing or reselling your API key.
- Stop using the same subscription from multiple public IP locations at the same time.
- Rotate the API key immediately if you suspect it has leaked.
- If this is a legitimate team or enterprise scenario, contact the administrator for exemption or a suitable plan.

If the behavior does not improve, the account may be locked automatically after %d consecutive bad daily evaluations.

Time: %s
`,
		displayUserRiskIdentity(user),
		score,
		status,
		config.WarningThreshold,
		config.LockThreshold,
		strings.Join(reasonLines, "\n"),
		config.AutoLockAfterConsecutiveBadDays,
		time.Now().Format(time.RFC3339),
	))
}

func displayUserRiskIdentity(user *User) string {
	if user == nil {
		return "user"
	}
	if strings.TrimSpace(user.Username) != "" {
		return user.Username
	}
	if strings.TrimSpace(user.Email) != "" {
		return user.Email
	}
	return "user"
}
