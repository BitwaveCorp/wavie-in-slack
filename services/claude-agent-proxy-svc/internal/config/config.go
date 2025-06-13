package config

type Config struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Port     int    `envconfig:"PORT" default:"8081"`

	ClaudeAPIKey string `envconfig:"CLAUDE_API_KEY" required:"true"`
	ClaudeModel  string `envconfig:"CLAUDE_MODEL" default:"claude-3-opus-20240229"`
}
