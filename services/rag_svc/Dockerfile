FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /rag-service ./cmd/main.go

# Create final image
FROM alpine:latest

WORKDIR /

# Copy binary from builder
COPY --from=builder /rag-service /rag-service

# Create entrypoint script
RUN printf '#!/bin/sh\n\
echo "Starting RAG Service..."\n\
echo "PORT: $PORT"\n\
echo "GCP_PROJECT_ID: $GCP_PROJECT_ID"\n\
echo "GCP_STORAGE_BUCKET: $GCP_STORAGE_BUCKET"\n\
echo "GCP_LOCATION: $GCP_LOCATION"\n\
echo "VERTEX_INDEX_ID: $VERTEX_INDEX_ID"\n\
echo "VERTEX_INDEX_ENDPOINT_ID: $VERTEX_INDEX_ENDPOINT_ID"\n\
echo "VERTEX_DEPLOYED_INDEX_ID: $VERTEX_DEPLOYED_INDEX_ID"\n\
echo "OPENAI_API_KEY: [REDACTED]"\n\
echo "OPENAI_EMBEDDING_MODEL: $OPENAI_EMBEDDING_MODEL"\n\
exec /rag-service\n' > /entrypoint.sh && chmod +x /entrypoint.sh

# Expose port
EXPOSE 8084

# Set entrypoint
ENTRYPOINT ["/entrypoint.sh"]
