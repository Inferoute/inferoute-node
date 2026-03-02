#!/bin/bash

set -e

echo "=== DEBUG TUNNEL SCRIPT ==="
echo "Container ID: $(hostname)"

# Configuration
BASE_URL="${BASE_URL:-http://auth:8081}"
CLOUDFLARE_URL="${CLOUDFLARE_URL:-http://host.docker.internal}"
PROVIDER_USERNAME="${USERNAME:-debug_provider_$(date +%s)}"

echo "Testing auth service connectivity..."
if curl -s --max-time 5 "$BASE_URL/health" > /dev/null; then
    echo "✅ Auth service is reachable"
else
    echo "❌ Auth service is not reachable"
    exit 1
fi

echo "Testing API service connectivity..."
if curl -s --max-time 5 "$CLOUDFLARE_URL/health" > /dev/null; then
    echo "✅ API service is reachable"
else
    echo "❌ API service is not reachable"
    exit 1
fi

echo "Creating user: $PROVIDER_USERNAME"
echo "Making request to: $BASE_URL/api/auth/users"

response=$(curl -s -w "HTTPSTATUS:%{http_code}" --max-time 10 \
    --location "$BASE_URL/api/auth/users" \
    --header 'Content-Type: application/json' \
    --header 'X-Internal-Key: klondikee' \
    --data "{\"username\": \"$PROVIDER_USERNAME\"}")

echo "Raw response: $response"

http_code=$(echo "$response" | grep -o "HTTPSTATUS:[0-9]*" | cut -d: -f2)
body=$(echo "$response" | sed 's/HTTPSTATUS:[0-9]*$//')

echo "HTTP Code: $http_code"
echo "Response Body: $body"

if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
    user_id=$(echo "$body" | jq -r '.user.id')
    echo "✅ User created successfully with ID: $user_id"
else
    echo "❌ User creation failed"
    exit 1
fi

echo "=== DEBUG COMPLETED SUCCESSFULLY ===" 