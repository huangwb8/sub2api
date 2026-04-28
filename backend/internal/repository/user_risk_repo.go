package repository

import (
	"context"
	"database/sql"
	"sort"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuserriskevent "github.com/Wei-Shaw/sub2api/ent/userriskevent"
	dbuserriskprofile "github.com/Wei-Shaw/sub2api/ent/userriskprofile"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userRiskRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewUserRiskRepository(client *dbent.Client, sqlDB *sql.DB) service.UserRiskRepository {
	return &userRiskRepository{client: client, sql: sqlDB}
}

func (r *userRiskRepository) GetByUserID(ctx context.Context, userID int64) (*service.UserRiskProfile, error) {
	profile, err := r.client.UserRiskProfile.Query().Where(dbuserriskprofile.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return userRiskProfileEntityToService(profile), nil
}

func (r *userRiskRepository) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*service.UserRiskProfile, error) {
	out := make(map[int64]*service.UserRiskProfile, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.client.UserRiskProfile.Query().Where(dbuserriskprofile.UserIDIn(userIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = userRiskProfileEntityToService(row)
	}
	return out, nil
}

func (r *userRiskRepository) ListUserIDsByStatuses(ctx context.Context, statuses []string) ([]int64, error) {
	q := r.client.UserRiskProfile.Query()
	if len(statuses) > 0 {
		q = q.Where(dbuserriskprofile.StatusIn(statuses...))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	userIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })
	return userIDs, nil
}

func (r *userRiskRepository) UpsertProfile(ctx context.Context, profile *service.UserRiskProfile) (*service.UserRiskProfile, error) {
	if profile == nil {
		return nil, nil
	}
	builder := r.client.UserRiskProfile.Create().
		SetUserID(profile.UserID).
		SetScore(profile.Score).
		SetStatus(profile.Status).
		SetConsecutiveBadDays(profile.ConsecutiveBadDays).
		SetLockReason(profile.LockReason).
		SetLastEvaluationSummary(profile.LastEvaluationSummary).
		SetExempted(profile.Exempted).
		SetExemptionReason(profile.ExemptionReason).
		SetUnlockReason(profile.UnlockReason).
		SetNillableLastEvaluatedAt(profile.LastEvaluatedAt).
		SetNillableLastWarnedAt(profile.LastWarnedAt).
		SetNillableGracePeriodStartedAt(profile.GracePeriodStartedAt).
		SetNillableLockedAt(profile.LockedAt).
		SetNillableExemptedAt(profile.ExemptedAt).
		SetNillableExemptedBy(profile.ExemptedBy).
		SetNillableUnlockedAt(profile.UnlockedAt).
		SetNillableUnlockedBy(profile.UnlockedBy)
	_, err := builder.
		OnConflictColumns(dbuserriskprofile.FieldUserID).
		UpdateNewValues().
		ID(ctx)
	if err != nil {
		return nil, err
	}
	return r.GetByUserID(ctx, profile.UserID)
}

func (r *userRiskRepository) AppendEvent(ctx context.Context, event *service.UserRiskEvent) (*service.UserRiskEvent, error) {
	if event == nil {
		return nil, nil
	}
	entity, err := r.client.UserRiskEvent.Create().
		SetUserID(event.UserID).
		SetEventType(event.EventType).
		SetSeverity(event.Severity).
		SetScoreDelta(event.ScoreDelta).
		SetScoreAfter(event.ScoreAfter).
		SetSummary(event.Summary).
		SetMetadata(event.Metadata).
		SetNillableWindowStart(event.WindowStart).
		SetNillableWindowEnd(event.WindowEnd).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return userRiskEventEntityToService(entity), nil
}

func (r *userRiskRepository) ListEventsByUserID(ctx context.Context, userID int64, limit int) ([]service.UserRiskEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.client.UserRiskEvent.Query().
		Where(dbuserriskevent.UserIDEQ(userID)).
		Order(dbent.Desc(dbuserriskevent.FieldCreatedAt), dbent.Desc(dbuserriskevent.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]service.UserRiskEvent, 0, len(rows))
	for _, row := range rows {
		item := userRiskEventEntityToService(row)
		if item != nil {
			out = append(out, *item)
		}
	}
	return out, nil
}

func userRiskProfileEntityToService(entity *dbent.UserRiskProfile) *service.UserRiskProfile {
	if entity == nil {
		return nil
	}
	return &service.UserRiskProfile{
		ID:                    entity.ID,
		UserID:                entity.UserID,
		Score:                 entity.Score,
		Status:                entity.Status,
		ConsecutiveBadDays:    entity.ConsecutiveBadDays,
		LastEvaluatedAt:       entity.LastEvaluatedAt,
		LastWarnedAt:          entity.LastWarnedAt,
		GracePeriodStartedAt:  entity.GracePeriodStartedAt,
		LockedAt:              entity.LockedAt,
		LockReason:            entity.LockReason,
		LastEvaluationSummary: entity.LastEvaluationSummary,
		Exempted:              entity.Exempted,
		ExemptedAt:            entity.ExemptedAt,
		ExemptedBy:            entity.ExemptedBy,
		ExemptionReason:       entity.ExemptionReason,
		UnlockedAt:            entity.UnlockedAt,
		UnlockedBy:            entity.UnlockedBy,
		UnlockReason:          entity.UnlockReason,
		CreatedAt:             entity.CreatedAt,
		UpdatedAt:             entity.UpdatedAt,
	}
}

func userRiskEventEntityToService(entity *dbent.UserRiskEvent) *service.UserRiskEvent {
	if entity == nil {
		return nil
	}
	return &service.UserRiskEvent{
		ID:          entity.ID,
		UserID:      entity.UserID,
		EventType:   entity.EventType,
		Severity:    entity.Severity,
		ScoreDelta:  entity.ScoreDelta,
		ScoreAfter:  entity.ScoreAfter,
		Summary:     entity.Summary,
		Metadata:    entity.Metadata,
		WindowStart: entity.WindowStart,
		WindowEnd:   entity.WindowEnd,
		CreatedAt:   entity.CreatedAt,
	}
}
