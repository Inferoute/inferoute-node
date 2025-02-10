# inferoute-node
Inferoute Node Application 



### Start all services,


### Start rabbitmq and create user

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