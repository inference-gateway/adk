package server

import (
	"context"
	"testing"
	"time"

	"github.com/inference-gateway/adk/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestStorageFactoryRegistry(t *testing.T) {
	registry := &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	mockFactory := &MockStorageFactory{
		provider: "test",
	}

	registry.Register("test", mockFactory)

	factory, err := registry.GetFactory("test")
	require.NoError(t, err)
	assert.Equal(t, mockFactory, factory)

	_, err = registry.GetFactory("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported storage provider")

	providers := registry.GetProviders()
	assert.Contains(t, providers, "test")
}

func TestStorageFactoryRegistryPanicsOnMismatch(t *testing.T) {
	registry := &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	mockFactory := &MockStorageFactory{
		provider: "actual",
	}

	assert.Panics(t, func() {
		registry.Register("expected", mockFactory)
	})
}

func TestCreateStorageWithValidation(t *testing.T) {
	registry := &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	mockFactory := &MockStorageFactory{
		provider:   "test",
		shouldFail: false,
	}
	registry.Register("test", mockFactory)

	logger := zaptest.NewLogger(t)
	cfg := config.QueueConfig{
		Provider: "test",
	}

	storage, err := registry.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)
}

func TestCreateStorageWithValidationFailure(t *testing.T) {
	registry := &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	mockFactory := &MockStorageFactory{
		provider:        "test",
		shouldFail:      true,
		validationError: "validation failed",
	}
	registry.Register("test", mockFactory)

	logger := zaptest.NewLogger(t)
	cfg := config.QueueConfig{
		Provider: "test",
	}

	_, err := registry.CreateStorage(context.Background(), cfg, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestInMemoryStorageFactory(t *testing.T) {
	factory := &InMemoryStorageFactory{}

	assert.Equal(t, "memory", factory.SupportedProvider())

	cfg := config.QueueConfig{
		Provider: "memory",
	}
	err := factory.ValidateConfig(cfg)
	assert.NoError(t, err)

	logger := zaptest.NewLogger(t)
	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	_, ok := storage.(*InMemoryStorage)
	assert.True(t, ok)
}

func TestInMemoryStorageFactoryWithCustomHistory(t *testing.T) {
	factory := &InMemoryStorageFactory{}
	logger := zaptest.NewLogger(t)

	cfg := config.QueueConfig{
		Provider: "memory",
		Options: map[string]string{
			"max_conversation_history": "50",
		},
	}

	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)
}

func TestGlobalRegistryFunctions(t *testing.T) {
	globalRegistry = &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	RegisterStorageProvider("memory", &InMemoryStorageFactory{})

	providers := GetSupportedProviders()
	assert.Contains(t, providers, "memory")

	factory, err := GetStorageProvider("memory")
	require.NoError(t, err)
	assert.NotNil(t, factory)

	logger := zaptest.NewLogger(t)
	cfg := config.QueueConfig{
		Provider: "memory",
	}

	storage, err := CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)
}

type MockStorageFactory struct {
	provider        string
	shouldFail      bool
	validationError string
}

func (m *MockStorageFactory) SupportedProvider() string {
	return m.provider
}

func (m *MockStorageFactory) ValidateConfig(config.QueueConfig) error {
	if m.shouldFail {
		return assert.AnError
	}
	if m.validationError != "" {
		return assert.AnError
	}
	return nil
}

func (m *MockStorageFactory) CreateStorage(ctx context.Context, config config.QueueConfig, logger *zap.Logger) (Storage, error) {
	if m.shouldFail {
		return nil, assert.AnError
	}

	return NewInMemoryStorage(logger, 20), nil
}

func TestConfigQueueConfigExtensions(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected config.QueueConfig
	}{
		{
			name:    "default memory provider",
			envVars: map[string]string{
				// No QUEUE_* env vars set
			},
			expected: config.QueueConfig{
				Provider:        "memory",
				MaxSize:         100,
				CleanupInterval: 120 * time.Second,
			},
		},
		{
			name: "redis provider with basic config",
			envVars: map[string]string{
				"QUEUE_PROVIDER": "redis",
				"QUEUE_URL":      "redis://localhost:6379",
			},
			expected: config.QueueConfig{
				Provider:        "redis",
				URL:             "redis://localhost:6379",
				MaxSize:         100,
				CleanupInterval: 120 * time.Second,
			},
		},
		{
			name: "redis with custom options",
			envVars: map[string]string{
				"QUEUE_PROVIDER": "redis",
				"QUEUE_URL":      "redis://localhost:6379",
				"QUEUE_MAX_SIZE": "200",
			},
			expected: config.QueueConfig{
				Provider:        "redis",
				URL:             "redis://localhost:6379",
				MaxSize:         200,
				CleanupInterval: 120 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookuper := &testLookuper{envVars: tt.envVars}

			baseConfig := &config.Config{}

			cfg, err := config.LoadWithLookuper(context.Background(), baseConfig, lookuper)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.Provider, cfg.QueueConfig.Provider)
			assert.Equal(t, tt.expected.URL, cfg.QueueConfig.URL)
			assert.Equal(t, tt.expected.MaxSize, cfg.QueueConfig.MaxSize)
			assert.Equal(t, tt.expected.CleanupInterval, cfg.QueueConfig.CleanupInterval)

			if tt.expected.Options != nil {
				for key, expectedValue := range tt.expected.Options {
					actualValue, exists := cfg.QueueConfig.Options[key]
					assert.True(t, exists, "Option %s should exist", key)
					assert.Equal(t, expectedValue, actualValue, "Option %s should match", key)
				}
			}
		})
	}
}

type testLookuper struct {
	envVars map[string]string
}

func (t *testLookuper) Lookup(key string) (string, bool) {
	value, exists := t.envVars[key]
	return value, exists
}
