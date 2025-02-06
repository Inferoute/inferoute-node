

NAMES:

Inferoute
Routeollama


# Services and Their Roles

All of these services listed below will run on a single server and are collectively called the central-node.

## 1.   Nginx (API Gateway) - TEST !!!

This is our main entry point for consumers. Consumers will connect using a universal API point (most likely this will be a cloudflare proxy) which will then point to this proxy.

- **Role:** The public entry point. Handles TLS termination, rate limiting, and basic routing. This is the proxy that all consumers and providers connect to (unless a provider is responding to)

-  **Flow:** Forwards incoming consumer requests to the Orchestrator and passes back responses to the consumer. Consumer request data and API-KEY (which will be in the AUTH-)

- NGINX servers takes in the request and will forward the request to Orchestrator if its a OpenAI API request.
- If it's not and its a general API request then it will forward the request to the relevant microservice.
	Like for example when a provider wants to verify an HMAC he will connect to our proxy and the proxy will forward it directly to the microservice.
- https://github.com/ollama/ollama/blob/main/docs/openai.md  NGINX should route based on the openai API standard.
- 

## 2.  Orchestrator (GO)

The main orchestrator of our application.

- **Role:** The central controller that coordinates the overall request workflow.

- **Responsibilities:**

	- Receives requests from Nginx.

	- Invokes the Authentication Service to validate the API key and check for sufficient funds using auth microservice -  /api/auth/validate

    - Generates HMAC(s) for the request using common/hmac.go

    - Creates a transaction record in the database - initial transaction status is pending. 

    - Initiates a holding deposit (deduct $1) for valid requests using auth microservice - /api/auth/hold

	- Queries the Provider Management Service for a list of healthy providers.

	- Calls the Provider Communication Service to dispatch the request (with HMAC) to a set of providers.

	- Receives the winning provider's response from the Provider Communication Service.

	- Forwards that response back to the Nginx proxy.

	- Initiates a release deposit using auth microservice - /api/auth/hold

	- Updates the transaction record in the database with the  total input tokens, total output tokens, latency and changes status to payment. This is so the payment processing service can pick it up. 
	also if anyhtig gets missed we can double check it using payment processing service. 

	- Publishes payment-related details to RabbitMQ for asynchronous processing by the Payment Processing Service. 

## 3.  Authentication Service (Go) - DONE !!!!

- **Role:** VManages user authentication, API key validation, and deposit handling. Ensures users have sufficient balance for operations and provides deposit hold/release functionality for transaction safety.

- **Endpoints (HTTP/JSON):**

POST /api/auth/users
	Create a new user account with API key
	Generates a unique API key for the user
	Sets initial deposit balance if provided
	Returns user ID and API key

POST /api/auth/validate
	- Validates API key and checks user's available balance
	- Ensures minimum balance requirement ($1.00)
	- Returns user details and available balance
	- Authentication: API key required in Bearer token
	- Response includes:
		- User ID
		- Available balance
		- Account status


POST /api/auth/hold
	- Places a temporary hold on user's deposit of $1
	- Used before processing transactions
	- Prevents double-spending
	- Authentication: API key required


POST /api/auth/release
	- Releases a previously held deposit of $1.00
	- Used after successful transaction completion
	- Authentication: API key required



## 4.  Provider Management Service (Go) - DONE!!!!!

 - **Role:** Provides APIs for providers to manage their models, availability status, and health reporting. Acts as the entry point for provider health updates which are then published to RabbitMQ for processing by the provider-health service

 ### The service integrates with:

 - RabbitMQ: Publishes health updates to "provider_health" exchange
 - Database: Stores model configurations and provider status
 - All endpoints require authentication via provider API key and include proper error handling with appropriate HTTP status codes and error messages

-  **Endpoints (HTTP/JSON):**

- POST /api/provider/models
	- Add a new model for the authenticated provider
	- New models are automatically set to is_active = true
	- Requires model name, service type (ollama/exolabs/llama_cpp), and token pricing
	- Authentication: Provider API key required

- GET /api/provider/models
	- List all models for the authenticated provider
	- Returns model configurations including active status and pricing
	- Authentication: Provider API key required

- PUT /api/provider/models/{model_id}
	- Update an existing model's configuration
	- Can modify model name, service type, and token pricing
	- Authentication: Provider API key required

- DELETE /api/provider/models/{model_id}
	- Remove a model from the provider's available models
	- Authentication: Provider API key required

- PUT /api/provider/pause
	- Update provider's pause status (true/false)
	- When paused, provider won't receive new requests
	- Authentication: Provider API key required

- POST /api/provider/health
	- Push provider's current health and model status
	- Accepts list of currently available models
	- Publishes update to RabbitMQ for processing by provider-health service
	- Authentication: Provider API key required in Bearer token

 - POST /api/provider/validate_hmac
     - Used by providers to validate HMACs
     - Requires provider API key authentication
     - Payload:
       - HMAC to validate
     - Returns:
       - Validation status
       - Original request data (if valid)
       - Transaction ID (if valid)


## 5.  Provider Communication Service (Go) - DONE!!!!

- **Role:** Dispatches consumer requests (with associated HMACs) to providers and collects responses. Also provides an API for providers to validate HMACs.

- **Architecture:**
  - Built using Echo framework for HTTP routing
  - Uses CockroachDB for transaction and HMAC validation
  - Implements graceful shutdown for clean service termination
  - Runs on port 8083 in development mode

- **Authentication:**
  - Skips authentication for `/api/send_requests` (used by orchestrator)
  - Requires provider API key in Bearer token for `/api/validate_hmac`
  - Validates API keys against the database in real-time

- **Key Components:**

  1. **Request Handler:**
     - Receives requests from the orchestrator
     - Validates provider existence and availability
     - Forwards requests to providers
     - Measures and tracks latency
     - Returns provider responses to orchestrator

  2. **HMAC Validator:**
     - Validates HMACs against the transaction table
     - Ensures HMAC is valid and transaction is pending
     - Returns associated request data to providers
     - Prevents replay attacks by checking transaction status

  3. **Error Handling:**
     - Custom error types for different scenarios
     - Proper HTTP status codes for each error case
     - Detailed error messages for debugging

- **Endpoints (HTTP/JSON):**

  1. **POST /api/send_requests**
     - Used by orchestrator to send requests to providers
     - No authentication required (internal endpoint)
     - Payload includes:
       - Provider ID
       - HMAC
       - Request data
       - Model name
     - Returns:
       - Success/failure status
       - Provider response data
       - Latency metrics

  2. **POST /api/validate_hmac**
     - Used by providers to validate HMACs
     - Requires provider API key authentication
     - Payload:
       - HMAC to validate
     - Returns:
       - Validation status
       - Original request data (if valid)
       - Transaction ID (if valid)

- **Flow:**
  1. Orchestrator sends request to `/api/send_requests`
  2. Service validates provider and prepares request
  3. Request is sent to provider's endpoint
  4. Provider validates HMAC using `/api/validate_hmac`
  5. Provider processes request and sends response
  6. Service forwards response back to orchestrator

- **Error Scenarios:**
  - Invalid provider ID
  - Provider not available
  - Invalid HMAC
  - Request timeout
  - Network failures
  - Invalid API keys

- **Monitoring:**
  - Latency tracking for provider responses
  - Request success/failure metrics
  - HMAC validation statistics
  - Provider availability monitoring


  CURRENT STATUS:

  Repsonse from provider is prited to console. We need to send to Orchestrator once build.


## 6.  Payment Processing Service (Go) with RabbitMQ

-  **Role:** Handles the asynchronous financial operations after the main workflow is complete.

-  **Workflow:** Consumes messages from a RabbitMQ queue, then debits the consumer, credits the provider, computes tokens per second, and releases the holding deposit.

- Once a transaction is completed the orchestrator should send a message to the payment service to process the transaction.
- The rabbitmq will include the following details:
	- Consumer ID
	- Provider ID
	- HMAC
	- Model name
	- Total input tokens
	- Total output tokens
	- Latency
	- status (which at this point should be set to payment)

- It is the job of the payment sevice to consume those rabitMQ messages and work out the following and update the transactions table:
	- Tokens per second (total output tokens / latency)
	- Consumer cost (total input tokens + total output tokens) * price per token ( each provider will have these prices attach to their models in the provider_models table)
	- Provider earnings - Consumer cost - 5%  (the 5% is the fee for the central node) - We ahouls also actually add another column to the transactions table to show the amount of the fee that is taken by our service. 



- **Technology:** Uses RabbitMQ for message queuing.

## 7.Consumer management service services (GO)

-  **Role:** Provides API to consumers.
	- **Consumers** can check their spend based on keys
	- **Providers** can check their profit based on models and time (need to implement in provider-management)


## 8. Provider-Health (GO) - DONE!!!!!!

**Role:** Processes health updates from providers via RabbitMQ, maintains provider health status, and provides APIs for health-related operations.
The service has several core components:

- **RabbitMQ Message Processing:**
	- Consumes health check messages from the "provider_health" exchange with "health_updates" routing key
	- For each health update:
		- Ignores models not in the database
		- Marks database models as inactive if not in health update
		- Updates provider health status (green/orange/red):
			- Green: All registered models are available
			- Orange: Some models are available
			- Red: No models are available
		- Updates last_health_check timestamp
		- Records health check in provider_health_history table

- **Health Status Management:**
Health status uses a traffic light system (green/orange/red) in both provider_status and provider_model tables
is_available field in provider_status is set to false when health status is red

- **Stale providers (no health check for 30+ minutes) can be marked unhealthy via API endpoint**

- **Provider Tier System:**
	- Providers are assigned tiers (1-3) based on 30-day health history:
		- Tier 1: 99%+ green status
		- Tier 2: 95-99% green status
		- Tier 3: <95% green status
	- Tier updates can be triggered via API endpoint

- **APIs:**

Note: The periodic checking of stale providers and tier updates has been moved to the Scheduling Service, which calls the respective APIs at configured intervals.

 - GET /api/health/nodes/healthy:  Get a list of healthy nodes (nodes that are green)
 - GET /api/health/provider/:provider_id: Get the health status of a specific provider
 - GET /api/health/providers/filter: Get a list of healthy providers offering a specific model within cost constraints	
 - POST /api/health/providers/update-tiers: Manually triggers the tier update process for all providers based on their health history
 - POST /api/health/providers/check-stale - Manually triggers the check for stale providers that haven't sent health updates recently. 




## 9.  CockroachDB - DONE!!!!

-  **Role:** Distributed data store for users, API keys, HMACs, provider data, and transaction records.

## 10.  Logging and Monitoring Service

-  **Role:** Capeture and store logs from all of our services.

Use OpenTelemetry to capture logs from all of our services and send to datadog
OR use this - https://www.multiplayer.app/docs/  Auto-documenttion and provides network map


## 11. Scheduling Service:

- We need to create an external service that runs on GCP  that runs the scheduling for each of our APIs that need to be manually triggered. This should connect to any of our central nodees as cockcraochDB will keep the DB consistent across all of our Nodes

#### provider_health 
 - check stale providers
 - update tiers

### 12. Provider client (GO)


- Should be applied via Docker - make sure docker is small and efficient as can be.
- Documentation to run client on your own (this will include the cron jon to send health updates)

-  **Role:** GO client running on provider side that will be used to send the health updates to the central node, pass request to Ollama and validate HMAC requests. 
- Also runs 

- It should also have a very short timeout set so that if the procider client cannot connect it local Ollama it will let our Procider-communiction service know about it. Our provider-communication service should then try to send the request to another provider.
- Will also be running  API that exposes nvidia-smi so nginx can check the utilization of the CPU. Check here.
https://github.com/Opa-/nvidia-smi-rest/tree/main
https://github.com/kesor/ollama-proxy/blob/main/Dockerfile

- We should also collect:
	- GPU TYPE 
	- Number of GPUs
	- Current ngrok or cloudflare URL that is running on the provider machine
	- Save the utilization in our health check as this will give us some usage stats overtime.


### 13. Autoamatic routing based on request.

https://github.com/lm-sys/RouteLLM
https://arxiv.org/abs/2406.18665

- See if we can implement this so that users don't specify a Model and we choose the best/cheapest model based on the request.

### 14. Documentation using fern

Rightbrain Petes new company uses fern and the documentation looks so good.
Use fern - book a demo.
https://www.buildwithfern.com


# Revised Request-Response Workflow

## 1.  Consumer Request Initiation:

 - The consumer sends a POST /process request (with API key and request data) to OpenSentry Nginx. Request will be based on the OpenAI API standard so the API-KEY will be in Authorization: Bearer $API_KEY

## 2.  Orchestration Start:

-  OpenSentry Nginx forwards the request to the Orchestrator. 

## 3.  Authentication & Fund Check:

-  The Orchestrator calls the Authentication Service to validate the API key and verify that the consumer has at least $1.

- If validation fails, the Orchestrator returns an error response immediately via the proxy.

-  If valid, the Orchestrator (or Authentication Service) performs holding deposit of $1. This is done on cockcroachDB against the consumers user row in the table. To ensure that even if they consumer connects to another central node proxy ( as we might have multiple in the future) the holding deposit is aleady there. 

## 4.  HMAC Generation:

-  The Orchestrator invokes the HMAC Service to generate one or more HMACs using the consumer's API key, first vew bytes of the request data, and a timestamp.

-  The generated HMAC(s) are stored in CockroachDB.

## 5.  Provider Selection:

-  The Orchestrator queries the Provider Management Service for a list of healthy providers (e.g., returns a list of three). 

## 6.  Dispatch to Providers:

-  The Orchestrator calls the Provider Communication Service's POST /send_requests endpoint, passing along the consumer's request data, the HMAC(s), and the selected providers.

-  The Provider Communication Service forwards the requests to these providers (using HTTP/JSON).

## 7.  Provider Response Handling:

- Providers first check HMAC is valid by passing the HMAC back to Provider Communication Service /post validate_hmac if valid then

-  As providers responds the Provider Communication Service collects the responses.

-  The first valid provider response is identified as the winner. 

-  The Provider Communication Service sends the winning response back to the Orchestrator via its POST /receive_response endpoint.

## 8.  Response Delivery:

• The Orchestrator, upon receiving the winning provider's response, immediately sends this final response back to OpenSentry Nginx, which then delivers it to the consumer.

## 9.  Asynchronous Payment Processing:

-  Simultaneously, the Orchestrator (or Provider Communication Service) publishes a payment task message to a designated RabbitMQ queue. This message includes details like:

	-  Consumer ID
	- Provider ID
	- HMAC
	- Token input and output counts
	-  Latency information
	-  Timestamp
	-  The Payment Processing Service later consumes these messages and performs:
	-  Debiting the consumer (according to provider pricing parameters)
	-  Crediting the provider
	-  Calculating tokens per second
	-  Releasing the $1 holding deposit
 

## Techstack 

- For all our GO microservices we will utilise the echo framework.
- Opensentry as our NGINX
- CockcroachDB as your database.
