FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN cd /build/examples/artifacts-filesystem/client && \
    CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo -o client .

FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /build/examples/artifacts-filesystem/client/client .

# Copy the uploads directory with dummy data
COPY --from=builder /build/examples/artifacts-filesystem/client/uploads ./uploads

# Create downloads directory
RUN mkdir -p /app/downloads

# Run the application
CMD ["./client"]
