//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRefreshTokenCacheStorePersistsToDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC()
	data := &service.RefreshTokenData{
		UserID:       42,
		TokenVersion: 7,
		FamilyID:     "family-1",
		CreatedAt:    now,
		ExpiresAt:    now.Add(30 * 24 * time.Hour),
	}

	mock.ExpectExec("INSERT INTO refresh_tokens").
		WithArgs("hash-1", data.UserID, data.TokenVersion, data.FamilyID, data.CreatedAt, data.ExpiresAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	cache := NewRefreshTokenCache(nil, db)
	require.NoError(t, cache.StoreRefreshToken(context.Background(), "hash-1", data, time.Hour))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenCacheGetReadsFromDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour)
	rows := sqlmock.NewRows([]string{"user_id", "token_version", "family_id", "created_at", "expires_at"}).
		AddRow(int64(42), int64(7), "family-1", now, expiresAt)
	mock.ExpectQuery("SELECT (.+) FROM refresh_tokens").
		WithArgs("hash-1").
		WillReturnRows(rows)

	cache := NewRefreshTokenCache(nil, db)
	got, err := cache.GetRefreshToken(context.Background(), "hash-1")
	require.NoError(t, err)
	require.Equal(t, int64(42), got.UserID)
	require.Equal(t, int64(7), got.TokenVersion)
	require.Equal(t, "family-1", got.FamilyID)
	require.Equal(t, expiresAt, got.ExpiresAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenCacheConsumeDeletesAndReturnsFromDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour)
	rows := sqlmock.NewRows([]string{"user_id", "token_version", "family_id", "created_at", "expires_at"}).
		AddRow(int64(42), int64(7), "family-1", now, expiresAt)
	mock.ExpectQuery("DELETE FROM refresh_tokens").
		WithArgs("hash-1").
		WillReturnRows(rows)

	cache := NewRefreshTokenCache(nil, db)
	got, err := cache.ConsumeRefreshToken(context.Background(), "hash-1")
	require.NoError(t, err)
	require.Equal(t, int64(42), got.UserID)
	require.Equal(t, "family-1", got.FamilyID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshTokenCacheDBNotFoundUsesServiceError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery("SELECT (.+) FROM refresh_tokens").
		WithArgs("missing-hash").
		WillReturnError(sql.ErrNoRows)

	cache := NewRefreshTokenCache(nil, db)
	_, err = cache.GetRefreshToken(context.Background(), "missing-hash")
	require.ErrorIs(t, err, service.ErrRefreshTokenNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
