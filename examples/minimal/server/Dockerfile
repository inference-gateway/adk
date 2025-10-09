FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server
ARG AGENT_NAME=""
ARG AGENT_DESCRIPTION=""
ARG AGENT_VERSION=""

# Build the application with ldflags
RUN cd /app/examples/minimal/server && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X 'main.AgentName=${AGENT_NAME}' -X 'main.AgentDescription=${AGENT_DESCRIPTION}' -X 'main.AgentVersion=${AGENT_VERSION}'" \
    -a -installsuffix cgo -o server .

# Build the server
RUN go build -o server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/examples/minimal/server/server .

EXPOSE 8080

CMD ["./server"]
