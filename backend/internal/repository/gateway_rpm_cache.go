package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

var gatewayRPMIncrScript = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
local ttl = redis.call('PTTL', KEYS[1])
if current == 1 or ttl == -1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`)

type gatewayRPMCache struct {
	rdb *redis.Client
}

func NewGatewayRPMCache(rdb *redis.Client) service.GatewayRPMCache {
	return &gatewayRPMCache{rdb: rdb}
}

func (c *gatewayRPMCache) Increment(ctx context.Context, key string, window time.Duration) (int64, error) {
	ttlMillis := window.Milliseconds()
	if ttlMillis < 1 {
		ttlMillis = int64(time.Minute / time.Millisecond)
	}
	count, err := gatewayRPMIncrScript.Run(ctx, c.rdb, []string{key}, ttlMillis).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment gateway rpm: %w", err)
	}
	return count, nil
}
