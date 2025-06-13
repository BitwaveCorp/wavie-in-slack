module github.com/BitwaveCorp/slack-wavie-bot-system-upgraded/services/claude-agent-proxy-svc

go 1.24

require (
	github.com/BitwaveCorp/shared-svcs/shared/utils v0.0.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/joho/godotenv v1.5.1
)

replace github.com/BitwaveCorp/shared-svcs/shared/utils => ../../shared/utils
