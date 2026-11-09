package server

import (
	"fmt"
	"slices"
	"sync"

	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
)

// ArtifactStorageFactory creates an artifact storage provider.
// Non-default backends are opt-in: they register themselves here from their
// subpackage via init(), so a program only links the storage it actually uses.
type ArtifactStorageFactory func(cfg *serverConfig.ArtifactsStorageConfig, logger *zap.Logger) (ArtifactStorageProvider, error)

// ArtifactStorageFactoryRegistry manages registered artifact storage providers
type ArtifactStorageFactoryRegistry struct {
	mu        sync.RWMutex
	factories map[string]ArtifactStorageFactory
}

// globalArtifactStorageRegistry is the global artifact storage factory registry
var globalArtifactStorageRegistry = &ArtifactStorageFactoryRegistry{
	factories: make(map[string]ArtifactStorageFactory),
}

// RegisterArtifactStorageProvider registers an artifact storage backend under an
// ARTIFACTS_STORAGE_PROVIDER value. Backends that ship outside this package call
// it from their subpackage's init(), opted into with a blank import.
func RegisterArtifactStorageProvider(provider string, factory ArtifactStorageFactory) {
	globalArtifactStorageRegistry.Register(provider, factory)
}

// GetArtifactStorageProvider retrieves an artifact storage backend factory
func GetArtifactStorageProvider(provider string) (ArtifactStorageFactory, error) {
	return globalArtifactStorageRegistry.GetFactory(provider)
}

// Register registers a factory for a provider
func (r *ArtifactStorageFactoryRegistry) Register(provider string, factory ArtifactStorageFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[provider] = factory
}

// GetFactory retrieves a factory for a provider
func (r *ArtifactStorageFactoryRegistry) GetFactory(provider string) (ArtifactStorageFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[provider]
	if !exists {
		return nil, fmt.Errorf("unsupported storage provider: %s (supported: %v)", provider, r.getProviderNames())
	}

	return factory, nil
}

// getProviderNames returns the registered provider names (must be called with the read lock held)
func (r *ArtifactStorageFactoryRegistry) getProviderNames() []string {
	providers := make([]string, 0, len(r.factories))
	for provider := range r.factories {
		providers = append(providers, provider)
	}
	slices.Sort(providers)
	return providers
}

// optInArtifactStorageImports maps providers shipped outside this package to the
// subpackage providing them, so the not-linked error can name the import to add
var optInArtifactStorageImports = map[string]string{
	"s3":    "github.com/inference-gateway/adk/server/storage/s3",
	"minio": "github.com/inference-gateway/adk/server/storage/s3",
}

// init registers the built-in filesystem provider, the default backend
func init() {
	RegisterArtifactStorageProvider("filesystem", filesystemArtifactStorageFactory)
}

// filesystemArtifactStorageFactory creates the built-in filesystem storage
func filesystemArtifactStorageFactory(cfg *serverConfig.ArtifactsStorageConfig, logger *zap.Logger) (ArtifactStorageProvider, error) {
	storage, err := NewFilesystemArtifactStorage(cfg)
	if err != nil {
		return nil, err
	}

	logger.Info("artifact service initialized with filesystem storage",
		zap.String("base_path", cfg.BasePath),
		zap.String("base_url", cfg.BaseURL))

	return storage, nil
}
