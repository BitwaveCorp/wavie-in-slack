package config

// ServerConfig represents server-related configuration
type ServerConfig struct {
	Port int `envconfig:"PORT" default:"8081"`
}

// OpenAIConfig represents OpenAI API configuration
type OpenAIConfig struct {
	APIKey string `envconfig:"OPENAI_API_KEY" required:"false"`
	Model  string `envconfig:"OPENAI_MODEL" default:"gpt-4"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level string `envconfig:"LOG_LEVEL" default:"info"`
}

// KnowledgeConfig represents knowledge base configuration
type KnowledgeConfig struct {
	BasePath string `envconfig:"KNOWLEDGE_BASE_PATH" default:"./knowledge"`
	Enabled  bool   `envconfig:"KNOWLEDGE_ENABLED" default:"true"`
}

// Config represents the service configuration
type Config struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Port     int    `envconfig:"PORT" default:"8081"`

	ClaudeAPIKey string `envconfig:"CLAUDE_API_KEY" required:"true"`
	ClaudeModel  string `envconfig:"CLAUDE_MODEL" default:"claude-3-opus-20240229"`

	// Knowledge base configuration
	KnowledgeBasePath string `envconfig:"KNOWLEDGE_BASE_PATH" default:"./knowledge"`
	KnowledgeEnabled  bool   `envconfig:"KNOWLEDGE_ENABLED" default:"true"`

	// Embedded configs
	Server    ServerConfig
	OpenAI    OpenAIConfig
	Logging   LoggingConfig
	Knowledge KnowledgeConfig
}
