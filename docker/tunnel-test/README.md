# Cloudflare Tunnel Testing Docker Setup

This directory contains Docker configuration to test multiple Cloudflare tunnels simultaneously.

## Quick Start

From the project root directory:

```bash
# Start 5 tunnel containers
./scripts/launch_tunnel_test.sh

# Monitor in real-time
./scripts/monitor_tunnels.sh
```

## What Each Container Does

1. **Creates a provider user** via your auth API
2. **Gets an API key** for that provider
3. **Requests a Cloudflare tunnel** via your tunnel API  
4. **Runs cloudflared** with the tunnel token
5. **Hosts a test HTTP server** to verify connectivity

## Files in This Directory

- `Dockerfile` - Container definition with cloudflared + scripts
- `create_user_and_tunnel.sh` - User creation and tunnel setup logic
- `run_tunnel.sh` - Main container entry point
- `docker-compose.yml` - Generated dynamically by launch script

## Container Configuration

### Environment Variables

- `BASE_URL` - Auth service URL (default: `http://auth:8081`)
- `CLOUDFLARE_URL` - API service URL (default: `http://localhost`) 
- `USERNAME` - Provider username (auto-generated if not set)

### Ports

- `11434` - Internal HTTP test server

### Dependencies

Containers require your inferoute services:
- `auth` service on port 8081
- `api` service accessible at localhost
- Database connectivity

## Usage Examples

### Build and Run Single Container

```bash
# Build the image
docker build -t tunnel-test .

# Run single container
docker run -e USERNAME=test_user_1 tunnel-test
```

### Scale with Docker Compose

```bash
# Generate compose file for 10 containers
cd ../../
./scripts/launch_tunnel_test.sh -c 10 start

# Check status
docker-compose -f docker/tunnel-test/docker-compose.yml ps

# View logs
docker-compose -f docker/tunnel-test/docker-compose.yml logs -f

# Stop all
docker-compose -f docker/tunnel-test/docker-compose.yml down
```

### Debug Container

```bash
# Access running container
docker exec -it tunnel-test-tunnel-test-1 /bin/bash

# Check tunnel info
docker exec tunnel-test-tunnel-test-1 cat /app/tunnel_info.json

# View container logs
docker logs tunnel-test-tunnel-test-1
```

## Expected Container Behavior

### Startup Sequence
1. Waits for auth/api services to be ready
2. Creates unique provider user
3. Creates provider entity and API key
4. Requests Cloudflare tunnel
5. Starts cloudflared daemon
6. Runs HTTP test server
7. Reports status every 30 seconds

### Success Indicators
- Container logs show "Tunnel status: RUNNING"
- Tunnel hostname is accessible via HTTPS
- HTTP test server responds on port 11434

### Failure Modes
- **Service unreachable**: Auth/API services not running
- **API errors**: User/tunnel creation fails
- **Tunnel connection**: cloudflared can't connect to Cloudflare
- **Token issues**: Invalid or expired tunnel token

## Logs and Debugging

### Container Logs Format
```
Starting tunnel setup for user: tunnel_test_X
Creating provider user...
User ID: 12345678-1234-1234-1234-123456789012
Creating provider entity...
Provider ID: 87654321-4321-4321-4321-210987654321
Creating provider API key...
API Key created successfully
Requesting Cloudflare tunnel...
Tunnel created successfully!
Hostname: tunnel-test-x-provider.dev.infer.bid
Starting Cloudflare Tunnel...
Tunnel status: RUNNING (Thu Oct 26 12:00:00 UTC 2023)
```

### Persistent Data
Each container saves tunnel info to `/app/tunnel_info.json`:
```json
{
    "username": "tunnel_test_1",
    "user_id": "...",
    "provider_id": "...", 
    "api_key": "...",
    "tunnel_token": "...",
    "hostname": "..."
}
```

## Testing Strategy

### Start Small (5-10 containers)
```bash
./scripts/launch_tunnel_test.sh -c 5 start
```

### Scale Gradually
```bash
./scripts/launch_tunnel_test.sh scale 10
./scripts/launch_tunnel_test.sh scale 25
./scripts/launch_tunnel_test.sh scale 50
```

### Monitor Results
```bash
./scripts/monitor_tunnels.sh
```

Watch for:
- Container startup failures
- API rate limiting
- Tunnel connectivity issues
- Cloudflare account limits

## Cleanup

### Stop Containers
```bash
./scripts/launch_tunnel_test.sh clean
```

### Remove Test Data
Test users remain in your database. Clean up manually if needed:
```sql
DELETE FROM users WHERE username LIKE 'tunnel_test_%';
```

### Remove Docker Resources
```bash
docker system prune -f
```

## Troubleshooting

### Container Won't Start
- Check if auth/api services are running
- Verify network connectivity: `docker network ls`
- Check service health: `curl http://localhost:8081/health`

### Tunnel Creation Fails
- Verify Cloudflare API key in your services
- Check API service logs for errors
- Test tunnel creation manually with curl

### Tunnel Connects But Not Reachable
- Wait 1-2 minutes for DNS propagation
- Check cloudflared logs in container
- Verify Cloudflare dashboard shows tunnel as connected

### High Resource Usage
- Monitor container memory: `docker stats`
- Check system resources: `htop` or `docker system df`
- Consider reducing container count

### Check Service Health

```bash
# Verify auth service
curl http://localhost:8081/health

# Verify API service  
curl http://localhost/health

# Test tunnel creation manually
curl -X POST http://localhost/api/cloudflare/tunnel/request \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"service_url": "http://localhost:11434"}'
```

For more detailed documentation, see the main `README_TUNNEL_TEST.md` in the project root. 