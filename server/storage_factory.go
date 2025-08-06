package server

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/inference-gateway/adk/server/config"
	"go.uber.org/zap"
)

// StorageFactory defines the interface for creating storage instances
type StorageFactory interface {
	// CreateStorage creates a storage instance with the given configuration
	CreateStorage(ctx context.Context, config config.QueueConfig, logger *zap.Logger) (Storage, error)
	
	// SupportedProvider returns the provider name this factory supports
	SupportedProvider() string
	
	// ValidateConfig validates the configuration for this provider
	ValidateConfig(config config.QueueConfig) error
}

// StorageFactoryRegistry manages registered storage providers
type StorageFactoryRegistry struct {
	mu        sync.RWMutex
	factories map[string]StorageFactory
}

// globalRegistry is the global storage factory registry
var globalRegistry = &StorageFactoryRegistry{
	factories: make(map[string]StorageFactory),
}

// RegisterStorageProvider registers a storage provider factory
func RegisterStorageProvider(provider string, factory StorageFactory) {
	globalRegistry.Register(provider, factory)
}

// GetStorageProvider retrieves a storage provider factory
func GetStorageProvider(provider string) (StorageFactory, error) {
	return globalRegistry.GetFactory(provider)
}

// GetSupportedProviders returns a list of all registered providers
func GetSupportedProviders() []string {
	return globalRegistry.GetProviders()
}

// CreateStorage creates a storage instance using the registered factories
func CreateStorage(ctx context.Context, config config.QueueConfig, logger *zap.Logger) (Storage, error) {
	return globalRegistry.CreateStorage(ctx, config, logger)
}

// Register registers a factory for a provider
func (r *StorageFactoryRegistry) Register(provider string, factory StorageFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if factory.SupportedProvider() != provider {
		panic(fmt.Sprintf("factory provider mismatch: expected %s, got %s", provider, factory.SupportedProvider()))
	}
	
	r.factories[provider] = factory
}

// GetFactory retrieves a factory for a provider
func (r *StorageFactoryRegistry) GetFactory(provider string) (StorageFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	factory, exists := r.factories[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported storage provider: %s (supported: %v)", provider, r.getProviderNames())
	}
	
	return factory, nil
}

// GetProviders returns a list of all registered provider names
func (r *StorageFactoryRegistry) GetProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getProviderNames()
}

// getProviderNames returns provider names (must be called with read lock held)
func (r *StorageFactoryRegistry) getProviderNames() []string {
	providers := make([]string, 0, len(r.factories))
	for provider := range r.factories {
		providers = append(providers, provider)
	}
	return providers
}

// CreateStorage creates a storage instance using the appropriate factory
func (r *StorageFactoryRegistry) CreateStorage(ctx context.Context, config config.QueueConfig, logger *zap.Logger) (Storage, error) {
	factory, err := r.GetFactory(config.Provider)
	if err != nil {
		return nil, err
	}
	
	if err := factory.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration for provider %s: %w", config.Provider, err)
	}
	
	return factory.CreateStorage(ctx, config, logger)
}

// InMemoryStorageFactory implements StorageFactory for in-memory storage
type InMemoryStorageFactory struct{}

// SupportedProvider returns the provider name
func (f *InMemoryStorageFactory) SupportedProvider() string {
	return "memory"
}

// ValidateConfig validates the configuration for in-memory storage
func (f *InMemoryStorageFactory) ValidateConfig(config config.QueueConfig) error {
	// In-memory storage doesn't require URL or credentials
	return nil
}

// CreateStorage creates an in-memory storage instance
func (f *InMemoryStorageFactory) CreateStorage(ctx context.Context, config config.QueueConfig, logger *zap.Logger) (Storage, error) {
	// Use default max conversation history if not specified in options
	maxConversationHistory := 20 // default value
	
	if maxHistoryStr, exists := config.Options["max_conversation_history"]; exists {
		if maxHistory, err := strconv.Atoi(maxHistoryStr); err == nil && maxHistory > 0 {
			maxConversationHistory = maxHistory
		}
	}
	
	return NewInMemoryStorage(logger, maxConversationHistory), nil
}

// init registers the default in-memory storage provider
func init() {
	RegisterStorageProvider("memory", &InMemoryStorageFactory{})
}