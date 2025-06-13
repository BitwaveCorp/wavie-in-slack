FROM golang:1.24-alpine AS builder

WORKDIR /app

# First, create a temporary directory for the shared utils package
RUN mkdir -p /go/src/github.com/BitwaveCorp/shared-svcs/shared/utils

# Create a minimal go.mod in the shared utils directory
RUN echo 'module github.com/BitwaveCorp/shared-svcs/shared/utils\n\ngo 1.24' > /go/src/github.com/BitwaveCorp/shared-svcs/shared/utils/go.mod

# Create a dummy utils.go file to satisfy imports
RUN echo 'package utils\n\n// Placeholder for shared utils\nfunc Placeholder() string {\n    return "placeholder"\n}' > /go/src/github.com/BitwaveCorp/shared-svcs/shared/utils/utils.go

# Copy the Go module files
COPY go.mod ./

# Modify go.mod to remove the replace directive
RUN sed -i '/replace github.com\/BitwaveCorp\/shared-svcs\/shared\/utils/d' go.mod

# Create a temporary main.go file that imports the dependencies
RUN echo 'package main; import _ "github.com/kelseyhightower/envconfig"; import _ "github.com/joho/godotenv"; func main() {}' > temp_main.go

# Initialize modules and download dependencies
RUN go mod tidy && \
    go get github.com/kelseyhightower/envconfig@v1.4.0 && \
    go get github.com/joho/godotenv@v1.5.1 && \
    go get github.com/BitwaveCorp/shared-svcs/shared/utils@v0.0.0 && \
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
