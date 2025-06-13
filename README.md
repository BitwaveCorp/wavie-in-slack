# Slack Wavie Bot System (Upgraded)

This is the upgraded version of the Slack Wavie Bot system with a comprehensive feedback system and multi-tenant knowledge base integration. This version allows users to provide feedback to bot responses through reactions (👍/👎) and text messages (starting with "***"), and supports multiple specialized agents with their own knowledge bases.

## Services

The system consists of three microservices:

1. **slack-events-listener-svc**: Handles Slack events, processes user messages, and collects feedback. Each instance of this service represents a specific agent with its own identity and knowledge base.
2. **claude-agent-proxy-svc**: Connects to the Claude AI API to generate responses. Includes a multi-tenant knowledge base system that retrieves and injects relevant knowledge into prompts based on the agent ID.
3. **broadcast-bot-svc**: Posts messages to Slack channels, including user feedback.

## Features

- **AI-powered responses**: Uses Claude AI to generate helpful responses to user queries.
- **Multi-tenant knowledge base system**:
  - Each agent has its own dedicated knowledge base
  - Support for uploading zipped markdown files as knowledge sources
  - Knowledge content is automatically injected into Claude prompts
  - Web UI for managing agents and knowledge files
- **1:1 Bot-to-Agent mapping**:
  - Each Slack bot instance represents a single specialized agent
  - Agent identity is configured via environment variables
  - No need for complex agent selection logic in messages
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

### Multi-tenant Knowledge Base Configuration

#### Claude Agent Proxy Service

```
# Knowledge base settings
KNOWLEDGE_ENABLED=true
KNOWLEDGE_BASE_PATH=./knowledge
```

#### Slack Events Listener Service

To configure which agent a particular bot instance represents:

```
# Agent configuration
AGENT_ID=finance-bot  # Replace with the specific agent ID for this bot instance
```

## Multi-tenant Knowledge Base Setup

The system supports multiple agents, each with their own specialized knowledge base. Here's how to set it up:

1. **Start the Claude Agent Proxy Service** with knowledge management enabled:

```bash
cd services/claude-agent-proxy-svc && go run cmd/claude-agent-proxy-svc/main.go
```

2. **Access the Knowledge Management UI** at `http://localhost:8081/knowledge`

3. **Create Agents** using the web interface or API:

```bash
# Create a new agent via API
curl -X POST http://localhost:8081/api/knowledge/agents -H "Content-Type: application/json" -d '{"name":"Finance Bot", "id":"finance-bot", "description":"Specialized agent for finance questions"}'
```

4. **Upload Knowledge Files** for each agent:
   - Prepare markdown files with relevant knowledge for each agent
   - Zip the markdown files
   - Upload the zip file through the web UI or API, associating it with a specific agent

5. **Deploy Multiple Bot Instances** - For each agent you want to expose as a Slack bot:
   - Create a new Slack App and Bot Token
   - Deploy an instance of the slack-events-listener-svc with:

## Knowledge Base Storage Structure

Understanding how the knowledge base is stored can help with maintenance and troubleshooting:

### Agent Storage

All agents are stored in a central registry file located at:
```
<knowledge_base_path>/registry.json
```

By default, this is at `./knowledge/registry.json` relative to where the Claude Agent Proxy Service is running. This registry contains:
- Agent definitions (ID, name, description, tenant ID)
- Knowledge file metadata (including agent associations)

### Knowledge File Storage

When you upload knowledge files, they are stored in the following structure:

```
<knowledge_base_path>/files/<file_id>/
  ├── content.zip        # The original uploaded zip file
  └── extracted/         # Directory containing extracted markdown files
      ├── file1.md
      ├── file2.md
      └── ...
```

Where:
- `<knowledge_base_path>` is the configured knowledge base directory (default: `./knowledge`)
- `<file_id>` is a unique UUID generated for each uploaded knowledge file

The system reads the markdown files from the `extracted` directory when retrieving knowledge for a specific agent. The association between knowledge files and agents is maintained in the registry.json file.
     - The specific Slack Bot Token
     - The corresponding Agent ID

This creates a 1:1 mapping between Slack bots and knowledge agents, where each bot has access to its own specialized knowledge base.

## Development

To run the services locally:

```bash
# In separate terminal windows
cd services/slack-events-listener-svc && go run cmd/slack-events-listener-svc/main.go
cd services/claude-agent-proxy-svc && go run cmd/claude-agent-proxy-svc/main.go
cd services/broadcast-bot-svc && go run cmd/broadcast-bot-svc/main.go
```
