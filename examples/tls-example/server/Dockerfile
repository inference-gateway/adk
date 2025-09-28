FROM golang:1.25-alpine AS builder

# Build-time arguments for agent metadata
ARG AGENT_NAME="tls-server"
ARG AGENT_DESCRIPTION="TLS-enabled A2A server demonstrating secure communication"
ARG AGENT_VERSION="1.0.0"

WORKDIR /app

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application with build-time metadata
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName=${AGENT_NAME} \
              -X github.com/inference-gateway/adk/server.BuildAgentDescription=${AGENT_DESCRIPTION} \
              -X github.com/inference-gateway/adk/server.BuildAgentVersion=${AGENT_VERSION}" \
    -o server .

FROM alpine:latest

# Install ca-certificates for TLS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/server .

# Create directory for certificates
RUN mkdir -p /certs

# Expose the TLS port
EXPOSE 8443

CMD ["./server"]