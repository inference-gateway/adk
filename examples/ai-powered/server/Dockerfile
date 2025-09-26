FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server
ARG AGENT_NAME="ai-powered-agent"
ARG AGENT_DESCRIPTION="An AI-powered A2A server with weather and time tools"
ARG AGENT_VERSION="0.1.0"

RUN go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName=${AGENT_NAME} -X github.com/inference-gateway/adk/server.BuildAgentDescription=${AGENT_DESCRIPTION} -X github.com/inference-gateway/adk/server.BuildAgentVersion=${AGENT_VERSION}" -o server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
