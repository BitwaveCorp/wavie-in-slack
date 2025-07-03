package knowledge

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	
	"google.golang.org/api/option"
)

// StorageType defines the type of storage backend
type StorageType string

const (
	// LocalStorage uses the local filesystem for storage
	LocalStorage StorageType = "local"
	// GCPStorage uses Google Cloud Storage for storage
	GCPStorage StorageType = "gcp"
)

// StorageConfig contains configuration for storage backends
type StorageConfig struct {
	// Common config
	Type       StorageType `envconfig:"STORAGE_TYPE" default:"local"`
	
	// Local storage config
	LocalPath  string `envconfig:"LOCAL_STORAGE_PATH" default:"knowledge"`
	
	// GCP storage config
	GCPBucket  string `envconfig:"GCP_STORAGE_BUCKET"`
	GCPProject string `envconfig:"GCP_PROJECT_ID"`
	// Optional service account key file path
	GCPKeyFile string `envconfig:"GCP_KEY_FILE"`
}

// NewStorageBackend creates a new storage backend based on configuration
func NewStorageBackend(ctx context.Context, config StorageConfig, logger *slog.Logger) (StorageBackend, error) {
	switch config.Type {
	case LocalStorage:
		return NewStorageManager(config.LocalPath, logger)
	case GCPStorage:
		if config.GCPBucket == "" {
			return nil, fmt.Errorf("GCP_STORAGE_BUCKET must be set when using GCP storage")
		}
		
		// Create temporary directory for file operations if needed
		tempDir := filepath.Join(os.TempDir(), "wavie-knowledge-tmp")
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %w", err)
		}
		
		// Configure GCP client options
		var opts []option.ClientOption
		if config.GCPKeyFile != "" {
			opts = append(opts, option.WithCredentialsFile(config.GCPKeyFile))
		}
		// Note: Project ID is set in the client context, not needed as an option
		
		return NewGCPStorageManager(
			ctx, 
			config.GCPBucket, 
			"registry.json", 
			logger, 
			tempDir,
			opts...,
		)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", config.Type)
	}
}
