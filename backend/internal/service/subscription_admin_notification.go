package service

import (
	"context"
	"fmt"
	"html"
	"log"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
)

// SubscriptionNotificationEmailer abstracts the async email sink used by subscription notifications.
type SubscriptionNotificationEmailer interface {
	EnqueueCustomEmail(email, subject, body string) error
}

type subscriptionCapacityHint struct {
	ActiveSubscriptions int64
	TotalAccounts       int64
	ActiveAccounts      int64
	AdditionalAccounts  int64
	AccountTypeHint     string
	PreferenceScore     int
}

// SetAdminNotificationDeps wires optional dependencies for admin notification delivery.
func (s *SubscriptionService) SetAdminNotificationDeps(settingRepo SettingRepository, userRepo UserRepository, emailer SubscriptionNotificationEmailer) {
	if s == nil {
		return
	}
	s.settingRepo = settingRepo
	s.userRepo = userRepo
	s.notificationEmailer = emailer
}

func (s *SubscriptionService) notifyAdminsOfNewSubscription(ctx context.Context, sub *UserSubscription) {
	if s == nil || sub == nil || s.notificationEmailer == nil {
		return
	}
	notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.enqueueAdminSubscriptionCreatedEmails(notifyCtx, sub); err != nil {
		log.Printf("enqueue admin subscription notification failed: subscription_id=%d user_id=%d group_id=%d err=%v", sub.ID, sub.UserID, sub.GroupID, err)
	}
}

func (s *SubscriptionService) enqueueAdminSubscriptionCreatedEmails(ctx context.Context, sub *UserSubscription) error {
	recipients := s.resolveAdminSubscriptionRecipients(ctx)
	if len(recipients) == 0 {
		return nil
	}

	user := s.loadNotificationUser(ctx, sub)
	group := s.loadNotificationGroup(ctx, sub)
	siteName := s.loadSubscriptionNotificationSiteName(ctx)
	hint := s.estimateSubscriptionCapacityHint(ctx, sub, group)

	subject := buildSubscriptionAdminNotificationSubject(siteName, group)
	body := buildSubscriptionAdminNotificationBody(siteName, sub, user, group, hint)

	var firstErr error
	for _, recipient := range recipients {
		if err := s.notificationEmailer.EnqueueCustomEmail(recipient, subject, body); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *SubscriptionService) resolveAdminSubscriptionRecipients(ctx context.Context) []string {
	if s == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if s.settingRepo == nil {
		return nil
	}

	email, err := s.settingRepo.GetValue(ctx, SettingKeySubscriptionNotificationEmail)
	if err != nil {
		return nil
	}
	return normalizeEmails([]string{email})
}

func (s *SubscriptionService) loadSubscriptionNotificationSiteName(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return "Sub2API"
	}
	siteName, err := s.settingRepo.GetValue(ctx, SettingKeySiteName)
	if err != nil || strings.TrimSpace(siteName) == "" {
		return "Sub2API"
	}
	return strings.TrimSpace(siteName)
}

func (s *SubscriptionService) loadNotificationUser(ctx context.Context, sub *UserSubscription) *User {
	if sub == nil {
		return nil
	}
	if sub.User != nil {
		return sub.User
	}
	if s == nil || s.userRepo == nil {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, sub.UserID)
	if err != nil {
		return nil
	}
	return user
}

func (s *SubscriptionService) loadNotificationGroup(ctx context.Context, sub *UserSubscription) *Group {
	if sub == nil {
		return nil
	}
	if s != nil && s.groupRepo != nil {
		group, err := s.groupRepo.GetByID(ctx, sub.GroupID)
		if err == nil && group != nil {
			return group
		}
	}
	return sub.Group
}

func (s *SubscriptionService) estimateSubscriptionCapacityHint(ctx context.Context, sub *UserSubscription, group *Group) subscriptionCapacityHint {
	hint := subscriptionCapacityHint{
		ActiveSubscriptions: 1,
		AccountTypeHint:     subscriptionAccountTypeHint(group),
		PreferenceScore:     defaultSubscriptionCapacityTightness,
	}
	if s != nil && s.settingRepo != nil {
		if raw, err := s.settingRepo.GetValue(ctx, SettingKeySubscriptionCapacityTightness); err == nil {
			hint.PreferenceScore = parseSubscriptionCapacityTightness(raw)
		}
	}
	if group != nil {
		hint.TotalAccounts = group.AccountCount
		hint.ActiveAccounts = group.ActiveAccountCount
		hint.AccountTypeHint = subscriptionAccountTypeHint(group)
	}
	if s != nil && s.entClient != nil && sub != nil {
		now := time.Now().UTC()
		activeSubscriptions, err := s.entClient.UserSubscription.Query().
			Where(
				usersubscription.GroupIDEQ(sub.GroupID),
				usersubscription.StatusEQ(SubscriptionStatusActive),
				usersubscription.ExpiresAtGT(now),
			).
			Count(ctx)
		if err == nil && activeSubscriptions > 0 {
			hint.ActiveSubscriptions = int64(activeSubscriptions)
		}
	}
	profile := buildCapacityRecommendationPreferenceProfile(hint.PreferenceScore)
	hint.AdditionalAccounts = estimateAdditionalAccounts(
		hint.ActiveSubscriptions,
		hint.ActiveAccounts,
		profile,
	)
	return hint
}

func estimateAdditionalAccounts(
	activeSubscriptions, activeAccounts int64,
	profile capacityRecommendationPreferenceProfile,
) int64 {
	if activeSubscriptions <= 0 {
		return 0
	}
	if activeAccounts <= 0 {
		return 1
	}

	beforeNewSubscriptionCount := activeSubscriptions - 1
	targetSubscriptionsPerAccount := int64(1)
	if beforeNewSubscriptionCount > 0 {
		targetSubscriptionsPerAccount = ceilDivInt64(beforeNewSubscriptionCount, activeAccounts)
		if targetSubscriptionsPerAccount <= 0 {
			targetSubscriptionsPerAccount = 1
		}
	}
	adjustedTarget := int64(math.Ceil(float64(targetSubscriptionsPerAccount) * profile.EmailTargetMultiplier))
	if adjustedTarget > targetSubscriptionsPerAccount+1 {
		adjustedTarget = targetSubscriptionsPerAccount + 1
	}
	if adjustedTarget <= 0 {
		adjustedTarget = 1
	}

	requiredAccounts := ceilDivInt64(activeSubscriptions, adjustedTarget)
	if requiredAccounts <= activeAccounts {
		return 0
	}
	return requiredAccounts - activeAccounts
}

func ceilDivInt64(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	return (numerator + denominator - 1) / denominator
}

func subscriptionAccountTypeHint(group *Group) string {
	if group == nil {
		return "同平台可调度账号"
	}

	if group.RequireOAuthOnly {
		switch group.Platform {
		case PlatformAnthropic:
			return "Anthropic OAuth / Setup Token"
		case PlatformOpenAI:
			return "OpenAI OAuth"
		case PlatformGemini:
			return "Gemini OAuth"
		case PlatformAntigravity:
			return "Antigravity OAuth"
		default:
			return "OAuth"
		}
	}

	switch group.Platform {
	case PlatformAnthropic:
		return "Anthropic OAuth / Setup Token 或 API Key"
	case PlatformOpenAI:
		return "OpenAI API Key / OAuth"
	case PlatformGemini:
		return "Gemini API Key / OAuth"
	case PlatformAntigravity:
		return "Antigravity OAuth"
	default:
		return "同平台可调度账号"
	}
}

func buildSubscriptionAdminNotificationSubject(siteName string, group *Group) string {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		siteName = "Sub2API"
	}
	groupName := "未知分组"
	if group != nil && strings.TrimSpace(group.Name) != "" {
		groupName = strings.TrimSpace(group.Name)
	}
	return fmt.Sprintf("[%s] 新增用户订阅通知: %s", siteName, groupName)
}

func buildSubscriptionAdminNotificationBody(siteName string, sub *UserSubscription, user *User, group *Group, hint subscriptionCapacityHint) string {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		siteName = "Sub2API"
	}

	userEmail := "-"
	userName := "-"
	if user != nil {
		if strings.TrimSpace(user.Email) != "" {
			userEmail = strings.TrimSpace(user.Email)
		}
		if strings.TrimSpace(user.Username) != "" {
			userName = strings.TrimSpace(user.Username)
		}
	}

	groupName := "-"
	groupPlatform := "-"
	if group != nil {
		if strings.TrimSpace(group.Name) != "" {
			groupName = strings.TrimSpace(group.Name)
		}
		if strings.TrimSpace(group.Platform) != "" {
			groupPlatform = strings.TrimSpace(group.Platform)
		}
	}

	startsAt := "-"
	expiresAt := "-"
	status := "-"
	source := subscriptionCreationSource(sub)
	if sub != nil {
		if !sub.StartsAt.IsZero() {
			startsAt = sub.StartsAt.Format("2006-01-02 15:04:05 MST")
		}
		if !sub.ExpiresAt.IsZero() {
			expiresAt = sub.ExpiresAt.Format("2006-01-02 15:04:05 MST")
		}
		if strings.TrimSpace(sub.Status) != "" {
			status = strings.TrimSpace(sub.Status)
		}
	}

	suggestion := fmt.Sprintf("当前暂无必须立即补充的账号，建议继续关注该分组负载。优先关注账号类型：%s。", hint.AccountTypeHint)
	if hint.AdditionalAccounts > 0 {
		suggestion = fmt.Sprintf("建议补充 %d 个 %s 账号。", hint.AdditionalAccounts, hint.AccountTypeHint)
	}

	return fmt.Sprintf(`
<h2>%s 新增用户订阅通知</h2>
<p>系统刚刚新增了一条用户订阅，请管理员评估是否需要及时补充上游账号容量。</p>
<p><b>用户邮箱</b>: %s</p>
<p><b>用户名</b>: %s</p>
<p><b>订阅分组</b>: %s</p>
<p><b>平台</b>: %s</p>
<p><b>来源</b>: %s</p>
<p><b>状态</b>: %s</p>
<p><b>开始时间</b>: %s</p>
<p><b>到期时间</b>: %s</p>
<p><b>额度紧张度</b>: %d / 100</p>
<p><b>补号建议</b>: %s</p>
<p><b>推导依据</b>: 当前活跃订阅 %d 个，可调度账号 %d 个，分组总账号 %d 个。</p>
`,
		escapeSubscriptionHTML(siteName),
		escapeSubscriptionHTML(userEmail),
		escapeSubscriptionHTML(userName),
		escapeSubscriptionHTML(groupName),
		escapeSubscriptionHTML(groupPlatform),
		escapeSubscriptionHTML(source),
		escapeSubscriptionHTML(status),
		escapeSubscriptionHTML(startsAt),
		escapeSubscriptionHTML(expiresAt),
		hint.PreferenceScore,
		escapeSubscriptionHTML(suggestion),
		hint.ActiveSubscriptions,
		hint.ActiveAccounts,
		hint.TotalAccounts,
	)
}

func subscriptionCreationSource(sub *UserSubscription) string {
	if sub == nil {
		return "未知来源"
	}
	if sub.AssignedBy != nil && *sub.AssignedBy > 0 {
		return "管理员手动分配"
	}

	notes := strings.ToLower(strings.TrimSpace(sub.Notes))
	switch {
	case strings.Contains(notes, "payment order"):
		return "用户支付开通"
	case strings.Contains(notes, "auto assigned by default user subscriptions setting"):
		return "默认订阅自动分配"
	default:
		return "系统自动新增"
	}
}

func escapeSubscriptionHTML(value string) string {
	return html.EscapeString(strings.TrimSpace(value))
}
