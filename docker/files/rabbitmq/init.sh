#!/bin/sh

# Wait for RabbitMQ to start
until rabbitmqctl await_startup; do
    echo "Waiting for RabbitMQ to start..."
    sleep 2
done

# Create queues and set them as durable
rabbitmqctl set_parameter policy provider_health_updates '{"durable": true}'
rabbitmqctl set_parameter policy transactions_queue '{"durable": true}'

# Create exchanges
rabbitmqctl set_parameter policy provider_health '{"type": "topic", "durable": true}'
rabbitmqctl set_parameter policy transactions_exchange '{"type": "topic", "durable": true}'

# Create bindings
rabbitmqctl set_parameter policy provider_health_binding '{"source": "provider_health", "destination": "provider_health_updates", "routing_key": "provider.health.updates"}'
rabbitmqctl set_parameter policy transactions_binding '{"source": "transactions_exchange", "destination": "transactions_queue", "routing_key": "transactions"}'

echo "RabbitMQ initialization completed" 