package config

import (
	"os"
	"strconv"
)

// Config holds the service configuration
type Config struct {
	Server ServerConfig
	OpenAI OpenAIConfig
	GCP    GCPConfig
	Vertex VertexConfig
	Claude ClaudeConfig
}

// ServerConfig holds the server configuration
type ServerConfig struct {
	Port string
}

// OpenAIConfig holds the OpenAI API configuration
type OpenAIConfig struct {
	APIKey string
	Model  string
}

// GCPConfig holds the GCP configuration
type GCPConfig struct {
	ProjectID     string
	StorageBucket string
	Location      string
}

// VertexConfig holds the Vertex AI configuration
type VertexConfig struct {
	IndexID         string
	IndexEndpointID string
	DeployedIndexID string
}

// ClaudeConfig holds the Claude API configuration
type ClaudeConfig struct {
	APIKey string
	Model  string
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8084"),
		},
		OpenAI: OpenAIConfig{
			APIKey: getEnv("OPENAI_API_KEY", ""),
			Model:  getEnv("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),
		},
		GCP: GCPConfig{
			ProjectID:     getEnv("GCP_PROJECT_ID", ""),
			StorageBucket: getEnv("GCP_STORAGE_BUCKET", ""),
			Location:      getEnv("GCP_LOCATION", "us-central1"),
		},
		Vertex: VertexConfig{
			IndexID:         getEnv("VERTEX_INDEX_ID", ""),
			IndexEndpointID: getEnv("VERTEX_INDEX_ENDPOINT_ID", ""),
			DeployedIndexID: getEnv("VERTEX_DEPLOYED_INDEX_ID", ""),
		},
		Claude: ClaudeConfig{
			APIKey: getEnv("CLAUDE_API_KEY", ""),
			Model:  getEnv("CLAUDE_MODEL", "claude-3-opus-20240229"),
		},
	}
}

// Helper function to get environment variable with a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to get environment variable as integer
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
