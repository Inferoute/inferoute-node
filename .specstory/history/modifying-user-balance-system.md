## SpecStory

## Modifying User Balance System (23/02/2025, 16:06:42)

_**User**_

First can you read @technical.md and read al the code so you can understand how it all works.



Currently balances are attached to providers and consumers, but we have now chaged it so that users can have multiple providers or consumers, so can we chnage how our system works, so that balances are attached to users. This will have mutiple implications.


Todo:
1. Change the  balances table and ensure  balances are attached users.
2. In the orchestrator when a request comes, this will be via a consumer. So we need to update the orchestrator to first extract the user attached to the consumer and keep it handy.
3. Then when we place a deposit hold we need to now do this for the user not the consumer. So we need to also update the auth micro-serice APIs( hold and release deposits ) to work on the user. The orchestrator will also need to be updateed in how it calls the APIs so that it now passes the user.
4. Also in the payment microservice, we need to ensure we credit the user attached to a provider and debt the user attached to the consumer.


6. Finally update @seed.sql to reflect how the new balances table works.


---

_**Assistant**_

I'll help you modify the system to attach balances to users instead of providers/consumers. Let me analyze the current structure and propose the changes needed.

First, let's modify the balances table schema:

```sql
-- Drop existing balances table
DROP TABLE IF EXISTS balances;

-- Create new balances table linked to users
CREATE TABLE IF NOT EXISTS balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    available_amount DECIMAL(18,8) NOT NULL DEFAULT 0,
    held_amount DECIMAL(18,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK (available_amount >= 0),
    CHECK (held_amount >= 0)
);
```

Now let's update the seed.sql to reflect this change:

---

_****_



---

_****_



---



---

