FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy the Go module files
COPY go.mod ./

# Create a temporary main.go file that imports the dependencies
RUN echo 'package main; import _ "github.com/kelseyhightower/envconfig"; import _ "github.com/joho/godotenv"; func main() {}' > temp_main.go

# Initialize modules and download dependencies
RUN go mod tidy && \
    go get github.com/kelseyhightower/envconfig@v1.4.0 && \
    go get github.com/joho/godotenv@v1.5.1 && \
    go mod download

# Remove the temporary file and copy the actual source code
RUN rm temp_main.go
COPY . .

# Create the shared utils directory structure if it doesn't exist
RUN mkdir -p shared/utils

# Create a minimal go.mod in the shared utils directory
RUN echo 'module github.com/BitwaveCorp/shared-svcs/shared/utils\n\ngo 1.24' > shared/utils/go.mod

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/claude-agent-proxy-svc

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8081
CMD ["./main"]
