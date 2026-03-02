#!/bin/bash

set -e

# Configuration
DEFAULT_COUNT=5
DEFAULT_ACTION="start"
COMPOSE_FILE="docker/tunnel-test/docker-compose.yml"

# Help function
show_help() {
    cat << EOF
Cloudflare Tunnel Test Launcher

Usage: $0 [OPTIONS] [ACTION]

OPTIONS:
    -c, --count NUM     Number of tunnel containers to launch (default: $DEFAULT_COUNT)
    -h, --help         Show this help message

ACTIONS:
    start              Start tunnel containers (default)
    stop               Stop tunnel containers
    restart            Restart tunnel containers
    logs               Show logs from all tunnel containers
    status             Show status of tunnel containers
    clean              Stop and remove containers
    scale NUM          Scale to specific number of containers

Examples:
    $0                          # Start 5 tunnel containers
    $0 -c 10 start             # Start 10 tunnel containers
    $0 stop                    # Stop all tunnel containers
    $0 logs                    # Show logs
    $0 scale 15                # Scale to 15 containers
    $0 clean                   # Clean up everything

EOF
}

# Parse arguments
COUNT=$DEFAULT_COUNT
ACTION=$DEFAULT_ACTION

while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--count)
            COUNT="$2"
            shift 2
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        start|stop|restart|logs|status|clean)
            ACTION="$1"
            shift
            ;;
        scale)
            ACTION="scale"
            COUNT="$2"
            shift 2
            ;;
        *)
            if [[ "$1" =~ ^[0-9]+$ ]]; then
                COUNT="$1"
            else
                echo "Unknown option: $1"
                show_help
                exit 1
            fi
            shift
            ;;
    esac
done

# Validate count
if ! [[ "$COUNT" =~ ^[0-9]+$ ]] || [ "$COUNT" -lt 1 ] || [ "$COUNT" -gt 100 ]; then
    echo "Error: Count must be a number between 1 and 100"
    exit 1
fi

# Generate docker-compose.yml
generate_compose_file() {
    local count=$1
    local compose_dir="docker/tunnel-test"
    
    mkdir -p "$compose_dir"
    
    cat > "$COMPOSE_FILE" << EOF
services:
EOF

    # Add tunnel test services
    for i in $(seq 1 $count); do
        cat >> "$COMPOSE_FILE" << EOF
  tunnel-test-$i:
    build: .
    environment:
      - BASE_URL=http://auth:8081
      - CLOUDFLARE_URL=http://host.docker.internal
      - USERNAME=tunnel_test_$i
    networks:
      - default
    restart: unless-stopped
    
EOF
    done

    cat >> "$COMPOSE_FILE" << EOF
networks:
  default:
    external: true
    name: inferoute-net
EOF

    echo "Generated compose file for $count containers"
}

# Change to project root
cd "$(dirname "$0")/.."

case $ACTION in
    start)
        echo "Starting $COUNT tunnel containers..."
        generate_compose_file $COUNT
        docker-compose -f "$COMPOSE_FILE" up -d
        echo "Started $COUNT tunnel containers"
        echo "Use '$0 logs' to see container logs"
        echo "Use '$0 status' to check container status"
        ;;
    
    stop)
        echo "Stopping tunnel containers..."
        if [ -f "$COMPOSE_FILE" ]; then
            docker-compose -f "$COMPOSE_FILE" stop
            echo "Stopped tunnel containers"
        else
            echo "No compose file found - containers may not be running"
        fi
        ;;
    
    restart)
        echo "Restarting tunnel containers..."
        if [ -f "$COMPOSE_FILE" ]; then
            docker-compose -f "$COMPOSE_FILE" restart
            echo "Restarted tunnel containers"
        else
            echo "No compose file found - use 'start' action first"
        fi
        ;;
    
    logs)
        if [ -f "$COMPOSE_FILE" ]; then
            docker-compose -f "$COMPOSE_FILE" logs -f
        else
            echo "No compose file found - containers may not be running"
        fi
        ;;
    
    status)
        if [ -f "$COMPOSE_FILE" ]; then
            echo "Tunnel container status:"
            docker-compose -f "$COMPOSE_FILE" ps
            echo ""
            echo "Summary:"
            running=$(docker-compose -f "$COMPOSE_FILE" ps -q | wc -l)
            echo "Total containers: $running"
        else
            echo "No compose file found - containers may not be running"
        fi
        ;;
    
    clean)
        echo "Cleaning up tunnel containers..."
        if [ -f "$COMPOSE_FILE" ]; then
            docker-compose -f "$COMPOSE_FILE" down -v
            echo "Cleaned up tunnel containers"
        else
            echo "No compose file found - nothing to clean"
        fi
        ;;
    
    scale)
        echo "Scaling to $COUNT tunnel containers..."
        generate_compose_file $COUNT
        docker-compose -f "$COMPOSE_FILE" up -d
        echo "Scaled to $COUNT tunnel containers"
        ;;
    
    *)
        echo "Unknown action: $ACTION"
        show_help
        exit 1
        ;;
esac 