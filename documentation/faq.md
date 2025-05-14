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
3. After receiving a response, the holding deposit is released.
4. The actual cost is calculated based on input/output tokens and the initial provider prices that were stored when the transaction was created.
5. A payment message is published to RabbitMQ for processing
6. The transaction record is updated with the final details

### Q: How do we handle provider price changes during transactions?
A: The platform stores the initial pricing for all selected providers when a transaction is created. When a provider successfully fulfills a request, we use their stored initial pricing for the cost calculations, regardless of any price changes they may have made during the transaction. This ensures fairness and transparency in pricing.

### Q: How do we prevent a consumer from double spending?
A: When a request is received, a $1.00 holding deposit is placed. This prevents the consumer from trying to double spend.

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

### Q: How does the platform handle stale providers?
A: The platform has a robust stale provider detection system:


**Manual Trigger:**
- Can be triggered via API endpoint `/api/health/providers/check-stale`
- Marks providers as unavailable if they haven't sent a health check in the last 5 minutes
- Only affects providers that are currently marked as available and not paused
- Updates the `is_available` flag to `false` for stale providers
- Returns the number of providers that were marked as unavailable
- Protected by internal API key for security

**Impact:**
- Stale providers are automatically removed from the provider selection pool
- Helps maintain service reliability by quickly detecting offline providers
- Prevents requests from being routed to potentially unresponsive providers

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
A: Providers are assigned to tiers (1-3) based on their health check history over the past 30 days:

**Tier Calculation:**
- Tier 1 (Premium): ≥99% green health checks
- Tier 2 (Standard): ≥95% but <99% green health checks
- Tier 3 (Basic): <95% green health checks or new providers

**Update Process:**
- Tiers are not automatically updated but the API needs to be called.
- Can be manually triggered via API endpoint `/api/health/providers/update-tiers`
- Only providers with at least 10 health checks in the past 30 days are evaluated
- Tier changes are logged with health percentage and check counts

**Impact on Business:**
- Higher tiers can generally charge premium rates for their services
- Tier affects provider selection priority in the orchestrator
- Pricing examples by tier:
  - Tier 1: Premium pricing (e.g., deepseek-r1:8b at 0.15/0.3)
  - Tier 2: Mid-range pricing (e.g., deepseek-r1:8b at 0.5/0.15)
  - Tier 3: Lower pricing (e.g., basic models at 0.2/0.6)

**Provider Selection:**
- Higher tier providers are prioritized in selection
- Within tiers, providers are ranked by:
  - Average tokens per second (TPS)
  - Price constraints
  - Current health status
  - Availability

New providers start at Tier 3 and can move up by maintaining consistent uptime and performance.

## Provider Pricing and Cheating Prevention

### Q: How does the platform prevent providers from changing prices during a transaction?
A: The platform implements a strict cheating detection system:
1. When a transaction starts, its creation timestamp is recorded
2. When the transaction completes, its completion timestamp is recorded
3. The system checks if the provider's pricing was updated between these timestamps
4. If a price update is detected during this window, it's considered cheating

### Q: What happens if a provider is caught changing prices during a transaction?
A: The platform takes several actions:
1. The consumer is not charged for the request
2. The provider receives no earnings
3. The platform charges a $1 penalty fee to the provider.
4. The incident is recorded in the `provider_cheating_incidents` table for investigation
5. The transaction status is marked as 'cheating_detected'

### Q: How can I check if a provider has attempted to cheat?
A: You can query the `provider_cheating_incidents` table, which records:
- The provider and model IDs
- Transaction details (HMAC, timestamps)
- The pricing that was attempted to be changed
- The total tokens involved in the transaction

### Q: Why is changing prices during a transaction considered cheating?
A: Price changes during transactions are prohibited because:
1. The consumer agreed to the original price when making the request
2. Mid-transaction price changes could lead to unexpected costs
3. It maintains pricing transparency and fairness
4. It prevents providers from exploiting price differences during processing 