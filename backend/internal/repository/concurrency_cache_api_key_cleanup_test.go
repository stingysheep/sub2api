package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCleanupStaleProcessSlotsCleansAndIndexesAPIKeySlots(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	cache := NewConcurrencyCache(client, 30, 1800).(*concurrencyCache)
	apiKeyID := int64(903)
	key := apiKeySlotKey(apiKeyID)
	now, err := cache.redisUnixSeconds(ctx)
	require.NoError(t, err)
	require.NoError(t, client.ZAdd(ctx, key,
		redis.Z{Score: float64(now), Member: "oldproc-1"},
		redis.Z{Score: float64(now), Member: "keep-1"},
	).Err())

	require.NoError(t, cache.CleanupStaleProcessSlots(ctx, "keep-"))
	members, err := client.ZRange(ctx, key, 0, -1).Result()
	require.NoError(t, err)
	require.Equal(t, []string{"keep-1"}, members)
	_, err = client.ZScore(ctx, apiKeyActiveIndexKey, strconv.FormatInt(apiKeyID, 10)).Result()
	require.NoError(t, err)

	require.NoError(t, cache.ReleaseAPIKeySlot(ctx, apiKeyID, "keep-1"))
	_, err = client.ZScore(ctx, apiKeyActiveIndexKey, strconv.FormatInt(apiKeyID, 10)).Result()
	require.ErrorIs(t, err, redis.Nil)
}
