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

	- Receives requests from Nginx via an API call.

	- Invokes the Authentication Service to validate the API key and check for sufficient funds using auth microservice -  /api/auth/validate  (requires INTERNAL_API_KEY). Also at this point we should check what the maximum a user is willing to pay for their inference for input and output, if a specific price is set for the requested model otherwise use the global settings.
		- So we need a new table called consumers where such settings can be stored. Links to the users table. Allows user to set their maximum price per 1,000,0000 input tokens and per 1,000,000 output tokens.
		- We also need a consumer_models table where user can set their minimum and maximum price per 1,000,000 input tokens and per 1,000,000 output tokens for a specific model. So by default the consumer will use the global settings but can override them for a specific model. 
	
	- Generates HMAC(s) for the request using common/hmac.go

	- Queries the Provider Management Service for a list of healthy providers using the model name and the maximum price per 1,000,000 input tokens and per 1,000,000 output tokens. So essentiallly we only spit out providers where the price is eqaual or lower to our amount. The consumer is always charged at the price of the provider they are connected to. 

	- We then need a little function that works out the best provider for the consumer based on price and latency. 

    - Creates a transaction record in the database - initial transaction status is pending. Record the input and output price per 1,000,000 tokens, provider id, model name and HMAC.

    - Initiates a holding deposit (deduct $1) for valid requests using auth microservice - /api/auth/hold

	- Calls the Provider Communication Service to dispatch the request (with HMAC) to the provider. We send provider_url, request data, model name, HMAC to the provider communication service via /api/provider_comms/send_requests

	- Receives the winning provider's response from the Provider Communication Service.

	- Forwards that response back to the Nginx proxy which should then send it back to the consumer.

	- Initiates a release deposit using auth microservice - /api/auth/release

	- Updates the transaction record in the database with the  total input tokens, total output tokens, latency and changes status to payment. This is so the payment processing service can pick it up. 
	also if anyhtig gets missed we can double check it using payment processing service. 

	- Publishes payment-related details to RabbitMQ for asynchronous processing by the Payment Processing Service. 


## 3.  Authentication Service (Go) - DONE !!!!  (requires INTERNAL_API_KEY so only authorized internal go services can use these APIs)

- **Role:** Manages user authentication, API key validation, and deposit handling for both consumers and providers. Ensures users have sufficient balance for operations and provides deposit hold/release functionality for transaction safety.

- **Endpoints (HTTP/JSON):**

POST /api/auth/users
  - Create a new user account with API key
  - Requires user type (consumer/provider)
  - Generates a unique API key for the user
  - Creates associated consumer or provider record
  - Sets initial deposit balance if provided (for consumers)
  - Returns user ID and API key

POST /api/auth/validate
  - Validates API key and checks user's details
  - For consumers: ensures minimum balance requirement ($1.00)
  - For providers: verifies active status
  - Returns user details and type-specific information
  - Authentication: API key required in Bearer token
  - Response includes:
    - User ID and type
    - For consumers: available balance and account status
    - For providers: tier and health status

POST /api/auth/hold
  - Places a temporary hold on consumer's deposit of $1
  - Only applicable for consumer accounts
  - Used before processing transactions
  - Prevents double-spending
  - Authentication: API key required

POST /api/auth/release
  - Releases a previously held deposit of $1.00
  - Only applicable for consumer accounts
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
	- Returns simplified response with just pause status
	- Authentication: Provider API key required

- GET /api/provider/health
	- Get the current health status of the authenticated provider
	- Returns provider health details including:
		- Provider ID
		- Username
		- Health status (green/orange/red)
		- Tier level
		- Availability status
		- Latest latency metrics
		- Last health check timestamp
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

- PUT /api/provider/api_url
    - Update provider's API URL endpoint
    - Requires provider API key authentication
    - Payload:
      - New API URL
    - Returns:
      - Success status and message

Create abnother API to allow providers to change their URL endpoint (container willd o this autopmactially)

## 5.  Provider Communication Service (Go) - DONE!!!!

- **Role:** Dispatches consumer requests (with associated HMACs) to providers and collects responses. Also provides an API for providers to validate HMACs.

- **Architecture:**
  - Built using Echo framework for HTTP routing
  - Uses CockroachDB for transaction and HMAC validation
  - Implements graceful shutdown for clean service termination
  - Runs on port 8083 in development mode

- **Authentication:**
  - Skips authentication for `/api/provider_comms/send_requests` (used by orchestrator and requires INTERNAL_API_KEY)
  - Validates API keys against the database in real-time

- **Key Components:**

  1. **Request Handler:**
     - Receives requests from the orchestrator
     - Validates provider existence and availability
     - Forwards requests to providers
     - Measures and tracks latency
     - Returns provider responses to orchestrator

  3. **Error Handling:**
     - Custom error types for different scenarios
     - Proper HTTP status codes for each error case
     - Detailed error messages for debugging

- **Endpoints (HTTP/JSON):**

  1. **POST /api/send_requests**
     - Used by orchestrator to send requests to providers
     - Payload includes:
       - Provider ID
       - HMAC
	   - provider_url
       - Request data
       - Model name
     - Returns:
       - Success/failure status
       - Provider response data
       - Latency metrics


- **Flow:**
  1. Orchestrator sends request to `/api/provider_comms/send_requests`
  2. Service validates provider and prepares request
  3. Request is sent to provider's endpoint
  4. Provider validates HMAC using `/api/provider/validate_hmac`
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


## 6.  Payment Processing Service (Go) with RabbitMQ - DONE

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

 - GET /api/health/providers/healthy:  Get a list of healthy providers (providers that are green)
 - GET /api/health/providers/filter: Get a list of healthy providers offering a specific model within cost constraints	
 - GET /api/health/provider/:provider_id: Get the health status of a specific provider
 - POST /api/health/providers/update-tiers: Manually triggers the tier update process for all providers based on their health history (protected by INTERNAL_API_KEY)
 - POST /api/health/providers/check-stale - Manually triggers the check for stale providers that haven't sent health updates recently. (protected by INTERNAL_API_KEY)

We will need some Cloud cron thing to run the check for stale providers and update tiers from externally.


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
- Client should also first check whether they are busy performing inference and if so reject the request
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
- Allow a user to set a strong model and a weaker model and we can use this to route requests, based on their incoming text.

### 14. Documentation using fern

Rightbrain Petes new company uses fern and the documentation looks so good.
Use fern - book a demo.
https://www.buildwithfern.com



################ Initial Account creation process


Providers



### Consumers 

	1. Logs in using google, github etc..
	2. We create a consumer row in the database.
	3. User is send to welcome page where he is asked for min and max price per 1,000,000 input tokens and per 1,000,000 output tokens. (explain this is a global setting and can be overridden for a specific model)
	4. User clicks next and is sent to a page where he can create an API key.
	5. Shows them example of how to use the service (basically worsk out of the box like OpenAI comopatible API)

# Revised Request-Response Workflow

## 1. Consumer Request Initiation:
- Consumer sends a request to the OpenAI-compatible endpoints (/v1/chat/completions, /v1/completions)
- Request includes API key in Authorization header (Bearer format) and model-specific parameters
- Request is received by Nginx which routes to the Orchestrator service

## 2. Authentication & Validation:
- Orchestrator validates the API key through the Auth Service (/api/auth/validate)
- Validates consumer exists and has sufficient balance
- Places a $1 holding deposit via Auth Service (/api/auth/hold)
- Verifies consumer's price limits for the requested model

## 3. Provider Selection:
- Queries Provider Health Service for available providers matching:
  - Requested model
  - Consumer's maximum price constraints
  - Health status (green/orange)
  - Provider tier requirements
- Selects optimal provider based on price, latency, and tier

## 4. Transaction Initialization:
- Creates a transaction record with 'pending' status
- Generates HMAC for request validation
- Records initial transaction details (consumer, provider, model, pricing)

## 5. Request Processing:
- Forwards request to Provider Communication Service with:
  - Provider's URL
  - HMAC
  - Original request data
  - Model name
- Provider Communication Service sends request to provider
- Provider validates HMAC via Provider Management Service
- Provider processes request and returns response

## 6. Response Handling:
- Provider Communication Service receives response
- Returns response data, latency metrics, and token counts to Orchestrator
- Orchestrator immediately forwards response to consumer

## 7. Transaction Finalization:
- Updates transaction with:
  - Total input/output tokens
  - Latency metrics
  - Changes status to 'payment'
- Publishes payment message to RabbitMQ with:
  - Consumer/Provider IDs
  - Token counts
  - Pricing information
  - Latency data

## 8. Asynchronous Payment Processing:
- Payment Processing Service consumes RabbitMQ message
- Calculates:
  - Tokens per second
  - Consumer cost
  - Provider earnings
  - Service fee (5%)
- Updates balances:
  - Debits consumer
  - Credits provider
  - Releases holding deposit
- Updates transaction status to 'completed'

## Techstack 

- For all our GO microservices we will utilise the echo framework.
- Opensentry as our NGINX
- CockcroachDB as your database.

# User Management

The platform implements a flexible user management system where users can be associated with either consumers or providers:

## User Types and Relationships

- **Users**: Base entity containing authentication information
  - Unique ID (UUID)
  - Username
  - Type (consumer/provider)
  - Created/Updated timestamps
  - Authentication details

- **Consumers**: Extended user type for those consuming inference services
  - Links to user via user_id
  - Global settings for maximum price per 1M input/output tokens
  - Balance and payment information
  - Can override pricing settings per model via consumer_models table

- **Providers**: Extended user type for those providing inference services
  - Links to user via user_id
  - Provider URL endpoint
  - Health status and tier information
  - Model configurations and pricing
  - Earnings and performance metrics
