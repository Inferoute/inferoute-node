#!/bin/bash

# Create bin directory if it doesn't exist
mkdir -p bin

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to handle errors
handle_error() {
    echo -e "${RED}Error: $1${NC}"
    exit 1
}

# Function to start a service
start_service() {
    local service_name=$1
    local binary_path=$2
    local source_path=$3
    
    echo -e "${GREEN}Building $service_name...${NC}"
    go build -o $binary_path $source_path || handle_error "Failed to build $service_name"
    
    echo -e "${GREEN}Starting $service_name...${NC}"
    $binary_path &
    echo $! > bin/$service_name.pid
}

# Kill existing processes if they exist
if [ -f bin/provider-management.pid ]; then
    kill $(cat bin/provider-management.pid) 2>/dev/null
    rm bin/provider-management.pid
fi

if [ -f bin/auth-service.pid ]; then
    kill $(cat bin/auth-service.pid) 2>/dev/null
    rm bin/auth-service.pid
fi

if [ -f bin/provider-health.pid ]; then
    kill $(cat bin/provider-health.pid) 2>/dev/null
    rm bin/provider-health.pid
fi

# Start services
start_service "provider-management" "bin/provider-management" "cmd/provider-management/main.go"
start_service "auth-service" "bin/auth-service" "cmd/auth/main.go"
start_service "provider-health" "bin/provider-health" "cmd/provider-health/main.go"

echo -e "${GREEN}Services started successfully!${NC}"
echo "To stop services, run: kill \$(cat bin/*.pid)"

# Wait for all services
wait