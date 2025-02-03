


# Services and Their Roles

All of these services listed below will run on a single server and are collectively called the central-node.

## 1.   OpenSentry Nginx (API Gateway)

This is our main entry point for consumers. Consumers will conect using a univeral API point (most likely this will be a cloudflare proxy) which will then point to this proxy.

- **Role:** The public entry point. Handles TLS termination, rate limiting, and basic routing.

-  **Flow:** Forwards incoming consumer requests to the Orchestrator and passes back responses to the consumer. Consumer request data and API-KEY (which will be in the AUTH-)

## 2.  Orchestrator (GO)

The main orchestrator of our application.

- **Role:** The central controller that coordinates the overall request workflow.

- **Responsibilities:**

	- Receives requests from Nginx.

	- Invokes the Authentication Service to validate the API key and check for sufficient funds. 

    - Generates HMAC(s) for the request

    - Creates a transaction record in the database.

    - Initiates a holding deposit (deduct $1) for valid requests using auth service

	- Queries the Provider Management Service for a list of healthy providers.

	- Calls the Provider Communication Service to dispatch the request (with HMAC) to a set of providers.

	- Receives the winning provider’s response from the Provider Communication Service.

	- Forwards that response back to the Nginx proxy.

	- Publishes payment-related details to RabbitMQ for asynchronous processing by the Payment Processing Service.

## 3.  Authentication Service (Go)

- **Role:** Validates the consumer’s API key and verifies the consumer’s balance (ensuring they have at least $1).

- **Endpoints (HTTP/JSON):**

	- POST /api/auth/users - Create a new user with API key and initial balance
    - POST /api/auth/validate - Validate API key and check consumer's balance
    - POST /api/auth/hold - Place a hold on consumer's deposit
    - POST /api/auth/release - Release a previously held deposit



## 5.  Provider Management Service (Go)


 - **Role:** Maintains a list of APIs which providers can use to add models, delete models, pause and unpause their services. List a providers models, get a providers status and pushes health status to RAbbitmq for digest.

-  **Endpoints (HTTP/JSON):**

- POST /api/provider/models - Add a new model for the provider
- GET /api/provider/models - List all models for the provider
- PUT /api/provider/models/{model_id} - Update an existing model
- DELETE /api/provider/models/{model_id} - Delete a model
- GET /api/provider/status - Get provider's current status
- PUT /api/provider/pause - Update provider's pause status
- POST /api/provider/health - Push provider's current health and model status


## 6.  Provider Communication Service (Go)

Dispatches 

  - **Role:** Dispatches the consumer’s request (with the associated HMACs) to a set of providers and collects responses and provides an API where providers can validate the HMAC request received. Once a provider has vlidated the request the will send the datya back to this service.

 - **Flow:** Once a provider returns a valid response (the “winning” response), it sends that response back to the Orchestrator, which wil send it back to the consumer via the nginx proxy

-  **Endpoints (HTTP/JSON):**

	- POST /send_requests – initiates sending requests to the providers.
 
	- POST /validate_hmac - Allows providers to validate their HMAC is valid and open.

	- POST /receive_response – receives provider responses and determines the winner.

## 7.  Payment Processing Service (Go) with RabbitMQ

-  **Role:** Handles the asynchronous financial operations after the main workflow is complete.

-  **Workflow:** Consumes messages from a RabbitMQ queue, then debits the consumer, credits the provider, computes tokens per second, and releases the holding deposit.

- **Technology:** Uses RabbitMQ for message queuing.

## 8. General API services (GO)

-  **Role:** Provides API to consumers and providers that do not relate to LLM requests, for example:
	-  **Consumers** can check their  spend based on keys
	- **Providers** can check their profit based on models and time
	- **Providers**  can set when their LLM compute is available by pausing and unpausing themselves from bveing available for external consumers.  
	- **Providers**can update their pricing remotely and set models they have available.

## Provider-Health

-  **Role:** Looks at rabbit message queue for provider health and updates the provider status in the database AND maintains a hlist of healthy nodes that is used by the orchestrator to select providers.


## 9.  CockroachDB

-  **Role:** Distributed data store for users, API keys, HMACs, provider data, and transaction records.

## 10.  Logging and Monitoring Service

-  **Role:** Centralized logging and metrics aggregation (using Prometheus, Grafana, ELK/EFK, etc.).



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
