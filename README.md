# inferoute-node
Inferoute Node Application 

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.21 or later

##### Production 
- WireGuard configured with proper IP (for CockroachDB cluster)

##### Development
- none 

### 1. Create Network
```bash
docker network create --subnet=172.18.0.0/16 inferoute-net
```

### 2. Build containers

#### Development
```bash
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               build
```

#### Production
```bash
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               build
```

### 3. Initialize CockroachDB:

```bash
# Start CockroachDB
# Development
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               up -d cockroachdb

# Production
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               up -d cockroachdb

# Initialize cluster
docker exec -i cockroachdb cockroach init --insecure

# Import the schema
docker exec -i cockroachdb cockroach sql --insecure < schema.sql
docker exec -i cockroachdb cockroach sql --insecure -d inferoute < seed.sql

# Verify the database is running
docker exec -i cockroachdb cockroach sql --insecure -e "SHOW DATABASES;"
```

### 4. Initialize RabbitMQ

```bash
# Start RabbitMQ
# Development
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               up -d rabbitmq

# Production
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               up -d rabbitmq

# Create users
docker exec rabbitmq rabbitmqctl add_user inferoute Nightshade900! && \
docker exec rabbitmq rabbitmqctl set_permissions -p / inferoute ".*" ".*" ".*" && \
docker exec rabbitmq rabbitmqctl set_user_tags inferoute administrator

# Create queues and exchanges
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare exchange name=provider_health type=topic durable=true && \
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare exchange name=transactions_exchange type=topic durable=true && \
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare queue name=provider_health_updates durable=true && \
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare queue name=transactions_queue durable=true && \
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare binding source=provider_health destination=provider_health_updates routing_key="provider.health.updates" && \
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare binding source=transactions_exchange destination=transactions_queue routing_key="transactions"

# Confirm queues exist
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! list queues && \
echo "=== Exchanges ===" && \
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! list exchanges
```

### 5. Start remaining containers:

```bash
# Development
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               --profile development up -d

# Production
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               --profile production up -d
```

### 6. Verify services:
```bash
# Check all container statuses
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env ps

# Check Traefik routes
curl -s localhost:8080/api/http/routers | jq

# Access RabbitMQ management UI
open http://localhost:15672  # Default credentials: inferoute/Nightshade900!
```

### Troubleshooting

#### RabbitMQ Issues
If RabbitMQ fails to start, check the logs:
```bash
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               logs rabbitmq
```

#### CockroachDB Issues
Check cluster status:
```bash
docker exec -i cockroachdb cockroach node status --insecure
```

### Test Payment Processor
```bash
# Build and run test
go build -o bin/test_payment_processor test_payment_processor.go
./bin/test_payment_processor
```

### To bring things down 

## Develpoment 

# First bring everything down
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               --profile development down 

# Then bring it back up
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               --profile development up -d


## Production 

# First bring everything down
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               --profile production down 

# Then bring it back up
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               --profile production up -d


### Client for Providers
Provider nodes need:
- Nginx server with API key validation and HMAC verification
- Health status reporting (via cron)
- Clients can implement their own solution using nginx and cron

# Create a new user
rabbitmqctl add_user inferoute Nightshade900!

# Give it permissions (configure, write, read) on all resources
rabbitmqctl set_permissions -p / inferoute ".*" ".*" ".*"

# Make it an administrator (optional, if you need admin access)
rabbitmqctl set_user_tags inferoute administrator

http://localhost:15672


### Start cockcroachdb




FROm DEV to PROD:


### Test Payment processor speed to process:
1. run start_services.sh
2. run test_payment_processor.go

go build -o bin/test_payment_processor test_payment_processor.go
./bin/test_payment_processor

### Client for providers

- Needs to have Nginx server with ability to send us their API key  and validate HMAC before sending request to us.
- Need to have a way to push health status to us, use cron for now.
- Clients can roll their own using nginx and cron.
- 



#### Docker start