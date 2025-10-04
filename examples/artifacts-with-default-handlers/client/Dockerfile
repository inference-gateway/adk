# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o client main.go

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy the binary
COPY --from=builder /app/client .

# Run the client
CMD ["./client"]