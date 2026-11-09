package server_test

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	mocks "github.com/inference-gateway/adk/server/mocks"

	zap "go.uber.org/zap"
	zaptest "go.uber.org/zap/zaptest"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
)

func TestNewArtifactService_UnknownStorageProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  string
	}{
		{
			name:     "unknown provider falls back to the generic error",
			provider: "gcs",
			wantErr:  "unsupported storage provider: gcs",
		},
		{
			name:     "s3 names the opt-in backend subpackage to import",
			provider: "s3",
			wantErr:  "add a blank import of github.com/inference-gateway/adk/server/storage/s3",
		},
		{
			name:     "minio legacy alias names the opt-in backend subpackage to import",
			provider: "minio",
			wantErr:  "add a blank import of github.com/inference-gateway/adk/server/storage/s3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &serverConfig.ArtifactsConfig{
				Enable: true,
				StorageConfig: serverConfig.ArtifactsStorageConfig{
					Provider: tt.provider,
					BasePath: t.TempDir(),
				},
			}

			artifactService, err := server.NewArtifactService(cfg, zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)))
			require.Error(t, err)
			assert.Nil(t, artifactService)
			assert.ErrorContains(t, err, tt.wantErr)
			assert.ErrorContains(t, err, tt.provider)
		})
	}
}

func TestNewArtifactService_RegistryDrivenProvider(t *testing.T) {
	provider := "test-registry-backend"
	storage := &mocks.FakeArtifactStorageProvider{}

	var configured serverConfig.ArtifactsStorageConfig
	server.RegisterArtifactStorageProvider(provider, func(cfg *serverConfig.ArtifactsStorageConfig, logger *zap.Logger) (server.ArtifactStorageProvider, error) {
		configured = *cfg
		logger.Info("initialized the test artifact storage", zap.String("provider", provider))
		return storage, nil
	})

	cfg := &serverConfig.ArtifactsConfig{
		Enable: true,
		StorageConfig: serverConfig.ArtifactsStorageConfig{
			Provider: provider,
			BasePath: "./test-artifacts-registry",
		},
	}

	artifactService, err := server.NewArtifactService(cfg, zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)))
	require.NoError(t, err)
	require.NotNil(t, artifactService)
	assert.Equal(t, "./test-artifacts-registry", configured.BasePath)
}

func TestNewArtifactService_RegistryDrivenProviderFailure(t *testing.T) {
	provider := "test-failing-backend"
	server.RegisterArtifactStorageProvider(provider, func(cfg *serverConfig.ArtifactsStorageConfig, logger *zap.Logger) (server.ArtifactStorageProvider, error) {
		return nil, assert.AnError
	})

	cfg := &serverConfig.ArtifactsConfig{
		Enable: true,
		StorageConfig: serverConfig.ArtifactsStorageConfig{
			Provider: provider,
			BasePath: "./test-artifacts-failing",
		},
	}

	artifactService, err := server.NewArtifactService(cfg, zaptest.NewLogger(t, zaptest.Level(zap.ErrorLevel)))
	require.Error(t, err)
	assert.Nil(t, artifactService)
	assert.ErrorContains(t, err, "failed to create test-failing-backend storage")
	assert.ErrorIs(t, err, assert.AnError)
}
