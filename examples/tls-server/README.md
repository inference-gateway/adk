# TLS-Enabled A2A Server Example

This example demonstrates how to run an A2A server with TLS encryption, providing secure HTTPS communication between client and server using self-signed certificates.

## Table of Contents

- [What This Example Shows](#what-this-example-shows)
- [Directory Structure](#directory-structure)
- [Quick Start](#quick-start)
- [TLS Configuration](#tls-configuration)
- [Certificate Generation](#certificate-generation)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## What This Example Shows

- A2A server configured with TLS/SSL encryption
- Self-signed certificate generation for development
- HTTPS communication between client and server
- Docker Compose orchestration with TLS setup
- Secure task submission and response handling

## Directory Structure

```
tls-example/
├── server/
│   ├── main.go           # TLS-enabled A2A server
│   ├── config/
│   │   └── config.go     # Server configuration
│   └── go.mod            # Server dependencies
├── client/
│   ├── main.go           # TLS-aware A2A client
│   ├── config/
│   │   └── config.go     # Client configuration
│   └── go.mod            # Client dependencies
├── certs/
│   └── generate-certs.sh # Self-signed certificate generation script
├── docker-compose.yaml   # TLS orchestration with cert generation
└── README.md

Note: Uses ../Dockerfile.server and ../Dockerfile.client for containers
```

## Quick Start

### Using Docker Compose (Recommended)

1. **Navigate to the example directory:**

```bash
cd examples/tls-example
```

2. **Run the complete TLS setup:**

```bash
docker-compose up --build
```

This will:

1. **Generate self-signed certificates** (if they don't exist)
2. **Start the TLS-enabled A2A server** on port 8443 (HTTPS)
3. **Run the client** to test secure communication
4. **Demonstrate encrypted message exchange**

5. **View the logs to see TLS communication:**

```bash
# In another terminal
docker-compose logs -f tls-server
docker-compose logs -f tls-client
```

### Manual Certificate Generation

If you want to generate certificates manually:

```bash
cd certs
chmod +x generate-certs.sh
./generate-certs.sh
```

### Running Locally (Development)

#### 1. Generate Certificates

```bash
cd certs
./generate-certs.sh
```

#### 2. Start the TLS Server

```bash
cd server
export A2A_SERVER_TLS_ENABLE=true
export A2A_SERVER_TLS_CERT_PATH=../certs/server.crt
export A2A_SERVER_TLS_KEY_PATH=../certs/server.key
export A2A_SERVER_PORT=8443
go run main.go
```

#### 3. Run the TLS Client

```bash
cd client
export A2A_SERVER_URL=https://localhost:8443
export A2A_SKIP_TLS_VERIFY=true
go run main.go
```

## TLS Configuration

### Server TLS Settings

The server supports the following TLS-related environment variables:

| Environment Variable       | Description                  | Default             |
| -------------------------- | ---------------------------- | ------------------- |
| `A2A_SERVER_TLS_ENABLE`    | Enable TLS/HTTPS             | `true`              |
| `A2A_SERVER_TLS_CERT_PATH` | Path to TLS certificate file | `/certs/server.crt` |
| `A2A_SERVER_TLS_KEY_PATH`  | Path to TLS private key file | `/certs/server.key` |
| `A2A_SERVER_PORT`          | HTTPS server port            | `8443`              |

### Client TLS Settings

The client supports these TLS-related environment variables:

| Environment Variable  | Description                       | Default                  |
| --------------------- | --------------------------------- | ------------------------ |
| `A2A_SERVER_URL`      | HTTPS server URL                  | `https://localhost:8443` |
| `A2A_SKIP_TLS_VERIFY` | Skip TLS certificate verification | `true` (for self-signed) |
| `A2A_TIMEOUT`         | Request timeout                   | `30s`                    |

## Certificate Details

### Generated Certificates

The `generate-certs.sh` script creates:

- **`ca.crt`** - Root Certificate Authority certificate
- **`ca.key`** - Root CA private key
- **`server.crt`** - Server certificate (signed by CA)
- **`server.key`** - Server private key

### Certificate Features

- **Validity**: 365 days (1 year)
- **Key Size**: 4096-bit RSA
- **Hash Algorithm**: SHA-256
- **Subject Alternative Names**:
  - `localhost`
  - `*.localhost`
  - `server`
  - `tls-server`
  - `127.0.0.1`
  - `::1`

### Security Notes

⚠️ **Important**: These certificates are self-signed and intended for development/testing only.

- **Do not use in production** environments
- The client skips TLS verification by default (`A2A_SKIP_TLS_VERIFY=true`)
- For production, use certificates from a trusted Certificate Authority
- Store private keys securely and never commit them to version control

## Understanding the Code

### TLS Server (`server/main.go`)

The server configures TLS through the ADK server configuration:

```go
ServerConfig: serverConfig.ServerConfig{
    Port:        "8443",
    TLSEnabled:  true,
    TLSCertFile: "/certs/server.crt",
    TLSKeyFile:  "/certs/server.key",
},
```

Key features:

- **Certificate validation** before startup
- **HTTPS endpoint** on port 8443
- **TLS-aware health checks**
- **Secure task processing**

### TLS Client (`client/main.go`)

The client creates an HTTP client configured for TLS:

```go
httpClient := &http.Client{
    Timeout: cfg.Timeout,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: cfg.SkipTLSVerify,
        },
    },
}
```

Key features:

- **HTTPS communication** with the server
- **Configurable TLS verification** (skip for self-signed certs)
- **Timeout handling** for secure requests
- **Certificate error handling**

## Testing TLS Communication

### 1. Server Health Check

```bash
# Test with curl (skip certificate verification)
curl -k https://localhost:8443/health

# Test with certificate verification
curl --cacert certs/ca.crt https://localhost:8443/health
```

### 2. Agent Card Retrieval

```bash
# Get agent information over HTTPS
curl -k https://localhost:8443/agent-card
```

### 3. Manual Task Submission

```bash
# Submit a task securely
curl -k -X POST https://localhost:8443/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "message": {
      "role": "user",
      "parts": [{"type": "text", "text": "Hello secure server!"}]
    }
  }'
```

## Production Considerations

For production deployments:

### 1. Use Valid Certificates

```bash
# Use Let's Encrypt or commercial CA certificates
A2A_SERVER_TLS_CERT_FILE=/etc/ssl/certs/server.crt
A2A_SERVER_TLS_KEY_FILE=/etc/ssl/private/server.key
```

### 2. Enable Certificate Verification

```bash
# Client should verify certificates in production
A2A_SKIP_TLS_VERIFY=false
```

### 3. Secure Certificate Storage

- Store certificates in secure locations
- Use proper file permissions (600 for keys, 644 for certificates)
- Consider using certificate management tools
- Implement certificate rotation

### 4. Network Security

- Use firewalls to restrict access
- Consider mutual TLS (mTLS) for enhanced security
- Implement proper logging and monitoring

## Troubleshooting

### Certificate Issues

```bash
# Verify certificate validity
openssl x509 -in certs/server.crt -text -noout

# Check certificate chain
openssl verify -CAfile certs/ca.crt certs/server.crt

# Test TLS connection
openssl s_client -connect localhost:8443 -servername localhost
```

### Connection Issues

```bash
# Test server connectivity
nc -zv localhost 8443

# Check if TLS is working
curl -k -v https://localhost:8443/health
```

### Docker Issues

```bash
# Check certificate generation
docker-compose logs cert-generator

# Check server startup
docker-compose logs tls-server

# Restart with fresh certificates
docker-compose down -v
docker-compose up --build
```

## Next Steps

- Explore the `minimal` example for basic A2A concepts
- Check the `ai-powered` example for AI integration
- Learn about streaming with the `streaming` example
- Review production deployment patterns in the main documentation

## Troubleshooting

### Troubleshooting with A2A Debugger

```bash
# List tasks and debug the A2A server (note: uses --skip-tls-verify for self-signed certs)
docker compose run --rm a2a-debugger tasks list
```

## Related Examples

- **`minimal/`** - Basic A2A server/client setup
- **`ai-powered/`** - AI integration patterns
- **`streaming/`** - Real-time communication
- **`default-handlers/`** - Built-in task handlers
