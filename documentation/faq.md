# Frequently Asked Questions

## Provider Selection and Orchestration

### Q: When a user has the `default_to_own_models` set to TRUE, what does that mean?
A: When `default_to_own_models` is set to TRUE, the orchestrator will first try to use the user's own providers and only if they fail will the orchestrator choose other providers. It's important to note that even when a user does not choose to use their own providers, their providers might still be included as the orchestrator performs two provider selection functions: one to get the user's providers and another to get healthy providers that fit the requirements. The healthy provider list might still contain the user's providers.

### Q: What happens in terms of costs when a user uses their own providers?
A: The platform does not charge a service fee in those instances. When a user's request is routed to their own provider, the transaction is recorded but no service fee is applied, effectively making it a zero-fee transaction within the platform.

### Q: How does the orchestrator select providers for a request?
A: The orchestrator follows a sophisticated selection process:
1. It first checks if the user has `default_to_own_models` set to TRUE
2. If TRUE, it fetches the user's own providers that support the requested model
3. It then gets a list of healthy providers that meet the price constraints
4. Providers are scored based on price and performance (TPS - tokens per second)
5. If the user has their own providers and `default_to_own_models` is TRUE, up to 3 user providers are prioritized at the front of the list
6. Up to 3 non-user providers are added as fallbacks
7. The orchestrator tries providers in order until a successful response is received

### Q: What health statuses can providers have and what do they mean?
A: Providers can have three health statuses:
- **Green**: Fully operational with all models available
- **Orange**: Partially operational with some models available
- **Red**: Not operational or unavailable

The orchestrator will only select providers with "green" or "orange" health status, never those with "red" status.

### Q: How are providers scored and ranked?
A: Providers are scored using a weighted algorithm that considers:
1. Price (input and output token costs)
2. Performance (average tokens per second)

The weighting depends on the `sort` parameter in the request:
- For `sort=cost` (default): 70% price, 30% performance
- For `sort=throughput`: 20% price, 80% performance

## Transactions and Payments

### Q: How does the platform handle payments for API requests?
A: The platform uses a multi-step process:
1. When a request is received, a $1.00 holding deposit is placed. This prevents the consumer from trying to double spend.
2. The request is forwarded to the selected provider.  
3. After receiving a response, the holding deposit is released. We also record the costs that the provider had in the DB and is attached to the transaction. This prevents 
4. The actual cost is calculated based on input/output tokens and provider prices
5. A payment message is published to RabbitMQ for processing
6. The transaction record is updated with the final details

### Q: How do we prevent from updating their costs mid transaction.
A: !!!!


### Q: How do we prevent a consumer from double spending.
A: When a request is received, a $1.00 holding deposit is placed. This prevents the consumer from trying to double spend

### Q: What happens if a provider fails to respond?
A: If the primary provider fails to respond, the orchestrator will try fallback providers in order of their score. If all providers fail, the holding deposit is released and an error is returned to the user.

### Q: How are transaction costs calculated?
A: Transaction costs are calculated using the formula:
```
InputCost = InputTokens × InputPricePerToken
OutputCost = OutputTokens × OutputPricePerToken
TotalCost = InputCost + OutputCost
```

The service fee is a percentage of the total cost, and the provider earnings are the total cost minus the service fee.

## User and Provider Settings

### Q: What are consumer settings and how do they affect requests?
A: Consumer settings include price constraints for input and output tokens. These settings can be:
- Global settings that apply to all models
- Model-specific settings that override global settings for particular models

The orchestrator uses these settings to filter providers based on price compatibility.

### Q: Can users set different price constraints for different models?
A: Yes, users can set model-specific price constraints in the `consumer_models` table. These override the global settings in the `consumers` table for the specified models.

### Q: How does the platform determine which providers a user owns?
A: The platform identifies user-owned providers by matching the user ID with provider IDs in the database. When `default_to_own_models` is TRUE, the orchestrator makes a special request to the health service to fetch providers associated with the user's ID.

## Technical Details

### Q: What happens when a provider's health status changes?
A: Provider health is monitored by the Provider Health Service, which:
1. Receives health check messages from providers
2. Evaluates the availability of models
3. Updates the provider's health status in the database
4. The orchestrator will automatically adapt to these changes in subsequent requests

### Q: How does the platform ensure fair provider selection?
A: The platform uses a scoring system that balances price and performance. This ensures that:
1. Users get the best value for their money
2. High-performing providers are rewarded
3. The selection can be tuned using the `sort` parameter to prioritize cost or throughput

### Q: What is the purpose of the HMAC in transactions?
A: The HMAC (Hash-based Message Authentication Code) serves several purposes:
1. It uniquely identifies each transaction
2. It allows providers to verify that requests are legitimate
3. It links the initial request to the completion and payment
4. It prevents unauthorized modifications of the request

### Q: How does the platform handle provider tiers?
A: Providers are assigned to tiers (1-3) based on their quality and reliability:
- Tier 1: Premium providers with highest quality
- Tier 2: Standard providers with good quality
- Tier 3: Basic providers or new providers

The tier affects provider selection, with higher-tier providers generally receiving preference when other factors are equal. 