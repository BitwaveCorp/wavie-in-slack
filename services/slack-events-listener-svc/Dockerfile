FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy the Go module files
COPY go.mod ./

# Create a temporary main.go file that imports the dependency
RUN echo 'package main; import _ "github.com/kelseyhightower/envconfig"; func main() {}' > temp_main.go

# Initialize modules and download dependencies
RUN go mod tidy && \
    go get github.com/kelseyhightower/envconfig@v1.4.0 && \
    go mod download

# Remove the temporary file and copy the actual source code
RUN rm temp_main.go
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/slack-events-listener-svc

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
