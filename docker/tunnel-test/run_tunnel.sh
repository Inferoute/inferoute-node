#!/bin/bash

set -e

echo "========================================="
echo "Starting Cloudflare Tunnel Test Container"
echo "Container ID: $(hostname)"
echo "========================================="

# Configuration
BASE_URL="${BASE_URL:-http://auth:8081}"
CLOUDFLARE_URL="${CLOUDFLARE_URL:-http://cloudflare-service:8089}"
# Always generate unique username with timestamp and random number
PROVIDER_USERNAME="${USERNAME:-provider}_$(date +%s)_$(shuf -i 1000-9999 -n 1)"
MAX_RETRIES=5

echo "Creating provider user: $PROVIDER_USERNAME"

# Function to make API calls with retries
make_request() {
    local url=$1
    local data=$2
    local retry_count=0
    
    echo "Making request to: $url" >&2
    
    while [ $retry_count -lt $MAX_RETRIES ]; do
        response=$(curl -s --max-time 10 --location "$url" \
            --header 'Content-Type: application/json' \
            --header 'X-Internal-Key: klondikee' \
            --data "$data")
            
        # Simple check - if response contains expected JSON structure, it's likely successful
        if echo "$response" | jq -e '.' >/dev/null 2>&1; then
            echo "$response"
            return 0
        fi
        
        retry_count=$((retry_count + 1))
        echo "Attempt $retry_count/$MAX_RETRIES failed. Retrying in 2 seconds..." >&2
        echo "Response: $response" >&2
        sleep 2
    done
    
    echo "Failed after $MAX_RETRIES attempts" >&2
    return 1
}

# Create provider user
echo "Creating provider user ($PROVIDER_USERNAME)..."
user_response=$(make_request "$BASE_URL/api/auth/users" "{
    \"username\": \"$PROVIDER_USERNAME\"
}")

user_id=$(echo "$user_response" | jq -r '.user.id')
if [ "$user_id" = "null" ] || [ -z "$user_id" ]; then
    echo "Failed to create user or extract user ID"
    echo "Response: $user_response"
    exit 1
fi
echo "User ID: $user_id"

# Create provider entity
echo "Creating provider entity..."
entity_response=$(make_request "$BASE_URL/api/auth/entities" "{
    \"user_id\": \"$user_id\",
    \"type\": \"provider\",
    \"name\": \"$PROVIDER_USERNAME\",
    \"api_url\": \"https://api.$PROVIDER_USERNAME.com\"
}")

provider_id=$(echo "$entity_response" | jq -r '.provider.id')
if [ "$provider_id" = "null" ] || [ -z "$provider_id" ]; then
    echo "Failed to create provider entity or extract provider ID"
    echo "Response: $entity_response"
    exit 1
fi
echo "Provider ID: $provider_id"

# Create provider API key
echo "Creating provider API key..."
api_key_response=$(make_request "$BASE_URL/api/auth/api-keys" "{
    \"user_id\": \"$user_id\",
    \"provider_id\": \"$provider_id\",
    \"type\": \"provider\",
    \"description\": \"API Key for $PROVIDER_USERNAME tunnel test\"
}")

provider_api_key=$(echo "$api_key_response" | jq -r '.api_key')
if [ "$provider_api_key" = "null" ] || [ -z "$provider_api_key" ]; then
    echo "Failed to create API key or extract key"
    echo "Response: $api_key_response"
    exit 1
fi
echo "API Key created successfully: ${provider_api_key:0:20}..."

# Request Cloudflare tunnel
echo "Requesting Cloudflare tunnel..."
tunnel_response=$(curl -s --location "$CLOUDFLARE_URL/api/cloudflare/tunnel/request" \
    --header 'Content-Type: application/json' \
    --header "Authorization: Bearer $provider_api_key" \
    --data '{
        "service_url": "http://localhost:11434"
    }')

# Check if we got a valid JSON response with tunnel info
if echo "$tunnel_response" | jq -e '.token' >/dev/null 2>&1; then
    tunnel_token=$(echo "$tunnel_response" | jq -r '.token')
    hostname=$(echo "$tunnel_response" | jq -r '.hostname')
    
    echo "Tunnel created successfully!"
    echo "Hostname: $hostname"
    echo "Token: ${tunnel_token:0:20}..."
    
    # Save tunnel info to file
    cat > /app/tunnel_info.json << EOF
{
    "provider_username": "$PROVIDER_USERNAME",
    "user_id": "$user_id",
    "provider_id": "$provider_id",
    "api_key": "$provider_api_key",
    "tunnel_token": "$tunnel_token",
    "hostname": "$hostname"
}
EOF
    
    echo "Tunnel info saved to /app/tunnel_info.json"
else
    echo "Failed to create tunnel"
    echo "Response: $tunnel_response"
    exit 1
fi

echo "========================================="
echo "Starting Cloudflare Tunnel"
echo "Provider: $PROVIDER_USERNAME"
echo "Hostname: $hostname"
echo "Token: ${tunnel_token:0:20}..."
echo "========================================="

# Create a simple HTTP server to test the tunnel
echo "Starting test HTTP server on port 11434..."
{
    while true; do
        echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\n\r\n<h1>Tunnel Test - $PROVIDER_USERNAME</h1><p>Container: $(hostname)</p><p>Time: $(date)</p><p>Tunnel working!</p>" | nc -l -p 11434
    done
} &

HTTP_SERVER_PID=$!

# Function to cleanup on exit
cleanup() {
    echo "Shutting down..."
    kill $HTTP_SERVER_PID 2>/dev/null || true
    exit 0
}

trap cleanup SIGTERM SIGINT

# Start cloudflared tunnel
echo "Running: cloudflared tunnel run --token $tunnel_token"
cloudflared tunnel run --token "$tunnel_token" &
TUNNEL_PID=$!

# Monitor both processes
while true; do
    if ! kill -0 $TUNNEL_PID 2>/dev/null; then
        echo "ERROR: Cloudflare tunnel process died!"
        break
    fi
    
    if ! kill -0 $HTTP_SERVER_PID 2>/dev/null; then
        echo "ERROR: HTTP server process died!"
        break
    fi
    
    echo "Tunnel status: RUNNING ($(date))"
    sleep 30
done

cleanup 