package service

import "time"

const (
	TemporaryInvitationQualifyingAmountCNY = 30.0
	TemporaryInvitationSignupWindow        = 24 * time.Hour
	TemporaryInvitationDeleteWindow        = 7 * 24 * time.Hour
)

func IsInvitationRedeemType(codeType string) bool {
	return codeType == RedeemTypeInvitation || codeType == RedeemTypeInvitationTemp
}

func IsTemporaryInvitationRedeemType(codeType string) bool {
	return codeType == RedeemTypeInvitationTemp
}

func TemporaryInvitationQualified(totalAmountCNY float64) bool {
	return totalAmountCNY > TemporaryInvitationQualifyingAmountCNY
}

func ApplyTemporaryInvitationWindow(user *User, now time.Time) {
	if user == nil {
		return
	}
	deadline := now.Add(TemporaryInvitationSignupWindow)
	user.TemporaryInvitation = true
	user.TemporaryInvitationDeadlineAt = &deadline
	user.TemporaryInvitationDisabledAt = nil
	user.TemporaryInvitationDeleteAt = nil
}

func ClearTemporaryInvitationState(user *User) {
	if user == nil {
		return
	}
	user.TemporaryInvitation = false
	user.TemporaryInvitationDeadlineAt = nil
	user.TemporaryInvitationDisabledAt = nil
	user.TemporaryInvitationDeleteAt = nil
}

func MarkTemporaryInvitationDisabled(user *User, now time.Time) {
	if user == nil {
		return
	}
	deleteAt := now.Add(TemporaryInvitationDeleteWindow)
	user.Status = StatusDisabled
	user.TemporaryInvitation = true
	user.TemporaryInvitationDisabledAt = &now
	user.TemporaryInvitationDeleteAt = &deleteAt
}
