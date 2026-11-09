package server

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

// TestNewA2AServer_UnlinkedRedisProviderRefusesStart: QUEUE_PROVIDER=redis
// without the opt-in redis subpackage imported must fail startup naming the
// missing import instead of silently falling back to in-memory storage.
func TestNewA2AServer_UnlinkedRedisProviderRefusesStart(t *testing.T) {
	server := NewA2AServer(&serverConfig.Config{
		QueueConfig: serverConfig.QueueConfig{Provider: "redis", URL: "redis://localhost:6379"},
	}, zap.NewNop(), nil)

	require.Error(t, server.startupErr)
	assert.Contains(t, server.startupErr.Error(), "github.com/inference-gateway/adk/server/storage/redis")
	assert.Error(t, server.Start(context.Background()))
}

// TestNewA2AServer_UnknownProviderRefusesStart: an unregistered provider name
// is a misconfiguration; refuse to start instead of degrading to in-memory.
func TestNewA2AServer_UnknownProviderRefusesStart(t *testing.T) {
	server := NewA2AServer(&serverConfig.Config{
		QueueConfig: serverConfig.QueueConfig{Provider: "postgres"},
	}, zap.NewNop(), nil)

	require.Error(t, server.startupErr)
	assert.Contains(t, server.startupErr.Error(), "not registered")
}

// TestNewA2AServer_NoProviderFallsBackToMemory keeps the pre-existing behavior
// for an unconfigured queue.
func TestNewA2AServer_NoProviderFallsBackToMemory(t *testing.T) {
	server := NewA2AServer(&serverConfig.Config{}, zap.NewNop(), nil)

	assert.NoError(t, server.startupErr)
	_, ok := server.storage.(*InMemoryStorage)
	assert.True(t, ok)
}
