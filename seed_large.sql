-- Seed data for testing with 10,000 providers, each with a deepseek-r1:8b model
-- This file is for performance testing of provider selection

-- Clear existing data
TRUNCATE users, api_keys, providers, provider_models, provider_health_history, balances, system_settings, user_settings CASCADE;

-- Initialize system settings
INSERT INTO system_settings (setting_key, setting_value, description) VALUES
    ('fee_percentage', '5', 'Service fee percentage (5%)');

-- Create base users (we'll create more in batches)
INSERT INTO users (id, username, created_at, updated_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'enterprise_user', NOW(), NOW()),
    ('22222222-2222-2222-2222-222222222222', 'business_user', NOW(), NOW()),
    ('33333333-3333-3333-3333-333333333333', 'startup_user', NOW(), NOW());

-- Create consumers
INSERT INTO consumers (id, user_id, name, max_input_price_tokens, max_output_price_tokens, created_at, updated_at) VALUES
    ('11111111-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111', 'Enterprise Consumer', 1.0, 2.0, NOW(), NOW()),
    ('22222222-2222-2222-2222-222222222222', '22222222-2222-2222-2222-222222222222', 'Business Consumer', 0.8, 1.5, NOW(), NOW()),
    ('33333333-3333-3333-3333-333333333333', '33333333-3333-3333-3333-333333333333', 'Startup Consumer', 0.5, 1.0, NOW(), NOW());

-- Create consumer API keys
INSERT INTO api_keys (id, consumer_id, api_key, is_active, created_at, updated_at) VALUES
    ('77777777-7777-7777-7777-777777777777', '11111111-1111-1111-1111-111111111111', 'test_key_enterprise', true, NOW(), NOW()),
    ('88888888-8888-8888-8888-888888888888', '22222222-2222-2222-2222-222222222222', 'test_key_business', true, NOW(), NOW()),
    ('99999999-9999-9999-9999-999999999999', '33333333-3333-3333-3333-333333333333', 'test_key_startup', true, NOW(), NOW());

-- Create balances for consumer users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 10000.00, 0.00, NOW(), NOW()), -- Enterprise user
    ('22222222-2222-2222-2222-222222222222', 5000.00, 0.00, NOW(), NOW()),  -- Business user
    ('33333333-3333-3333-3333-333333333333', 1000.00, 0.00, NOW(), NOW());  -- Startup user

-- Now create 10,000 providers and models in batches of 1,000

-- Batch 1: Create 1,000 provider users (1-1000)
WITH number_series AS (
    SELECT generate_series(1, 1000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 1: Create providers for the users we just created
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT, -- Random tier between 1-3
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
LIMIT 1000;

-- Batch 1: Create provider models
INSERT INTO provider_models (
    id, 
    provider_id, 
    model_name, 
    service_type, 
    input_price_tokens, 
    output_price_tokens, 
    average_tps, 
    transaction_count, 
    is_active, 
    created_at, 
    updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
LIMIT 1000;

-- Batch 1: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
LIMIT 1000;

-- Create an API key for one provider (for testing)
INSERT INTO api_keys (id, provider_id, api_key, is_active, created_at, updated_at)
SELECT
    gen_random_uuid(),
    id,
    'test_provider_key_' || id,
    true,
    NOW(),
    NOW()
FROM providers
LIMIT 1;

-- Repeat for batches 2-10
-- Batch 2: Create 1,000 provider users (1001-2000)
WITH number_series AS (
    SELECT generate_series(1001, 2000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 2: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 2: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 2: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 3: Create 1,000 provider users (2001-3000)
WITH number_series AS (
    SELECT generate_series(2001, 3000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 3: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 3: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 3: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 4: Create 1,000 provider users (3001-4000)
WITH number_series AS (
    SELECT generate_series(3001, 4000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 4: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 4: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 4: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 5: Create 1,000 provider users (4001-5000)
WITH number_series AS (
    SELECT generate_series(4001, 5000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 5: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 5: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 5: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 6: Create 1,000 provider users (5001-6000)
WITH number_series AS (
    SELECT generate_series(5001, 6000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 6: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 6: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 6: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 7: Create 1,000 provider users (6001-7000)
WITH number_series AS (
    SELECT generate_series(6001, 7000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 7: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 7: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 7: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 8: Create 1,000 provider users (7001-8000)
WITH number_series AS (
    SELECT generate_series(7001, 8000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 8: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 8: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 8: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 9: Create 1,000 provider users (8001-9000)
WITH number_series AS (
    SELECT generate_series(8001, 9000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 9: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 9: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 9: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Batch 10: Create 1,000 provider users (9001-10000)
WITH number_series AS (
    SELECT generate_series(9001, 10000) AS n
)
INSERT INTO users (id, username, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    'provider_' || n,
    NOW(),
    NOW()
FROM number_series;

-- Batch 10: Create providers
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at)
SELECT 
    gen_random_uuid(),
    id,
    'Provider ' || username,
    true,
    'red',
    (random() * 2 + 1)::INT,
    false,
    'https://provider-' || id || '.example.com',
    NOW(),
    NOW()
FROM users
WHERE username LIKE 'provider_%'
AND id NOT IN (SELECT user_id FROM providers)
LIMIT 1000;

-- Batch 10: Create provider models
INSERT INTO provider_models (
    id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, 
    average_tps, transaction_count, is_active, created_at, updated_at
)
SELECT
    gen_random_uuid(),
    id,
    'deepseek-r1:8b',
    'ollama',
    (0.1 + (random() * 0.9))::DECIMAL(18,8),
    (0.2 + (random() * 1.8))::DECIMAL(18,8),
    (20 + (random() * 30))::DECIMAL(18,8),
    floor(random() * 1000)::INT,
    true,
    NOW() - (random() * INTERVAL '30 days'),
    NOW() - (random() * INTERVAL '7 days')
FROM providers
WHERE id NOT IN (SELECT provider_id FROM provider_models)
LIMIT 1000;

-- Batch 10: Create balances for provider users
INSERT INTO balances (user_id, available_amount, held_amount, created_at, updated_at)
SELECT
    user_id,
    (1000 + (random() * 4000))::DECIMAL(18,2),
    0.00,
    NOW(),
    NOW()
FROM providers
WHERE user_id NOT IN (SELECT user_id FROM balances)
LIMIT 1000;

-- Create user settings for all users
INSERT INTO user_settings (user_id, max_input_price_tokens, max_output_price_tokens, default_to_own_models, created_at, updated_at)
SELECT
    id,
    1.0,
    2.0,
    TRUE,
    NOW(),
    NOW()
FROM users
WHERE id NOT IN (SELECT user_id FROM user_settings); 