# Docker Compose Configuration

This directory contains the Docker Compose configuration files for the Inferoute Node project. The configuration is split into three files to support different environments:

## File Structure

- `docker-compose.yml`: Base configuration shared across all environments
- `docker-compose.dev.yml`: Development-specific overrides (HTTP only)
- `docker-compose.prod.yml`: Production-specific overrides (HTTPS with Let's Encrypt)

## Usage

### Development Environment

```bash
# Start the development environment
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               --profile development up -d

# Stop the development environment
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.dev.yml \
               --env-file docker/env/development.env \
               --profile development down
```

### Production Environment

```bash
# Start the production environment
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               --profile production up -d

# Stop the production environment
docker compose -f docker/compose/docker-compose.yml \
               -f docker/compose/docker-compose.prod.yml \
               --env-file docker/env/production.env \
               --profile production down
```

## Environment Variables

Required environment variables for each environment should be defined in the corresponding `.env` file:

- Development: `docker/env/development.env`
- Production: `docker/env/production.env`

### Common Variables

- `RABBITMQ_USER`: RabbitMQ username
- `RABBITMQ_PASS`: RabbitMQ password
- `COCKROACH_USER`: CockroachDB username
- `COCKROACH_PASS`: CockroachDB password
- `COCKROACH_DB`: CockroachDB database name

## Services

The following services are available:

- **Traefik**: Reverse proxy and load balancer
  - Development: HTTP on port 80
  - Production: HTTPS on port 443 with Let's Encrypt
- **Orchestrator**: Main service orchestration
- **Auth**: Authentication service
- **Provider Management**: Provider management service
- **Provider Communication**: Provider communication service
- **Provider Health**: Provider health monitoring
- **Model Pricing**: Model pricing service
- **Scheduler**: Task scheduler
- **Payment Processing**: Payment processing service
- **CockroachDB**: Database service
- **RabbitMQ**: Message broker
- **API Testing**: Testing utilities

## Network

All services are connected to the `inferoute-net` network.

## Volumes

- `cockroach-data`: Persistent storage for CockroachDB
- `rabbitmq-data`: Persistent storage for RabbitMQ
- `traefik-certificates`: SSL certificates for production environment 