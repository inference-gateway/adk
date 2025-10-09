FROM golang:1.25-alpine AS builder

ARG AGENT_NAME="in-memory-queue-example"
ARG AGENT_DESCRIPTION="A2A server demonstrating in-memory queue storage"
ARG AGENT_VERSION="0.1.0"

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with build-time metadata
RUN cd examples/queue-storage/in-memory/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X 'github.com/inference-gateway/adk/server.BuildAgentName=${AGENT_NAME}' -X 'github.com/inference-gateway/adk/server.BuildAgentDescription=${AGENT_DESCRIPTION}' -X 'github.com/inference-gateway/adk/server.BuildAgentVersion=${AGENT_VERSION}'" \
    -a -installsuffix cgo -o server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /app

COPY --from=builder /app/examples/queue-storage/in-memory/server/server .

EXPOSE 8080

CMD ["./server"]
