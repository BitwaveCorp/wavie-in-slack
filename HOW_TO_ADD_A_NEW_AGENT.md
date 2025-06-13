# How to Add a New Agent to the Slack Wavie Bot System

This guide provides step-by-step instructions for adding a new specialized agent to your Slack Wavie Bot system. Each agent has its own knowledge base and appears as a separate bot in your Slack workspace.

## Overview of the Multi-Tenant Architecture

The Slack Wavie Bot system uses a 1:1 mapping between Slack bot instances and knowledge agents:

- Each **Slack bot** (with its own token) corresponds to exactly one **agent ID**
- Each **agent ID** is associated with specific **knowledge files** in the knowledge base
- The **Claude Agent Proxy** service manages all agent knowledge and handles the mapping
- Multiple **Slack Events Listener** instances (one per bot) connect to the same proxy

This architecture allows you to create specialized bots for different purposes while sharing the underlying AI infrastructure.

## Step 1: Create a New Agent in the Knowledge Management System

First, create the agent identity in the knowledge management system:

1. **Start the Claude Agent Proxy Service** if it's not already running:
   ```bash
   cd services/claude-agent-proxy-svc && go run cmd/claude-agent-proxy-svc/main.go
   ```

2. **Access the Knowledge Management UI** at `http://localhost:8081/knowledge`

3. **Create a New Agent**:
   - Click on the "Agent Management" tab
   - Fill in the agent details:
     - **ID**: A unique identifier for the agent (e.g., `finance-bot`)
     - **Name**: A human-readable name (e.g., `Finance Bot`)
     - **Description**: What this agent specializes in
     - **Tenant ID**: Organization or department (can be your company name)
   - Click "Create Agent"

   Alternatively, you can use the API:
   ```bash
   curl -X POST http://localhost:8081/api/knowledge/agents \
     -H "Content-Type: application/json" \
     -d '{"id":"finance-bot", "name":"Finance Bot", "description":"Specialized agent for finance questions", "tenant_id":"your-company"}'
   ```

4. **Add Knowledge to the Agent**:
   - Prepare markdown files with relevant knowledge for the agent
   - Zip the markdown files
   - In the Knowledge Management UI, click on the "Knowledge Files" tab
   - Upload the zip file and select your new agent from the list
   - Click "Upload"

## Step 2: Create a New Slack Bot Application

Each agent needs its own Slack bot application:

1. **Go to [api.slack.com/apps](https://api.slack.com/apps)** and sign in

2. **Create a New App**:
   - Click "Create New App"
   - Choose "From scratch"
   - Name your app (e.g., "Finance Bot")
   - Select your workspace
   - Click "Create App"

3. **Configure Bot Permissions**:
   - In the sidebar, go to "OAuth & Permissions"
   - Scroll to "Scopes" > "Bot Token Scopes"
   - Add the following scopes:
     - `app_mentions:read`
     - `channels:history`
     - `chat:write`
     - `reactions:read`
   - Save changes

4. **Enable Events**:
   - In the sidebar, go to "Event Subscriptions"
   - Toggle "Enable Events" to On
   - For now, leave the Request URL blank (we'll come back to this)
   - Under "Subscribe to bot events", add:
     - `app_mention`
     - `message.channels`
     - `reaction_added`
   - Save changes

5. **Install the App to Your Workspace**:
   - In the sidebar, go to "Install App"
   - Click "Install to Workspace"
   - Review permissions and click "Allow"

6. **Get Your Bot Credentials**:
   - In "OAuth & Permissions", copy the "Bot User OAuth Token" (starts with `xoxb-`)
   - In "Basic Information" > "App Credentials", copy the "Signing Secret"

## Step 3: Deploy a New Slack Events Listener Service

Now deploy a new instance of the Slack Events Listener service for this agent:

1. **Create a New .env File** for your agent:
   ```
   # Slack Bot Configuration
   SLACK_BOT_TOKEN=xoxb-your-new-bot-token-here
   SLACK_SIGNING_SECRET=your-new-signing-secret-here
   
   # Service URLs (same as your existing deployment)
   CLAUDE_PROXY_SERVICE_URL=https://your-claude-proxy-service-url
   BROADCAST_SERVICE_URL=https://your-broadcast-service-url
   
   # Agent Configuration - THIS IS THE IMPORTANT PART
   AGENT_ID=finance-bot  # Must match the ID created in the knowledge system
   
   # Server Configuration
   PORT=8080
   LOG_LEVEL=info
   ```

2. **Build and Deploy to Google Cloud Run**:
   ```bash
   # Build Docker image
   docker build -t gcr.io/your-project-id/finance-bot-events-listener:latest ./services/slack-events-listener-svc
   
   # Push to Google Container Registry
   docker push gcr.io/your-project-id/finance-bot-events-listener:latest
   
   # Deploy to Cloud Run with your new environment variables
   gcloud run deploy finance-bot-events-listener \
     --image gcr.io/your-project-id/finance-bot-events-listener:latest \
     --platform managed \
     --set-env-vars="AGENT_ID=finance-bot,SLACK_BOT_TOKEN=xoxb-your-token,SLACK_SIGNING_SECRET=your-secret,CLAUDE_PROXY_SERVICE_URL=https://your-url,BROADCAST_SERVICE_URL=https://your-url"
   ```

3. **Complete Slack Event Configuration**:
   - Copy the URL of your new Cloud Run service
   - Go back to your Slack App configuration at [api.slack.com/apps](https://api.slack.com/apps)
   - In "Event Subscriptions", paste your URL + `/slack/events` as the Request URL
   - Verify and save changes

## Step 4: Test Your New Agent

1. **Invite the bot to a channel** in your Slack workspace:
   ```
   /invite @finance-bot
   ```

2. **Mention the bot** to ask a question:
   ```
   @finance-bot What's the current interest rate?
   ```

3. **Verify the correct knowledge** is being used by checking that responses include information from the knowledge files you associated with this agent.

## How the Agent ID Mapping Works

Understanding how the agent ID flows through the system:

1. **Slack Events Listener Service**:
   - Reads the `AGENT_ID` from environment variables at startup
   - Includes this ID in every request to the Claude Agent Proxy
   - Does not dynamically change agents - one instance = one agent

2. **Claude Agent Proxy Service**:
   - Receives the agent ID in the request
   - Looks up all knowledge files associated with that agent ID
   - Retrieves the content from those files
   - Injects the relevant knowledge into the prompt sent to Claude
   - Returns the AI response enhanced with agent-specific knowledge

3. **Knowledge Base Registry**:
   - Stored at `<knowledge_base_path>/registry.json`
   - Contains all agent definitions and knowledge file associations
   - Maps each agent ID to its associated knowledge files
   - Used by the proxy service to retrieve the right knowledge for each request

## Important Considerations

- **Agent ID Consistency**: The agent ID in the Slack Events Listener environment must exactly match an agent ID in the knowledge management system
- **One Bot = One Agent**: Each Slack bot instance represents exactly one agent; to add more agents, deploy more bot instances
- **Shared Backend**: All bot instances communicate with the same Claude Agent Proxy and Broadcast services
- **Knowledge Isolation**: Knowledge is isolated by agent ID; one bot cannot access another bot's knowledge unless explicitly shared
- **Default Fallback**: If no agent ID is provided, the system defaults to using the "wavie-bot" agent

## Troubleshooting

- **Bot not using the right knowledge?** Verify the AGENT_ID in your .env file matches exactly with the ID in the knowledge system
- **Knowledge not appearing in responses?** Check that knowledge files are properly uploaded and associated with the correct agent
- **Getting generic responses?** Ensure the Claude Agent Proxy service has knowledge management enabled (KNOWLEDGE_ENABLED=true)
