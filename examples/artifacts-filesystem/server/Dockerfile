FROM golang:1.25-alpine AS builder

ARG AGENT_NAME=artifacts-filesystem-agent
ARG AGENT_DESCRIPTION="An agent that creates and serves artifacts using filesystem storage"
ARG AGENT_VERSION=0.1.0

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with ldflags
RUN cd examples/artifacts-filesystem/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X 'main.AgentName=${AGENT_NAME}' -X 'main.AgentDescription=${AGENT_DESCRIPTION}' -X 'main.AgentVersion=${AGENT_VERSION}'" \
    -a -installsuffix cgo -o main .

FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/examples/artifacts-filesystem/server/main .

# Create artifacts directory
RUN mkdir -p /app/artifacts

# Expose ports
EXPOSE 8080 8081

# Run the application
CMD ["./main"]