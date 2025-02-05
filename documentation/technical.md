


# Services and Their Roles

All of these services listed below will run on a single server and are collectively called the central-node.

## 1.   Nginx (API Gateway)

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

    - Creates a transaction record in the database.

    - Initiates a holding deposit (deduct $1) for valid requests using auth microservice - /api/auth/hold

	- Queries the Provider Management Service for a list of healthy providers.

	- Calls the Provider Communication Service to dispatch the request (with HMAC) to a set of providers.

	- Receives the winning provider’s response from the Provider Communication Service.

	- Forwards that response back to the Nginx proxy.

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


## 5.  Provider Communication Service (Go)

Dispatches 

  - **Role:** Dispatches the consumer’s request (with the associated HMACs) to a provider and collects the response and provides an API where providers can validate the HMAC request received. 

 - **Flow:** Once a provider returns a valid response, it sends that response back to the Orchestrator, which will send it back to the consumer via the nginx proxy, 


-  **Endpoints (HTTP/JSON):**

	- POST /send_requests – initiates sending requests to a provider (the provider will be specified in the request), which is used by orchestrator to send the request to providers and once the response is received it will be sent back to the orchestrator.
 
	- POST /validate_hmac - Allows providers to validate their HMAC is valid and open.


## 6.  Payment Processing Service (Go) with RabbitMQ

-  **Role:** Handles the asynchronous financial operations after the main workflow is complete.

-  **Workflow:** Consumes messages from a RabbitMQ queue, then debits the consumer, credits the provider, computes tokens per second, and releases the holding deposit.

- **Technology:** Uses RabbitMQ for message queuing.

## 7. General API services (GO)

-  **Role:** Provides API to consumers and providers that do not relate to LLM requests, for example:
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

-  **Role:** Centralized logging and metrics aggregation (using Prometheus, Grafana, ELK/EFK, etc.).


## 11. Scheduling Service:

- We need to create an external service that runs on GCP  that runs the scheduling for each of our APIs that need to be manually triggered. This should connect to any of our central nodees as cockcraochDB will keep the DB consistent across all of our Nodes

#### provider_health 
 - check stale providers
 - update tiers

### 12. Provider client


-  **Role:** Nginx client running on provider side that will be used to send the health updates to the central node, pass request to Ollama and validate HMAC requests. 
- Also runs 

- It should also have a very short timeout set so that if the procider client cannot connect it local Ollama it will let our Procider-communiction service know about it. Our provider-communication service should then try to send the request to another provider.
- Will also be running  API that exposes nvidia-smi so nginx can check the utilization of the CPU. Check here.
https://github.com/Opa-/nvidia-smi-rest/tree/main





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

-  The Orchestrator invokes the HMAC Service to generate one or more HMACs using the consumer’s API key, first vew bytes of the request data, and a timestamp.

-  The generated HMAC(s) are stored in CockroachDB.

## 5.  Provider Selection:

-  The Orchestrator queries the Provider Management Service for a list of healthy providers (e.g., returns a list of three). 

## 6.  Dispatch to Providers:

-  The Orchestrator calls the Provider Communication Service’s POST /send_requests endpoint, passing along the consumer’s request data, the HMAC(s), and the selected providers.

-  The Provider Communication Service forwards the requests to these providers (using HTTP/JSON).

## 7.  Provider Response Handling:

- Providers first check HMAC is valid by passing the HMAC back to Provider Communication Service /post validate_hmac if valid then

-  As providers responds the Provider Communication Service collects the responses.

-  The first valid provider response is identified as the winner. 

-  The Provider Communication Service sends the winning response back to the Orchestrator via its POST /receive_response endpoint.

## 8.  Response Delivery:

• The Orchestrator, upon receiving the winning provider’s response, immediately sends this final response back to OpenSentry Nginx, which then delivers it to the consumer.

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
