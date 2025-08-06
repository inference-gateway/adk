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
	// Create a test registry
	registry := &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	// Create a mock factory
	mockFactory := &MockStorageFactory{
		provider: "test",
	}

	// Test registration
	registry.Register("test", mockFactory)
	
	// Test retrieval
	factory, err := registry.GetFactory("test")
	require.NoError(t, err)
	assert.Equal(t, mockFactory, factory)

	// Test non-existent provider
	_, err = registry.GetFactory("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported storage provider")

	// Test getting providers list
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

	// This should panic because provider names don't match
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
		provider:       "test",
		shouldFail:     true,
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

	// Test provider name
	assert.Equal(t, "memory", factory.SupportedProvider())

	// Test validation (should always pass)
	cfg := config.QueueConfig{
		Provider: "memory",
		// No URL or credentials needed
	}
	err := factory.ValidateConfig(cfg)
	assert.NoError(t, err)

	// Test storage creation with default settings
	logger := zaptest.NewLogger(t)
	storage, err := factory.CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	// Verify it's actually an InMemoryStorage instance
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

	// Since we can't directly access the maxConversationHistory field,
	// we just verify that creation succeeded with the option
}

func TestGlobalRegistryFunctions(t *testing.T) {
	// Clear the global registry for clean test
	globalRegistry = &StorageFactoryRegistry{
		factories: make(map[string]StorageFactory),
	}

	// Register the default memory provider
	RegisterStorageProvider("memory", &InMemoryStorageFactory{})

	// Test getting supported providers
	providers := GetSupportedProviders()
	assert.Contains(t, providers, "memory")

	// Test getting storage provider
	factory, err := GetStorageProvider("memory")
	require.NoError(t, err)
	assert.NotNil(t, factory)

	// Test creating storage through global function
	logger := zaptest.NewLogger(t)
	cfg := config.QueueConfig{
		Provider: "memory",
	}

	storage, err := CreateStorage(context.Background(), cfg, logger)
	require.NoError(t, err)
	assert.NotNil(t, storage)
}

// MockStorageFactory is a test helper for mocking storage factories
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
	// Return a simple in-memory storage for testing
	return NewInMemoryStorage(logger, 20), nil
}

// TestConfigQueueConfigExtensions tests the new fields in QueueConfig
func TestConfigQueueConfigExtensions(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected config.QueueConfig
	}{
		{
			name: "default memory provider",
			envVars: map[string]string{
				// No QUEUE_* env vars set
			},
			expected: config.QueueConfig{
				Provider:        "memory",
				MaxSize:         100,
				CleanupInterval: 30 * time.Second,
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
				CleanupInterval: 30 * time.Second,
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
				CleanupInterval: 30 * time.Second,
				// Options parsing from env vars may require special handling
				// which can be implemented later if needed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test lookuper with our environment variables
			lookuper := &testLookuper{envVars: tt.envVars}

			// Create base config
			baseConfig := &config.Config{}

			// Load config with our test environment
			cfg, err := config.LoadWithLookuper(context.Background(), baseConfig, lookuper)
			require.NoError(t, err)

			// Check the queue config
			assert.Equal(t, tt.expected.Provider, cfg.QueueConfig.Provider)
			assert.Equal(t, tt.expected.URL, cfg.QueueConfig.URL)
			assert.Equal(t, tt.expected.MaxSize, cfg.QueueConfig.MaxSize)
			assert.Equal(t, tt.expected.CleanupInterval, cfg.QueueConfig.CleanupInterval)

			// Check options if they exist
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

// testLookuper is a test helper for mocking environment variables
type testLookuper struct {
	envVars map[string]string
}

func (t *testLookuper) Lookup(key string) (string, bool) {
	value, exists := t.envVars[key]
	return value, exists
}