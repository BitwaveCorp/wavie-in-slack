# Slack Wavie Bot System (Upgraded)

This is the upgraded version of the Slack Wavie Bot system with a comprehensive feedback system. This version allows users to provide feedback to bot responses through reactions (👍/👎) and text messages (starting with "***").

## Services

The system consists of three microservices:

1. **slack-events-listener-svc**: Handles Slack events, processes user messages, and collects feedback.
2. **claude-agent-proxy-svc**: Connects to the Claude AI API to generate responses.
3. **broadcast-bot-svc**: Posts messages to Slack channels, including user feedback.

## Features

- **AI-powered responses**: Uses Claude AI to generate helpful responses to user queries.
- **Dual-mode feedback system**:
  - Reaction-based feedback (👍/👎)
  - Text-based detailed feedback (messages starting with "***")
- **Feedback tracking**: All feedback includes user ID, channel, timestamp, and correlation ID.

## Deployment

Each service can be deployed independently using Docker and Google Cloud Run:

```bash
# Build Docker images
docker build -t gcr.io/your-project-id/slack-events-listener-svc:latest ./services/slack-events-listener-svc
docker build -t gcr.io/your-project-id/claude-agent-proxy-svc:latest ./services/claude-agent-proxy-svc
docker build -t gcr.io/your-project-id/broadcast-bot-svc:latest ./services/broadcast-bot-svc

# Push to Google Container Registry
docker push gcr.io/your-project-id/slack-events-listener-svc:latest
docker push gcr.io/your-project-id/claude-agent-proxy-svc:latest
docker push gcr.io/your-project-id/broadcast-bot-svc:latest

# Deploy to Cloud Run
gcloud run deploy slack-events-listener-svc --image gcr.io/your-project-id/slack-events-listener-svc:latest --platform managed
gcloud run deploy claude-agent-proxy-svc --image gcr.io/your-project-id/claude-agent-proxy-svc:latest --platform managed
gcloud run deploy broadcast-bot-svc --image gcr.io/your-project-id/broadcast-bot-svc:latest --platform managed
```

## Configuration

Each service requires its own `.env` file with appropriate configuration. See the `.env.example` files in each service directory for required variables.

## Development

To run the services locally:

```bash
# In separate terminal windows
cd services/slack-events-listener-svc && go run cmd/slack-events-listener-svc/main.go
cd services/claude-agent-proxy-svc && go run cmd/claude-agent-proxy-svc/main.go
cd services/broadcast-bot-svc && go run cmd/broadcast-bot-svc/main.go
```
