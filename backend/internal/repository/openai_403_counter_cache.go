package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const openAI403CounterPrefix = "openai_403_count:account:"

type openAI403CounterCache struct {
	rdb *redis.Client
}

// NewOpenAI403CounterCache 创建 OpenAI 403 连续失败计数缓存。
func NewOpenAI403CounterCache(rdb *redis.Client) service.OpenAI403CounterCache {
	return &openAI403CounterCache{rdb: rdb}
}

func (c *openAI403CounterCache) IncrementOpenAI403Count(ctx context.Context, accountID int64, windowMinutes int) (int64, error) {
	key := fmt.Sprintf("%s%d", openAI403CounterPrefix, accountID)
	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60
	}
	count, err := timeoutCounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment openai 403 count: %w", err)
	}
	return count, nil
}

func (c *openAI403CounterCache) ResetOpenAI403Count(ctx context.Context, accountID int64) error {
	key := fmt.Sprintf("%s%d", openAI403CounterPrefix, accountID)
	return c.rdb.Del(ctx, key).Err()
}
