module github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc

go 1.24

require (
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/kelseyhightower/envconfig v1.4.0
)

replace github.com/BitwaveCorp/shared-svcs/shared/utils => ../../shared/utils
