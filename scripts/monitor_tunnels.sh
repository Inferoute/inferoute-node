#!/bin/bash

set -e

COMPOSE_FILE="docker/tunnel-test/docker-compose.yml"
MONITOR_INTERVAL=30
LOG_FILE="tunnel_monitor.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

# Check if compose file exists
if [ ! -f "$COMPOSE_FILE" ]; then
    echo -e "${RED}Error: No tunnel containers running. Use './scripts/launch_tunnel_test.sh start' first.${NC}"
    exit 1
fi

# Change to project root
cd "$(dirname "$0")/.."

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Cloudflare Tunnel Monitor Started${NC}"
echo -e "${BLUE}========================================${NC}"
log "Monitor started"

# Function to get container status
get_container_status() {
    local container_name=$1
    local status=$(docker inspect --format='{{.State.Status}}' "$container_name" 2>/dev/null || echo "not_found")
    echo "$status"
}

# Function to test tunnel connectivity
test_tunnel_connectivity() {
    local container_name=$1
    local hostname=""
    
    # Extract hostname from container logs
    hostname=$(docker logs "$container_name" 2>/dev/null | grep "Hostname:" | tail -1 | awk '{print $2}' || echo "")
    
    if [ -n "$hostname" ]; then
        # Test if the tunnel is accessible
        if curl -s --max-time 10 "https://$hostname" > /dev/null 2>&1; then
            echo "CONNECTED"
        else
            echo "UNREACHABLE"
        fi
    else
        echo "NO_HOSTNAME"
    fi
}

# Function to get tunnel info from container
get_tunnel_info() {
    local container_name=$1
    
    # Try to extract tunnel info from container
    local info=$(docker exec "$container_name" cat /app/tunnel_info.json 2>/dev/null || echo "{}")
    
    if [ "$info" != "{}" ]; then
        local username=$(echo "$info" | jq -r '.username // "unknown"')
        local hostname=$(echo "$info" | jq -r '.hostname // "unknown"')
        echo "$username|$hostname"
    else
        echo "unknown|unknown"
    fi
}

# Main monitoring loop
monitor_loop() {
    local total_containers=0
    local running_containers=0
    local connected_tunnels=0
    local failed_containers=0
    
    echo -e "\n${YELLOW}Scanning containers...${NC}"
    
    # Get list of tunnel containers
    local containers=$(docker-compose -f "$COMPOSE_FILE" ps -q 2>/dev/null || echo "")
    
    if [ -z "$containers" ]; then
        echo -e "${RED}No containers found${NC}"
        return 1
    fi
    
    # Header for status table
    printf "${BLUE}%-20s %-15s %-30s %-50s %-12s${NC}\n" "CONTAINER" "STATUS" "USERNAME" "HOSTNAME" "CONNECTIVITY"
    printf "%.120s\n" "$(printf '=%.0s' {1..120})"
    
    # Check each container
    for container_id in $containers; do
        local container_name=$(docker inspect --format='{{.Name}}' "$container_id" | sed 's/^.//')
        local status=$(get_container_status "$container_name")
        local info=$(get_tunnel_info "$container_name")
        local username=$(echo "$info" | cut -d'|' -f1)
        local hostname=$(echo "$info" | cut -d'|' -f2)
        local connectivity=""
        
        total_containers=$((total_containers + 1))
        
        if [ "$status" = "running" ]; then
            running_containers=$((running_containers + 1))
            connectivity=$(test_tunnel_connectivity "$container_name")
            
            if [ "$connectivity" = "CONNECTED" ]; then
                connected_tunnels=$((connected_tunnels + 1))
                printf "${GREEN}%-20s %-15s %-30s %-50s %-12s${NC}\n" "$container_name" "$status" "$username" "$hostname" "$connectivity"
            else
                printf "${YELLOW}%-20s %-15s %-30s %-50s %-12s${NC}\n" "$container_name" "$status" "$username" "$hostname" "$connectivity"
            fi
        else
            failed_containers=$((failed_containers + 1))
            printf "${RED}%-20s %-15s %-30s %-50s %-12s${NC}\n" "$container_name" "$status" "$username" "$hostname" "FAILED"
        fi
    done
    
    # Summary
    echo ""
    printf "${BLUE}%-20s: %d${NC}\n" "Total Containers" "$total_containers"
    printf "${GREEN}%-20s: %d${NC}\n" "Running" "$running_containers"
    printf "${GREEN}%-20s: %d${NC}\n" "Connected Tunnels" "$connected_tunnels"
    printf "${RED}%-20s: %d${NC}\n" "Failed" "$failed_containers"
    
    # Log summary
    log "Status: Total=$total_containers, Running=$running_containers, Connected=$connected_tunnels, Failed=$failed_containers"
    
    # Check for issues
    if [ "$failed_containers" -gt 0 ]; then
        log "WARNING: $failed_containers containers have failed"
    fi
    
    local disconnected=$((running_containers - connected_tunnels))
    if [ "$disconnected" -gt 0 ]; then
        log "WARNING: $disconnected running containers are not connected"
    fi
}

# Handle interruption
cleanup() {
    echo -e "\n${YELLOW}Monitor stopped${NC}"
    log "Monitor stopped"
    exit 0
}

trap cleanup SIGTERM SIGINT

# Monitor continuously
while true; do
    clear
    echo -e "${BLUE}Cloudflare Tunnel Monitor - $(date)${NC}"
    echo -e "${BLUE}Log file: $LOG_FILE${NC}"
    echo -e "${BLUE}Press Ctrl+C to stop${NC}"
    
    monitor_loop
    
    echo -e "\n${YELLOW}Next update in $MONITOR_INTERVAL seconds...${NC}"
    sleep $MONITOR_INTERVAL
done 