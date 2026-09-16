package server

import (
	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

// NewRedisStorageForTest constructs a RedisStorage backed by the supplied
// RedisClient. Only compiled into the test binary, so this does not widen
// the production API.
func NewRedisStorageForTest(client RedisClient, logger *zap.Logger, cfg serverConfig.QueueConfig) *RedisStorage {
	return &RedisStorage{
		client: client,
		logger: logger,
		config: cfg,
	}
}
