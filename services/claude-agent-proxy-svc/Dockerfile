FROM golang:1.24-alpine AS builder

# Install git for dependency management
RUN apk add --no-cache git

WORKDIR /app

# Copy the Go module files
COPY go.mod ./

# Create a temporary directory for shared utils
RUN mkdir -p internal/shared/utils

# Create a dummy utils.go file in the internal directory
RUN echo 'package utils\n\n// Placeholder for shared utils\nfunc Placeholder() string {\n    return "placeholder"\n}' > internal/shared/utils/utils.go

# Create a temporary main.go file that imports the dependencies
RUN echo 'package main\n\nimport (\n    _ "github.com/kelseyhightower/envconfig"\n    _ "github.com/joho/godotenv"\n)\n\nfunc main() {}' > temp_main.go

# Initialize modules and download dependencies
RUN go mod tidy && \
    go get github.com/kelseyhightower/envconfig@v1.4.0 && \
    go get github.com/joho/godotenv@v1.5.1 && \
    go mod download

# Remove the temporary file and copy the actual source code
RUN rm temp_main.go
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/claude-agent-proxy-svc

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8081
CMD ["./main"]
