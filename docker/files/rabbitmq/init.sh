#!/bin/sh

# Wait for RabbitMQ to start
until rabbitmqctl await_startup; do
    echo "Waiting for RabbitMQ to start..."
    sleep 2
done

# Create queues
rabbitmqadmin declare queue name=provider_health_updates durable=true
rabbitmqadmin declare queue name=transactions_queue durable=true

# Create exchanges
rabbitmqadmin declare exchange name=provider_health type=topic durable=true
rabbitmqadmin declare exchange name=transactions_exchange type=topic durable=true

# Create bindings
rabbitmqadmin declare binding source=provider_health destination=provider_health_updates destination_type=queue routing_key="provider.health.updates"
rabbitmqadmin declare binding source=transactions_exchange destination=transactions_queue destination_type=queue routing_key="transactions"

echo "RabbitMQ initialization completed" 