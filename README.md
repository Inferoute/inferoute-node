# inferoute-node
Inferoute Node Application 

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.21 or later

##### Production 
- WireGuard configured with proper IP (for CockroachDB cluster) Or if 

##### Development
- none 


### 1. Build containers

### Development
docker compose --env-file docker/env/development.env build

### Production
docker compose --env-file docker/env/production.env build

#### Build a single container

### 2. Create configuration 


1. docker network create --subnet=172.18.0.0/16 inferoute-net  
2. Check development or production env files are correct.

 

### 3. Initialize CockroachDB:

## Start cockcroachdb 

```bash
docker compose --env-file docker/env/development.env up -d cockroachdb
```

```bash

# Initialise cluster
docker exec -i cockroachdb cockroach init --insecure

# Import the schema
docker exec -i cockroachdb cockroach sql --insecure < schema.sql
docker exec -i cockroachdb cockroach sql --insecure -d inferoute < seed.sql
# Verify the database is running
docker exec -i cockroachdb cockroach sql --insecure -e "SHOW DATABASES;"
```


### 4. Initialise Rabbitmq

## Start rabbitmq
```bash
docker compose --env-file docker/env/development.env up -d rabbitmq 
```

## Create users
```bash
docker exec rabbitmq rabbitmqctl add_user inferoute Nightshade900! && docker exec rabbitmq rabbitmqctl set_permissions -p / inferoute ".*" ".*" ".*" && docker exec rabbitmq rabbitmqctl set_user_tags inferoute administrator
```

## Create queus
```bash
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare exchange name=provider_health type=topic durable=true && docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare exchange name=transactions_exchange type=topic durable=true && docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare queue name=provider_health_updates durable=true && docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare queue name=transactions_queue durable=true && docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare binding source=provider_health destination=provider_health_updates routing_key="provider.health.updates" && docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! declare binding source=transactions_exchange destination=transactions_queue routing_key="transactions"
```

### Conirm queues exist

```bash
docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! list queues && echo "=== Exchanges ===" && docker exec rabbitmq rabbitmqadmin --username=inferoute --password=Nightshade900! list exchanges
```


### 5. Start remaining  containers:

```bash
docker compose --env-file docker/env/development.env up 
```

4. Verify services are running:
```bash
# Check all container statuses
docker compose ps

# Check nginx health
curl http://localhost/health

# Access RabbitMQ management UI
open http://localhost:15672  # Default credentials: inferoute/Nightshade900!

# Check traefik routes

curl -s localhost:8080/api/http/routers | jq
```

### Troubleshooting

#### RabbitMQ Issues
If RabbitMQ fails to start, check the logs:
```bash
docker compose logs rabbitmq
```


Eventually for proudction 
#### CockroachDB Issues - WE njeed to creaet certificates and make it secure
Check cluster status:
```bash
docker exec -i inferoute-node-cockroachdb-1 cockroach node status --insecure
```

```

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