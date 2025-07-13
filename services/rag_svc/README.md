# RAG Service for Wavie Slack Bot

This service implements Retrieval-Augmented Generation (RAG) functionality for the Wavie Slack Bot system. It provides vector-based document search capabilities to enhance Claude's responses with relevant context from your knowledge base.

## Features

- Document processing and chunking
- Vector embeddings generation using OpenAI's API
- Vector storage and retrieval using Google Cloud Vertex AI Vector Search
- API endpoints for document upload, question answering, and document deletion

## Architecture

The RAG service is designed as a standalone microservice that integrates with the existing Wavie Slack Bot system. It follows this workflow:

### Upload Flow:
1. User uploads a document (e.g., .md file)
2. File is saved to GCP bucket by the existing system
3. Claude agent proxy calls the RAG service to process the document
4. RAG service reads the file content from GCP Storage
5. Content is split into chunks with overlap
6. Embeddings are generated using OpenAI's API
7. Chunks and vectors are stored in Firestore and Vertex AI Vector Search

### Chat Flow:
1. User asks a question in Slack
2. Claude agent proxy calls the RAG service with the question
3. RAG service converts the question to an embedding
4. Similar vectors are found using Vertex AI Vector Search
5. Relevant document chunks are retrieved
6. Chunks are returned to the Claude agent proxy
7. Claude agent proxy sends the question + context to Claude
8. Claude responds with relevant information

## Setup

### Prerequisites

- Go 1.19+
- Google Cloud Platform account with:
  - Vertex AI Vector Search enabled
  - Firestore database
  - GCP Storage bucket
- OpenAI API key

### Environment Variables

Copy the `.env.example` file to `.env` and fill in the required values:

```
# Server configuration
PORT=8084

# GCP configuration
GCP_PROJECT_ID=your-project-id
GCP_STORAGE_BUCKET=your-storage-bucket
GCP_LOCATION=us-central1

# Vertex AI Vector Search configuration
VERTEX_INDEX_ID=your-index-id
VERTEX_INDEX_ENDPOINT_ID=your-index-endpoint-id
VERTEX_DEPLOYED_INDEX_ID=your-deployed-index-id

# OpenAI API configuration
OPENAI_API_KEY=your-openai-api-key
OPENAI_EMBEDDING_MODEL=text-embedding-3-small
```

### Setting up Vertex AI Vector Search

1. Create a Vector Search index in the GCP Console:
   - Go to Vertex AI > Vector Search
   - Create a new index with dimensions=1536 (for OpenAI embeddings)
   - Select "Cosine similarity" as the distance measure
   - Deploy the index to an endpoint

2. Note the index ID, endpoint ID, and deployed index ID for your environment variables.

## Building and Running

### Local Development

```bash
# Install dependencies
go mod tidy

# Run the service
go run cmd/main.go
```

### Docker

```bash
# Build the Docker image
docker build -t rag-service .

# Run the container
docker run -p 8084:8084 --env-file .env rag-service
```

### Google Cloud Run

```bash
# Build and push to Google Container Registry
gcloud builds submit --tag gcr.io/YOUR_PROJECT_ID/rag-service

# Deploy to Cloud Run
gcloud run deploy rag-service \
  --image gcr.io/YOUR_PROJECT_ID/rag-service \
  --platform managed \
  --memory 1Gi \
  --timeout 300s \
  --set-env-vars="GCP_PROJECT_ID=YOUR_PROJECT_ID,GCP_STORAGE_BUCKET=YOUR_BUCKET,..."
```

## API Endpoints

### Upload Document

```
POST /api/documents
Content-Type: application/json

{
  "document_id": "unique-doc-id",
  "file_path": "path/to/file.md"
}
```

Response:
```json
{
  "status": "success",
  "message": "Document processed successfully. Estimated cost: $0.000123",
  "document_id": "unique-doc-id",
  "chunk_count": 5
}
```

### Ask Question

```
POST /api/ask
Content-Type: application/json

{
  "question": "What is our vacation policy?"
}
```

Response:
```json
{
  "answer": "Based on the provided context, your vacation policy allows employees to take up to 15 days of paid time off per year...",
  "source_chunks": [
    "Our vacation policy allows employees to take up to 15 days of paid time off per year...",
    "To request vacation time, employees must submit a request through the HR portal..."
  ]
}
```

### Delete Document

```
DELETE /api/documents/{documentId}
```

Response:
```json
{
  "status": "success",
  "message": "Document embeddings deleted successfully",
  "document_id": "unique-doc-id"
}
```

### Health Check

```
GET /health
```

Response:
```json
{
  "status": "ok",
  "timestamp": "2025-07-12T22:15:55-04:00",
  "version": "1.0.0"
}
```

## Integration with Claude Agent Proxy

To integrate this service with the existing Claude Agent Proxy:

1. Add the RAG service URL to the Claude Agent Proxy configuration
2. Modify the document upload handler to call the RAG service
3. Modify the chat handler to retrieve context before calling Claude

## Performance Considerations

- The service uses batch processing for embedding generation to avoid rate limits
- Vector search is optimized for low latency using Vertex AI's specialized service
- Consider caching frequently accessed embeddings for better performance
