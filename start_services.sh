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

# Function to compile a service
compile_service() {
    local service_name=$1
    local binary_path=$2
    local source_path=$3
    
    echo -e "${GREEN}Building $service_name...${NC}"
    go build -o $binary_path $source_path || handle_error "Failed to build $service_name"
}

# Function to start a service
start_service() {
    local service_name=$1
    local binary_path=$2
    
    echo -e "${GREEN}Starting $service_name...${NC}"
    $binary_path &
    echo $! > bin/$service_name.pid
}

# Kill existing processes if they exist
for pid_file in bin/*.pid; do
    if [ -f "$pid_file" ]; then
        kill $(cat "$pid_file") 2>/dev/null
        rm "$pid_file"
    fi
done

# List of services
services=(
    "auth"
    "provider-management"
    "provider-health"
    "provider-communication"
    "payment-processing"
    "orchestrator"
    "model-pricing"
    "scheduler"
)

# First compile all services
echo -e "${GREEN}Compiling all services...${NC}"
for service in "${services[@]}"; do
    compile_service "$service" "bin/$service" "cmd/$service/main.go"
done

# Then start all services
echo -e "${GREEN}Starting all services...${NC}"
for service in "${services[@]}"; do
    start_service "$service" "bin/$service"
done

echo -e "${GREEN}All services started successfully!${NC}"
echo "To stop services, run: kill \$(cat bin/*.pid)"

# Wait for all services
wait