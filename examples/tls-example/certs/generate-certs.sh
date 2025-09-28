#!/bin/bash

# TLS Certificate Generation Script for A2A Server Example
#
# This script generates self-signed certificates for demonstrating
# TLS-enabled A2A server communication.
#
# Usage: ./generate-certs.sh
#
# Generated files:
#   - ca.key: Root CA private key
#   - ca.crt: Root CA certificate  
#   - server.key: Server private key
#   - server.crt: Server certificate signed by CA
#
# Note: These certificates are for development/testing only.
# Do not use in production environments.

set -e

echo "🔒 Generating TLS certificates for A2A Server example..."

# Create certificates directory if it doesn't exist
CERT_DIR="$(dirname "$0")"
cd "$CERT_DIR"

# Configuration
COUNTRY="US"
STATE="CA"
CITY="San Francisco"
ORGANIZATION="A2A Development"
ORGANIZATIONAL_UNIT="TLS Example"
EMAIL="dev@example.com"
COMMON_NAME="localhost"

# Certificate validity (1 year)
DAYS=365

echo "📁 Working in directory: $(pwd)"

# Clean up any existing certificates
echo "🧹 Cleaning up existing certificates..."
rm -f *.key *.crt *.csr *.srl

# 1. Generate CA private key
echo "🔑 Generating CA private key..."
openssl genrsa -out ca.key 4096

# 2. Generate CA certificate
echo "📜 Generating CA certificate..."
openssl req -new -x509 -key ca.key -sha256 -subj "/C=${COUNTRY}/ST=${STATE}/L=${CITY}/O=${ORGANIZATION}/OU=${ORGANIZATIONAL_UNIT} CA/CN=A2A Example CA/emailAddress=${EMAIL}" -days ${DAYS} -out ca.crt

# 3. Generate server private key
echo "🔑 Generating server private key..."
openssl genrsa -out server.key 4096

# 4. Generate server certificate signing request
echo "📋 Generating server certificate signing request..."
openssl req -new -key server.key -subj "/C=${COUNTRY}/ST=${STATE}/L=${CITY}/O=${ORGANIZATION}/OU=${ORGANIZATIONAL_UNIT}/CN=${COMMON_NAME}/emailAddress=${EMAIL}" -out server.csr

# 5. Create extensions file for server certificate
echo "📝 Creating certificate extensions..."
cat > server.ext << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
DNS.3 = server
DNS.4 = tls-server
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# 6. Generate server certificate signed by CA
echo "📜 Generating server certificate..."
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -days ${DAYS} -sha256 -extfile server.ext

# 7. Clean up temporary files
echo "🧹 Cleaning up temporary files..."
rm -f server.csr server.ext ca.srl

# 8. Set appropriate permissions
echo "🔐 Setting certificate permissions..."
chmod 600 *.key
chmod 644 *.crt

# 9. Verify certificates
echo "✅ Verifying certificates..."
echo
echo "CA Certificate info:"
openssl x509 -in ca.crt -text -noout | grep -E "(Subject|Issuer|Not Before|Not After)"

echo
echo "Server Certificate info:"
openssl x509 -in server.crt -text -noout | grep -E "(Subject|Issuer|Not Before|Not After|DNS|IP Address)"

echo
echo "Certificate chain verification:"
openssl verify -CAfile ca.crt server.crt

echo
echo "🎉 Certificate generation completed successfully!"
echo
echo "Generated files:"
echo "  📁 $(pwd)/"
echo "  ├── ca.crt        (Root CA certificate)"
echo "  ├── ca.key        (Root CA private key)"
echo "  ├── server.crt    (Server certificate)" 
echo "  └── server.key    (Server private key)"
echo
echo "📋 Next steps:"
echo "  1. Run 'docker-compose up --build' to start the TLS example"
echo "  2. The server will use these certificates for HTTPS"
echo "  3. The client will connect securely using TLS"
echo
echo "⚠️  Note: These are self-signed certificates for development only."
echo "    The client is configured to skip TLS verification by default."