package config

// Config represents the service configuration
type Config struct {
	Server    ServerConfig
	OpenAI    OpenAIConfig
	Logging   LoggingConfig
	Knowledge KnowledgeConfig
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Port     int    `envconfig:"PORT" default:"8081"`

	ClaudeAPIKey string `envconfig:"CLAUDE_API_KEY" required:"true"`
	ClaudeModel  string `envconfig:"CLAUDE_MODEL" default:"claude-3-opus-20240229"`

	// Knowledge base configuration
	KnowledgeBasePath string `envconfig:"KNOWLEDGE_BASE_PATH" default:"./knowledge"`
	KnowledgeEnabled  bool   `envconfig:"KNOWLEDGE_ENABLED" default:"true"`
}
