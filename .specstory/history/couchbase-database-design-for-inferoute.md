## SpecStory

## Couchbase Database Design for Inferoute (31/01/2025, 12:17:12)

_**User**_

Help me design a cockcroach  table and save it as a file that I can run to create a database.

Call the database inferoute.

I need 3 tables:

Table 1 - Users
Type - Users will either be consumers or Providers
Public-Key - the public. key assocated wth thei crypto wallet. 



Table 2 API-KEYS
A list of API-KEYS that can link back to a user. As a user might have more than one.


Table 3 - Transactions
Consumer - Links to a user id
Final Provider - Links to a single provider from user table
Providers - List of multiple providers from user table 
HMAC - a HMAC hash that wil be saved 
Total input tokens
Total output tokens
Tokens per second
Latency 

---

_**Assistant**_

I'll help you create a Cockroach DB schema. First, let me create a new SQL file with the schema.

---

_****_

```diff
- 
+ -- Create the inferoute database
+ CREATE DATABASE IF NOT EXISTS inferoute;
+ USE inferoute;
+ 
+ -- Users table
+ CREATE TABLE IF NOT EXISTS users (
+     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
+     type STRING NOT NULL CHECK (type IN ('consumer', 'provider')),
+     public_key STRING UNIQUE NOT NULL,
+     created_at TIMESTAMP DEFAULT current_timestamp(),
+     updated_at TIMESTAMP DEFAULT current_timestamp()
+ );
+ 
+ -- API Keys table with reference to users
+ CREATE TABLE IF NOT EXISTS api_keys (
+     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
+     user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
+     api_key STRING UNIQUE NOT NULL,
+     is_active BOOLEAN DEFAULT true,
+     created_at TIMESTAMP DEFAULT current_timestamp(),
+     updated_at TIMESTAMP DEFAULT current_timestamp(),
+     INDEX (user_id)
+ );
+ 
+ -- Transactions table
+ CREATE TABLE IF NOT EXISTS transactions (
+     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
+     consumer_id UUID NOT NULL REFERENCES users(id),
+     final_provider_id UUID NOT NULL REFERENCES users(id),
+     providers UUID[] NOT NULL, -- Array of provider IDs
+     hmac STRING NOT NULL,
+     total_input_tokens INTEGER NOT NULL,
+     total_output_tokens INTEGER NOT NULL,
+     tokens_per_second FLOAT NOT NULL,
+     latency INTEGER NOT NULL, -- Stored in milliseconds
+     created_at TIMESTAMP DEFAULT current_timestamp(),
+     INDEX (consumer_id),
+     INDEX (final_provider_id),
+     CONSTRAINT check_provider_type CHECK (
+         EXISTS (
+             SELECT 1 FROM users 
+             WHERE id = final_provider_id 
+             AND type = 'provider'
+         )
+     )
+ );
+ 
+ -- Add triggers for updated_at
+ CREATE OR REPLACE FUNCTION update_updated_at()
+ RETURNS TRIGGER AS $$
+ BEGIN
+     NEW.updated_at = current_timestamp();
+     RETURN NEW;
+ END;
+ $$ LANGUAGE plpgsql;
+ 
+ CREATE TRIGGER update_users_updated_at
+     BEFORE UPDATE ON users
+     FOR EACH ROW
+     EXECUTE FUNCTION update_updated_at();
+ 
+ CREATE TRIGGER update_api_keys_updated_at
+     BEFORE UPDATE ON api_keys
+     FOR EACH ROW
+     EXECUTE FUNCTION update_updated_at(); 
```

---

_****_



Key features of this schema:

1. All tables use UUID as primary keys
2. Added timestamps for auditing (created_at, updated_at)
3. Proper foreign key constraints between tables
4. Indexes on frequently queried fields
5. Type validation for user types
6. Automatic trigger for updating timestamps
7. Array type for providers list in transactions
8. Constraint ensuring final_provider is actually a provider
9. Cascade deletion for API keys when user is deleted

You can run this file directly in CockroachDB to create the database and tables. Want me to add any additional features or make any adjustments?

---

_**User**_

> cockroach sql --insecure --host=localhost --database=inferoute < schema.sql
CREATE DATABASE

Time: 31ms

SET

Time: 33ms

CREATE TABLE

Time: 23ms

CREATE TABLE

Time: 194ms

ERROR: column "type" does not exist
SQLSTATE: 42703
ERROR: column "type" does not exist
SQLSTATE: 42703
Failed running "sql"

---

_**Assistant**_

The error is occurring because the CHECK constraint in the transactions table is trying to reference a subquery before the table is fully created. Let me fix the schema by moving the provider type check to a separate constraint after table creation.

---

_****_

```diff
      created_at TIMESTAMP DEFAULT current_timestamp(),
      INDEX (consumer_id),
-     INDEX (final_provider_id),
-     CONSTRAINT check_provider_type CHECK (
+     INDEX (final_provider_id)
+ );
+ 
+ -- Add provider type check constraint
+ ALTER TABLE transactions
+ ADD CONSTRAINT check_provider_type CHECK (
      EXISTS (
          SELECT 1 FROM users 
-             WHERE id = final_provider_id 
+         WHERE id = transactions.final_provider_id 
          AND type = 'provider'
-         )
      )
  );
```

---

_****_



Try running the schema again. I've made these changes:

1. Moved the provider type check to an ALTER TABLE statement after table creation
2. Added proper table qualification (transactions.final_provider_id) in the check constraint
3. Cleaned up the constraint syntax

This should resolve the error you were seeing. Run the same command again:

```bash
cockroach sql --insecure --host=localhost --database=inferoute < schema.sql
```

---

_**User**_

CREATE DATABASE

Time: 7ms

SET

Time: 1ms

NOTICE: relation "users" already exists, skipping
CREATE TABLE

Time: 8ms

NOTICE: relation "api_keys" already exists, skipping
CREATE TABLE

Time: 7ms

CREATE TABLE

Time: 164ms

ERROR: column "type" does not exist
SQLSTATE: 42703
ERROR: column "type" does not exist
SQLSTATE: 42703
Failed running "sql"

---

_**Assistant**_

The issue is still with the check constraint. Let's try a different approach by using a foreign key constraint with a subquery instead.

---

_****_

```diff
  CREATE DATABASE IF NOT EXISTS inferoute;
  USE inferoute;
+ 
+ -- Drop existing tables if they exist
+ DROP TABLE IF EXISTS transactions;
+ DROP TABLE IF EXISTS api_keys;
+ DROP TABLE IF EXISTS users;
  
  -- Users table
      updated_at TIMESTAMP DEFAULT current_timestamp()
  );
+ 
+ -- Create a view for providers
+ CREATE VIEW providers AS
+ SELECT id FROM users WHERE type = 'provider';
  
  -- API Keys table with reference to users
      id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      consumer_id UUID NOT NULL REFERENCES users(id),
-     final_provider_id UUID NOT NULL REFERENCES users(id),
-     providers UUID[] NOT NULL, -- Array of provider IDs
+     final_provider_id UUID NOT NULL,
+     providers UUID[] NOT NULL,
      hmac STRING NOT NULL,
      total_input_tokens INTEGER NOT NULL,
      total_output_tokens INTEGER NOT NULL,
      tokens_per_second FLOAT NOT NULL,
-     latency INTEGER NOT NULL, -- Stored in milliseconds
+     latency INTEGER NOT NULL,
      created_at TIMESTAMP DEFAULT current_timestamp(),
      INDEX (consumer_id),
-     INDEX (final_provider_id)
- );
- 
- -- Add provider type check constraint
- ALTER TABLE transactions
- ADD CONSTRAINT check_provider_type CHECK (
-     EXISTS (
-         SELECT 1 FROM users 
-         WHERE id = transactions.final_provider_id 
-         AND type = 'provider'
-     )
+     INDEX (final_provider_id),
+     FOREIGN KEY (final_provider_id) REFERENCES users(id),
+     CONSTRAINT final_provider_must_be_provider 
+         CHECK (final_provider_id IN (SELECT id FROM providers))
  );
  
```

---

_****_



Key changes:
1. Added DROP TABLE statements to clean up existing tables
2. Created a view for providers that we can reference in constraints
3. Changed the check constraint to use a simpler IN clause with the providers view
4. Separated the foreign key and provider type checks

Try running the schema again. This should work now since we're:
1. Cleaning up existing tables first
2. Using a simpler constraint mechanism
3. Using a view to abstract the provider check

Run:
```bash
cockroach sql --insecure --host=localhost --database=inferoute < schema.sql
```

---



---

