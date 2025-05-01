#!/bin/sh

# Check if usernames are provided
if [ $# -ne 2 ]; then
    echo "Usage: $0 <consumer_username> <provider_username>"
    echo "Example: $0 test_consumer test_provider"
    exit 1
fi

CONSUMER_USERNAME=$1
PROVIDER_USERNAME=$2
MAX_RETRIES=5
BASE_URL="http://auth:8081"  # For actual API calls


# Function to make API calls with retries
make_request() {
    local url=$1
    local data=$2
    local retry_count=0
    
    echo "\nMaking request to: $url"
    echo "Request body: $data"
    
    while [ $retry_count -lt $MAX_RETRIES ]; do
        response=$(curl -s --location "$url" \
            --header 'Content-Type: application/json' \
            --header 'X-Internal-Key: klondikee' \
            --data "$data")
            
        # Check if response contains error about missing auth header or not found
        if echo "$response" | grep -q "missing authorization header\|Not Found"; then
            retry_count=$((retry_count + 1))
            echo "Attempt $retry_count/$MAX_RETRIES failed. Retrying in 2 seconds..."
            sleep 2
            continue
        fi
        
        # If we get here, we have a valid response
        echo "\nServer response:"
        echo "$response" | jq '.' || echo "$response"
        echo "----------------------------------------"
        sleep 1
        echo "$response"
        return
    done
    
    echo "Failed after $MAX_RETRIES attempts"
    exit 1
}


# Create first user (consumer)
echo "\nCreating consumer user ($CONSUMER_USERNAME)..."
consumer_user_response=$(make_request "$BASE_URL/api/auth/users" "{
    \"username\": \"$CONSUMER_USERNAME\"
}")
consumer_user_id=$(echo "$consumer_user_response" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
echo "Consumer User ID: $consumer_user_id"

# Create consumer entity
echo "\nCreating consumer entity..."
consumer_entity_response=$(make_request "$BASE_URL/api/auth/entities" "{
    \"user_id\": \"$consumer_user_id\",
    \"type\": \"consumer\",
    \"name\": \"$CONSUMER_USERNAME\"
}")
consumer_id=$(echo "$consumer_entity_response" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
echo "Consumer ID: $consumer_id"

# Create consumer API key
echo "\nCreating consumer API key..."
consumer_api_response=$(make_request "$BASE_URL/api/auth/api-keys" "{
    \"user_id\": \"$consumer_user_id\",
    \"consumer_id\": \"$consumer_id\",
    \"type\": \"consumer\",
    \"description\": \"API Key for $CONSUMER_USERNAME\"
}")

# Create second user (provider)
echo "\nCreating provider user ($PROVIDER_USERNAME)..."
provider_user_response=$(make_request "$BASE_URL/api/auth/users" "{
    \"username\": \"$PROVIDER_USERNAME\"
}")
provider_user_id=$(echo "$provider_user_response" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
echo "Provider User ID: $provider_user_id"

# Create provider entity
echo "\nCreating provider entity..."
provider_entity_response=$(make_request "$BASE_URL/api/auth/entities" "{
    \"user_id\": \"$provider_user_id\",
    \"type\": \"provider\",
    \"name\": \"$PROVIDER_USERNAME\",
    \"api_url\": \"https://api.provider.com\"
}")
provider_id=$(echo "$provider_entity_response" | grep -o '"id":"[^"]*' | cut -d'"' -f4)
echo "Provider ID: $provider_id"

# Create provider API key
echo "\nCreating provider API key..."
provider_api_response=$(make_request "$BASE_URL/api/auth/api-keys" "{
    \"user_id\": \"$provider_user_id\",
    \"provider_id\": \"$provider_id\",
    \"type\": \"provider\",
    \"description\": \"API Key for $PROVIDER_USERNAME\"
}")

# Print summary of all created resources
echo "\n=== SUMMARY OF CREATED RESOURCES ==="
echo "----------------------------------------"
echo "CONSUMER ($CONSUMER_USERNAME):"
echo "  User ID: $consumer_user_id"
echo "  Consumer ID: $consumer_id"
echo "  API Key Response:"
echo "$consumer_api_response" | jq '.' || echo "$consumer_api_response"
echo "\nPROVIDER ($PROVIDER_USERNAME):"
echo "  User ID: $provider_user_id"
echo "  Provider ID: $provider_id"
echo "  API Key Response:"
echo "$provider_api_response" | jq '.' || echo "$provider_api_response"
echo "----------------------------------------" 