FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server
ARG AGENT_NAME="ai-powered-agent"
ARG AGENT_DESCRIPTION="An AI-powered A2A server with weather and time tools - supporting streaming"
ARG AGENT_VERSION="0.1.0"

# Build the application with ldflags
RUN cd /app/examples/ai-powered-streaming/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X 'main.AgentName=${AGENT_NAME}' -X 'main.AgentDescription=${AGENT_DESCRIPTION}' -X 'main.AgentVersion=${AGENT_VERSION}'" \
    -a -installsuffix cgo -o server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/examples/ai-powered-streaming/server/server .

EXPOSE 8080

CMD ["./server"]
