# inferoute-node
Inferoute Node Application 

## Quick Start

### Prerequisites
- Docker and Docker Compose
- Go 1.21 or later
- WireGuard configured with proper IP (for CockroachDB cluster)

### Start all services

1. First, create a `.env` file in the root directory with your configuration (see `.env.example`)

2. Start all containers:
```bash
docker compose up -d
```

3. Initialize CockroachDB:
```bash
# Import the schema
docker exec -i inferoute-node-cockroachdb-1 cockroach sql --insecure < schema.sql

# Verify the database is running
docker exec -i inferoute-node-cockroachdb-1 cockroach sql --insecure -e "SHOW DATABASES;"
```

4. Verify services are running:
```bash
# Check all container statuses
docker compose ps

# Check nginx health
curl http://localhost/health

# Access RabbitMQ management UI
open http://localhost:15672  # Default credentials: inferoute/Nightshade900!
```

### Troubleshooting

#### RabbitMQ Issues
If RabbitMQ fails to start, check the logs:
```bash
docker compose logs rabbitmq
```

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