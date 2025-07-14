# Wavie Slack Bot - Deployed Services

This document contains the URLs for all deployed Wavie Slack Bot microservices.

## Service URLs

| Service | URL |
|---------|-----|
| Claude Agent Proxy Service | https://wavie-claude-proxy-s-455488113475.us-central1.run.app |
| RAG Service | https://wavie-rag-svc-455488113475.us-central1.run.app |
| Events Listener Service | https://wavie-in-slack-events-listener-455488113475.us-central1.run.app |
| Broadcaster Service | https://wavie-in-slack-broadcaster-455488113475.us-central1.run.app |

## Service Endpoints

### Claude Agent Proxy Service
- Knowledge Management UI: `/knowledge`
- Upload API: `/api/knowledge/upload`
- Delete API: `/api/knowledge/files/delete`
- List Files API: `/api/knowledge/files`

### RAG Service
- Document Upload: `/api/documents` (POST)
- Document Delete: `/api/documents/{documentId}` (DELETE)
- Ask Question: `/api/ask` (POST)
- Health Check: `/health` (GET)

## Testing Commands

### Test RAG Service Health
```bash
curl -s https://wavie-rag-svc-455488113475.us-central1.run.app/health
```

### Test RAG Service Question Answering
```bash
curl -X POST -H "Content-Type: application/json" -d '{"question": "What is Wavie?"}' https://wavie-rag-svc-455488113475.us-central1.run.app/api/ask
```

### Test Claude Proxy Health
```bash
curl -s https://wavie-claude-proxy-s-455488113475.us-central1.run.app/health
```
