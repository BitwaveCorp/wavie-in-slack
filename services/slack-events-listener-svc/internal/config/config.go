package config

type Config struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
	Port     int    `envconfig:"PORT" default:"8080"`

	SlackBotToken      string `envconfig:"SLACK_BOT_TOKEN" required:"true"`
	SlackSigningSecret string `envconfig:"SLACK_SIGNING_SECRET" required:"true"`

	ClaudeProxyServiceURL  string `envconfig:"CLAUDE_PROXY_SERVICE_URL" required:"true"`
	BroadcastServiceURL string `envconfig:"BROADCAST_SERVICE_URL" required:"true"`
	
	// The agent ID to use for this bot instance
	AgentID string `envconfig:"AGENT_ID" default:"wavie-bot"`
}
