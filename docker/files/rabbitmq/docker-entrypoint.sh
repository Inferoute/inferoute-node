#!/bin/sh
set -e

# Start RabbitMQ in the background
docker-entrypoint.sh rabbitmq-server &

# Wait for RabbitMQ to be ready
sleep 10

# Run initialization script
/usr/local/bin/init.sh

# Keep container running
wait $! 