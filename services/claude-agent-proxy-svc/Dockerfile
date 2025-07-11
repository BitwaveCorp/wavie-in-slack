FROM golang:1.24-alpine AS builder

# Add build logging
RUN echo "=== Starting build process for claude-agent-proxy-svc ==="

# Install git and other useful debugging tools
RUN echo "=== Installing dependencies ===" && \
    apk add --no-cache git curl jq && \
    echo "=== Dependencies installed ==="

WORKDIR /app

# Copy the Go module files
COPY go.mod ./
RUN echo "=== Go module file copied ==="

# Create a temporary directory for shared utils
RUN echo "=== Creating utility directories ===" && \
    mkdir -p internal/shared/utils

# Create a dummy utils.go file in the internal directory
RUN echo "=== Creating placeholder utils file ===" && \
    echo 'package utils\n\n// Placeholder for shared utils\nfunc Placeholder() string {\n    return "placeholder"\n}' > internal/shared/utils/utils.go

# Create a temporary main.go file that imports the dependencies
RUN echo "=== Creating temporary main.go for dependency resolution ===" && \
    echo 'package main\n\nimport (\n    _ "github.com/kelseyhightower/envconfig"\n    _ "github.com/joho/godotenv"\n)\n\nfunc main() {}' > temp_main.go

# Initialize modules and download dependencies
RUN echo "=== Downloading and initializing dependencies ===" && \
    go mod tidy && \
    go get github.com/kelseyhightower/envconfig@v1.4.0 && \
    go get github.com/joho/godotenv@v1.5.1 && \
    go mod download && \
    echo "=== Dependencies downloaded ==="

# Remove the temporary file and copy the actual source code
RUN echo "=== Removing temporary files ===" && \
    rm temp_main.go

RUN echo "=== Copying source code ==="
COPY . .

# Build the application with verbose output
RUN echo "=== Building application ===" && \
    CGO_ENABLED=0 GOOS=linux go build -v -o main ./cmd/claude-agent-proxy-svc && \
    echo "=== Build completed successfully ==="

# Create a smaller final image
FROM alpine:latest

# Add runtime logging
RUN echo "=== Preparing runtime environment ==="

# Install CA certificates and debugging tools
RUN apk --no-cache add ca-certificates curl jq

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Print environment variables at container startup (non-sensitive only)
RUN echo '#!/bin/sh\necho "=== Container starting ==="\necho "=== Environment variables (non-sensitive) ==="\nenv | grep -v -i "key\|secret\|password\|token" || true\necho "=== Starting application ==="\nexec "$@"' > /entrypoint.sh && \
    chmod +x /entrypoint.sh

EXPOSE 8081

# Use the entrypoint script to log before starting the application
ENTRYPOINT ["/entrypoint.sh"]
CMD ["./main"]
