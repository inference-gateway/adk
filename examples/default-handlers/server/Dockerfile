FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the server
ARG AGENT_NAME="default-handlers-agent"
ARG AGENT_DESCRIPTION="An AI-powered A2A server with default handlers"
ARG AGENT_VERSION="0.1.0"

# Build the application with ldflags
RUN cd /app/examples/default-handlers/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X 'main.AgentName=${AGENT_NAME}' -X 'main.AgentDescription=${AGENT_DESCRIPTION}' -X 'main.AgentVersion=${AGENT_VERSION}'" \
    -a -installsuffix cgo -o server .

# Start a new stage from scratch
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/examples/default-handlers/server/server .

# Expose port 8080
EXPOSE 8080

# Command to run the executable
CMD ["./server"]
