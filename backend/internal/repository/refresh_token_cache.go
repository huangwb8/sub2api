package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenKeyPrefix   = "refresh_token:"
	userRefreshTokensPrefix = "user_refresh_tokens:"
	tokenFamilyPrefix       = "token_family:"
)

// refreshTokenKey generates the Redis key for a refresh token.
func refreshTokenKey(tokenHash string) string {
	return refreshTokenKeyPrefix + tokenHash
}

// userRefreshTokensKey generates the Redis key for user's token set.
func userRefreshTokensKey(userID int64) string {
	return fmt.Sprintf("%s%d", userRefreshTokensPrefix, userID)
}

// tokenFamilyKey generates the Redis key for token family set.
func tokenFamilyKey(familyID string) string {
	return tokenFamilyPrefix + familyID
}

type refreshTokenCache struct {
	rdb *redis.Client
	db  *sql.DB
}

// NewRefreshTokenCache creates a new RefreshTokenCache implementation.
func NewRefreshTokenCache(rdb *redis.Client, db *sql.DB) service.RefreshTokenCache {
	return &refreshTokenCache{rdb: rdb, db: db}
}

func (c *refreshTokenCache) StoreRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	if c.db != nil {
		if err := c.storeRefreshTokenDB(ctx, tokenHash, data); err != nil {
			return err
		}
		if c.rdb == nil {
			return nil
		}
		_ = c.storeRefreshTokenRedis(ctx, tokenHash, data, ttl)
		return nil
	}
	return c.storeRefreshTokenRedis(ctx, tokenHash, data, ttl)
}

func (c *refreshTokenCache) storeRefreshTokenRedis(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	if c.rdb == nil {
		return fmt.Errorf("refresh token redis client not configured")
	}
	key := refreshTokenKey(tokenHash)
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal refresh token data: %w", err)
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *refreshTokenCache) storeRefreshTokenDB(ctx context.Context, tokenHash string, data *service.RefreshTokenData) error {
	if data == nil {
		return fmt.Errorf("refresh token data is nil")
	}
	_, err := c.db.ExecContext(ctx, `
INSERT INTO refresh_tokens (token_hash, user_id, token_version, family_id, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (token_hash) DO UPDATE SET
	user_id = EXCLUDED.user_id,
	token_version = EXCLUDED.token_version,
	family_id = EXCLUDED.family_id,
	created_at = EXCLUDED.created_at,
	expires_at = EXCLUDED.expires_at
`, tokenHash, data.UserID, data.TokenVersion, data.FamilyID, data.CreatedAt, data.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store refresh token db: %w", err)
	}
	return nil
}

func (c *refreshTokenCache) GetRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	if c.db != nil {
		data, err := c.getRefreshTokenDB(ctx, tokenHash)
		if err != nil {
			return nil, err
		}
		if c.rdb != nil {
			ttl := time.Until(data.ExpiresAt)
			if ttl > 0 {
				_ = c.storeRefreshTokenRedis(ctx, tokenHash, data, ttl)
			}
		}
		return data, nil
	}
	return c.getRefreshTokenRedis(ctx, tokenHash)
}

func (c *refreshTokenCache) getRefreshTokenRedis(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	if c.rdb == nil {
		return nil, service.ErrRefreshTokenNotFound
	}
	key := refreshTokenKey(tokenHash)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	var data service.RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token data: %w", err)
	}
	return &data, nil
}

func (c *refreshTokenCache) getRefreshTokenDB(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	row := c.db.QueryRowContext(ctx, `
SELECT user_id, token_version, family_id, created_at, expires_at
FROM refresh_tokens
WHERE token_hash = $1
`, tokenHash)
	data, err := scanRefreshTokenData(row)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *refreshTokenCache) ConsumeRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	if c.db != nil {
		row := c.db.QueryRowContext(ctx, `
DELETE FROM refresh_tokens
WHERE token_hash = $1
RETURNING user_id, token_version, family_id, created_at, expires_at
`, tokenHash)
		data, err := scanRefreshTokenData(row)
		if err != nil {
			return nil, err
		}
		if c.rdb != nil {
			_ = c.rdb.Del(ctx, refreshTokenKey(tokenHash)).Err()
		}
		return data, nil
	}

	data, err := c.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if err := c.DeleteRefreshToken(ctx, tokenHash); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *refreshTokenCache) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	var dbErr error
	if c.db != nil {
		_, dbErr = c.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)
	}
	if c.rdb == nil {
		return dbErr
	}
	key := refreshTokenKey(tokenHash)
	redisErr := c.rdb.Del(ctx, key).Err()
	if dbErr != nil {
		return dbErr
	}
	if c.db == nil {
		return redisErr
	}
	return nil
}

func (c *refreshTokenCache) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	// Get all token hashes for this user
	tokenHashes, err := c.GetUserTokenHashes(ctx, userID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get user token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		if c.db != nil {
			_, err := c.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
			return err
		}
		return nil
	}

	var dbErr error
	if c.db != nil {
		_, dbErr = c.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
	}
	if c.rdb == nil {
		return dbErr
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
	}
	keys = append(keys, userRefreshTokensKey(userID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	if dbErr != nil {
		return dbErr
	}
	if c.db == nil {
		return err
	}
	return nil
}

func (c *refreshTokenCache) DeleteTokenFamily(ctx context.Context, familyID string) error {
	// Get all token hashes in this family
	tokenHashes, err := c.GetFamilyTokenHashes(ctx, familyID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get family token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		if c.db != nil {
			_, err := c.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE family_id = $1`, familyID)
			return err
		}
		return nil
	}

	var dbErr error
	if c.db != nil {
		_, dbErr = c.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE family_id = $1`, familyID)
	}
	if c.rdb == nil {
		return dbErr
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
	}
	keys = append(keys, tokenFamilyKey(familyID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	if dbErr != nil {
		return dbErr
	}
	if c.db == nil {
		return err
	}
	return nil
}

func (c *refreshTokenCache) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	if c.rdb == nil {
		return nil
	}
	key := userRefreshTokensKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	if c.rdb == nil {
		return nil
	}
	key := tokenFamilyKey(familyID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	if c.db != nil {
		return c.getTokenHashesDB(ctx, `SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, userID)
	}
	if c.rdb == nil {
		return nil, nil
	}
	key := userRefreshTokensKey(userID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	if c.db != nil {
		return c.getTokenHashesDB(ctx, `SELECT token_hash FROM refresh_tokens WHERE family_id = $1`, familyID)
	}
	if c.rdb == nil {
		return nil, nil
	}
	key := tokenFamilyKey(familyID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	if c.db != nil {
		var exists bool
		err := c.db.QueryRowContext(ctx, `
SELECT EXISTS (
	SELECT 1 FROM refresh_tokens WHERE family_id = $1 AND token_hash = $2
)
`, familyID, tokenHash).Scan(&exists)
		return exists, err
	}
	if c.rdb == nil {
		return false, nil
	}
	key := tokenFamilyKey(familyID)
	return c.rdb.SIsMember(ctx, key, tokenHash).Result()
}

type refreshTokenScanner interface {
	Scan(dest ...any) error
}

func scanRefreshTokenData(row refreshTokenScanner) (*service.RefreshTokenData, error) {
	var data service.RefreshTokenData
	if err := row.Scan(&data.UserID, &data.TokenVersion, &data.FamilyID, &data.CreatedAt, &data.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	return &data, nil
}

func (c *refreshTokenCache) getTokenHashesDB(ctx context.Context, query string, arg any) ([]string, error) {
	rows, err := c.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		hashes = append(hashes, hash)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hashes, nil
}
