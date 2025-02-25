-- Seed data for testing

-- Clear existing data
TRUNCATE users, api_keys, providers, provider_models, provider_health_history, balances, system_settings, user_settings CASCADE;

-- Initialize system settings
INSERT INTO system_settings (setting_key, setting_value, description) VALUES
    ('fee_percentage', '5', 'Service fee percentage (5%)');

-- Create test users
INSERT INTO users (id, username, created_at, updated_at) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'tier1_provider', NOW(), NOW()),     -- Ultra reliable
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'tier2_provider_a', NOW(), NOW()),   -- Very reliable
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'tier2_provider_b', NOW(), NOW()),   -- Very reliable with occasional issues
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'tier3_provider_a', NOW(), NOW()),   -- Less reliable
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'tier3_provider_b', NOW(), NOW()),   -- Unreliable
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 'new_provider', NOW(), NOW()),        -- New provider, no history
    ('44444444-4444-4444-4444-444444444444', 'individual_user', NOW(), NOW()),
    ('11111111-1111-1111-1111-111111111111', 'enterprise_user', NOW(), NOW()),
    ('22222222-2222-2222-2222-222222222222', 'business_user', NOW(), NOW()),
    ('33333333-3333-3333-3333-333333333333', 'startup_user', NOW(), NOW());

-- Insert user settings for providers
INSERT INTO user_settings (user_id, max_input_price_tokens, max_output_price_tokens, default_to_own_models, created_at, updated_at) VALUES
    -- Provider users (higher limits since they're providers)
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 2.0, 4.0, TRUE, NOW(), NOW()),  -- Tier 1 provider
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 1.5, 3.0, TRUE, NOW(), NOW()),  -- Tier 2 provider A
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 1.5, 3.0, TRUE, NOW(), NOW()),  -- Tier 2 provider B
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 1.0, 2.0, TRUE, NOW(), NOW()),  -- Tier 3 provider A
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 1.0, 2.0, TRUE, NOW(), NOW()),  -- Tier 3 provider B
    ('44444444-4444-4444-4444-444444444444', 1.0, 2.0, TRUE, NOW(), NOW()),  -- Tier 3 provider B
    ('11111111-1111-1111-1111-111111111111', 1.0, 2.0, FALSE, NOW(), NOW()),  -- Enterprise user
    ('22222222-2222-2222-2222-222222222222', 0.8, 1.5, FALSE, NOW(), NOW()),  -- Business user
    ('33333333-3333-3333-3333-333333333333', 0.5, 1.0, FALSE, NOW(), NOW()),  -- Startup user
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 0.5, 1.0, TRUE, NOW(), NOW());   -- New provider (lower limits initially)

-- Create providers FIRST (before api_keys that reference them)
INSERT INTO providers (id, user_id, name, is_available, health_status, tier, paused, api_url, created_at, updated_at) VALUES
    -- Tier 1 Provider (Premium)
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '44444444-4444-4444-4444-444444444444', 'Tier 1 Provider', true, 'green', 1, false, 'https://6a2f-2a02-c7c-a0c9-5000-127c-61ff-fe4b-7035.ngrok-free.app', NOW(), NOW()),   -- Tier 1
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'Tier 2 Provider A', true, 'green', 2, false, 'https://6a2f-2a02-c7c-a0c9-5000-127c-61ff-fe4b-7035.ngrok-free.app', NOW(), NOW()),   -- Tier 2
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'Tier 2 Provider B', true, 'green', 2, false, 'https://6a2f-2a02-c7c-a0c9-5000-127c-61ff-fe4b-7035.ngrok-free.app', NOW(), NOW()),  -- Tier 2
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'Tier 3 Provider A', true, 'green', 3, false, 'https://6a2f-2a02-c7c-a0c9-5000-127c-61ff-fe4b-7035.ngrok-free.app', NOW(), NOW()),   -- Tier 3
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'Tier 3 Provider B', true, 'orange', 3, false, 'https://6a2f-2a02-c7c-a0c9-5000-127c-61ff-fe4b-7035.ngrok-free.app', NOW(), NOW()),     -- Tier 3
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'New Provider', true, 'orange', 3, false, null, NOW(), NOW());   -- Starting at Tier 3, no URL yet

-- NOW create API keys for providers (after providers exist)
INSERT INTO api_keys (id, provider_id, api_key, is_active, created_at, updated_at) VALUES
    ('11111111-1111-1111-1111-111111111111', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'test_key_tier1', true, NOW(), NOW()),
    ('22222222-2222-2222-2222-222222222222', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test_key_tier2a', true, NOW(), NOW()),
    ('33333333-3333-3333-3333-333333333333', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'test_key_tier2b', true, NOW(), NOW()),
    ('44444444-4444-4444-4444-444444444444', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'test_key_tier3a', true, NOW(), NOW()),
    ('55555555-5555-5555-5555-555555555555', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'test_key_tier3b', true, NOW(), NOW()),
    ('66666666-6666-6666-6666-666666666666', 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'test_key_new', true, NOW(), NOW());

-- Create provider models
INSERT INTO provider_models (id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, average_tps, transaction_count, is_active, created_at, updated_at) VALUES
    -- Tier 1 Provider (Premium models)
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'deepseek-r1:8b', 'ollama', 0.15, 0.3, 35.5, 1250, true, NOW(), NOW()),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaab', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'claude-3-opus', 'ollama', 0.15, 0.35, 42.8, 980, true, NOW(), NOW()),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaac', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'gemini-pro', 'ollama', 0.8, 0.25, 38.2, 850, true, NOW(), NOW()),

    -- Tier 2 Provider A (Mix of models)
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'deepseek-r1:8b', 'ollama', 0.5, 0.15, 28.4, 750, true, NOW(), NOW()),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbc', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'claude-2', 'ollama', 0.6, 0.18, 25.6, 620, true, NOW(), NOW()),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbd', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'mistral-medium', 'ollama', 0.4, 0.12, 31.2, 580, true, NOW(), NOW()),

    -- Tier 2 Provider B
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'deepseek-r1:8b', 'ollama', 0.45, 0.14, 26.8, 480, true, NOW(), NOW()),
    ('cccccccc-cccc-cccc-cccc-ccccccccccca', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'mistral-small', 'ollama', 0.3, 0.9, 33.5, 420, true, NOW(), NOW()),
    ('cccccccc-cccc-cccc-cccc-cccccccccccb', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'llama3.2', 'ollama', 0.2, 0.6, 29.1, 390, true, NOW(), NOW()),

    -- Tier 3 Provider A (Basic models)
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'deepseek-r1:32b', 'ollama', 0.2, 0.6, 22.4, 280, true, NOW(), NOW()),
    ('dddddddd-dddd-dddd-dddd-ddddddddddde', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'llama3.2', 'ollama', 0.15, 0.45, 24.8, 250, true, NOW(), NOW()),

    -- Tier 3 Provider B
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'mistral-small', 'ollama', 0.18, 0.5, 20.5, 180, true, NOW(), NOW()),
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeef', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'llama3.2', 'ollama', 0.1, 0.3, 18.9, 150, true, NOW(), NOW()),

    -- New Provider (Starting with basic models)
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'mistral-small', 'ollama', 0.2, 0.6, 15.2, 25, true, NOW(), NOW()),
    ('ffffffff-ffff-ffff-ffff-fffffffffff0', 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'llama3.2', 'ollama', 0.15, 0.45, 16.8, 18, true, NOW(), NOW());

-- Create health history for the last 30 days
-- Tier 1 Provider (99.9% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time, created_at, updated_at)
SELECT 
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    CASE 
        WHEN random() > 0.999 THEN 'orange'  -- Only 0.1% chance of non-green
        ELSE 'green' 
    END,
    floor(random() * 20 + 10)::int,  -- 10-30ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720)),  -- Last 30 days, hourly checks
    NOW(), NOW()
WHERE random() < 0.99;  -- 99% check success rate

-- Tier 2 Provider A (97% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time, created_at, updated_at)
SELECT 
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
    CASE 
        WHEN random() > 0.97 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 30 + 15)::int,  -- 15-45ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720)),
    NOW(), NOW()
WHERE random() < 0.98;  -- 98% check success rate

-- Tier 2 Provider B (95% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time, created_at, updated_at)
SELECT 
    'cccccccc-cccc-cccc-cccc-cccccccccccc',
    CASE 
        WHEN random() > 0.95 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 35 + 20)::int,  -- 20-55ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720)),
    NOW(), NOW()
WHERE random() < 0.97;  -- 97% check success rate

-- Tier 3 Provider A (90% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time, created_at, updated_at)
SELECT 
    'dddddddd-dddd-dddd-dddd-dddddddddddd',
    CASE 
        WHEN random() > 0.95 THEN 'red'
        WHEN random() > 0.90 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 40 + 25)::int,  -- 25-65ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720)),
    NOW(), NOW()
WHERE random() < 0.95;  -- 95% check success rate

-- Tier 3 Provider B (85% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time, created_at, updated_at)
SELECT 
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
    CASE 
        WHEN random() > 0.90 THEN 'red'
        WHEN random() > 0.85 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 50 + 30)::int,  -- 30-80ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720)),
    NOW(), NOW()
WHERE random() < 0.90;  -- 90% check success rate

-- New Provider (Just starting, only a few hours of history)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time, created_at, updated_at)
SELECT 
    'ffffffff-ffff-ffff-ffff-ffffffffffff',
    CASE 
        WHEN random() > 0.98 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 30 + 20)::int,  -- 20-50ms latency
    NOW() - (interval '1 hour' * generate_series(0, 24)),  -- Only 24 hours of history
    NOW(), NOW()
WHERE random() < 0.99;  -- 99% check success rate for initial period


-- Then create consumers
INSERT INTO consumers (id, user_id, name, max_input_price_tokens, max_output_price_tokens, created_at, updated_at) VALUES
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', '11111111-1111-1111-1111-111111111111', 'Enterprise: highest price tolerance', 1.0, 2.0, NOW(), NOW()),
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', '22222222-2222-2222-2222-222222222222', 'Business: high price tolerance', 0.8, 1.5, NOW(), NOW()),
    ('77777777-7777-7777-7777-777777777777', '33333333-3333-3333-3333-333333333333', 'Startup: medium price tolerance', 0.5, 1.0, NOW(), NOW()),
    ('66666666-6666-6666-6666-666666666666', '44444444-4444-4444-4444-444444444444', 'Individual: budget conscious', 0.3, 0.6, NOW(), NOW());

-- Then create API keys for consumers (after consumers exist)
INSERT INTO api_keys (id, consumer_id, api_key, is_active, created_at, updated_at) VALUES
    ('aaaaaaaa-1111-2222-3333-444444444444', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'consumer_key_enterprise', true, NOW(), NOW()),
    ('bbbbbbbb-1111-2222-3333-444444444444', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'consumer_key_business', true, NOW(), NOW()),
    ('cccccccc-1111-2222-3333-444444444444', '77777777-7777-7777-7777-777777777777', 'consumer_key_startup', true, NOW(), NOW()),
    ('dddddddd-1111-2222-3333-444444444444', '66666666-6666-6666-6666-666666666666', 'consumer_key_individual', true, NOW(), NOW());

-- Set up model-specific price settings for consumers
INSERT INTO consumer_models (id, consumer_id, model_name, max_input_price_tokens, max_output_price_tokens, created_at, updated_at) VALUES
    -- Enterprise user model preferences
    ('aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'llama3.2', 0.9, 1.8, NOW(), NOW()),
    ('aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeef', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'deepseek-r1:8b', 0.8, 1.6, NOW(), NOW()),
    
    -- Business user model preferences
    ('bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'llama3.2', 0.7, 1.4, NOW(), NOW()),
    ('bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeef', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'deepseek-r1:8b', 0.6, 1.2, NOW(), NOW()),
    
    -- Startup user model preferences
    ('cccccccc-cccc-cccc-dddd-eeeeeeeeeeee', '77777777-7777-7777-7777-777777777777', 'mistral-medium', 0.4, 0.8, NOW(), NOW()),
    ('cccccccc-cccc-cccc-dddd-eeeeeeeeeeef', '77777777-7777-7777-7777-777777777777', 'llama3.2', 0.3, 0.6, NOW(), NOW()),
    
    -- Individual user model preferences
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', '66666666-6666-6666-6666-666666666666', 'deepseek-r1:8b', 0.2, 0.4, NOW(), NOW()),
    ('dddddddd-dddd-dddd-dddd-dddddddddddf', '66666666-6666-6666-6666-666666666666', 'llama3.2', 0.2, 0.31, NOW(), NOW());

-- Set up initial balances for users
INSERT INTO balances (id, user_id, available_amount, held_amount, created_at, updated_at) VALUES
    -- Consumer users
    ('11111111-2222-3333-4444-555555555555', '11111111-1111-1111-1111-111111111111', 10000.00, 0.00, NOW(), NOW()),  -- Enterprise user
    ('22222222-3333-4444-5555-666666666666', '22222222-2222-2222-2222-222222222222', 5000.00, 0.00, NOW(), NOW()),   -- Business user
    ('33333333-4444-5555-6666-777777777777', '33333333-3333-3333-3333-333333333333', 1000.00, 0.00, NOW(), NOW()),   -- Startup user
    ('44444444-5555-6666-7777-888888888888', '44444444-4444-4444-4444-444444444444', 100.00, 0.00, NOW(), NOW()),    -- Individual user
    
    -- Provider users
    ('55555555-6666-7777-8888-999999999999', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 5000.00, 0.00, NOW(), NOW()),  -- Tier 1 provider user
    ('66666666-7777-8888-9999-aaaaaaaaaaaa', 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 2500.00, 0.00, NOW(), NOW()),  -- Tier 2 provider user
    ('77777777-8888-9999-aaaa-bbbbbbbbbbbb', 'cccccccc-cccc-cccc-cccc-cccccccccccc', 1000.00, 0.00, NOW(), NOW()),  -- Tier 2 provider user
    ('88888888-9999-aaaa-bbbb-cccccccccccc', 'ffffffff-ffff-ffff-ffff-ffffffffffff', 0.00, 0.00, NOW(), NOW()),     -- New provider user
    ('99999999-aaaa-bbbb-cccc-dddddddddddd', 'dddddddd-dddd-dddd-dddd-dddddddddddd', 750.00, 0.00, NOW(), NOW()),   -- Tier 3 provider user
    ('aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 500.00, 0.00, NOW(), NOW());   -- Tier 3 provider user

-- Add some dummy transactions with different states
INSERT INTO transactions (
    id, consumer_id, provider_id, hmac, model_name,
    total_input_tokens, total_output_tokens, tokens_per_second, latency,
    consumer_cost, provider_earnings, service_fee, status, created_at,
    input_price_tokens, output_price_tokens
) VALUES
    -- Completed transactions
    (
        gen_random_uuid(),
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'hmac_test_1',
        'gpt-4-turbo',
        100,
        150,
        10.5,
        250,
        0.0075,
        0.006,
        0.0015,
        'completed',
        NOW() - interval '1 hour',
        0.15,  -- $0.15 per million input tokens (from provider_models)
        0.30   -- $0.30 per million output tokens (from provider_models)
    ),
    (
        gen_random_uuid(),
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'hmac_test_2',
        'gpt-3.5-turbo',
        200,
        300,
        15.2,
        180,
        0.0045,
        0.0035,
        0.001,
        'completed',
        NOW() - interval '30 minutes',
        0.05,  -- $0.05 per million input tokens
        0.15   -- $0.15 per million output tokens
    ),
    -- Pending transaction (for HMAC validation testing)
    (
        gen_random_uuid(),
        '77777777-7777-7777-7777-777777777777',
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
        'test_pending_hmac',
        'mistral-medium',
        150,
        0,
        0,
        0,
        0,
        0,
        0,
        'pending',
        NOW() - interval '1 minute',
        0.04,  -- $0.04 per million input tokens
        0.12   -- $0.12 per million output tokens
    ),
    -- Failed transaction
    (
        gen_random_uuid(),
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'hmac_test_failed',
        'llama-2',
        50,
        0,
        0,
        500,
        0,
        0,
        0,
        'failed',
        NOW() - interval '15 minutes',
        0.15,  -- $0.15 per million input tokens
        0.45   -- $0.45 per million output tokens
    ),
    -- Multi-provider transaction (showing provider selection)
    (
        gen_random_uuid(),
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'hmac_test_multi',
        'claude-3-opus',
        300,
        450,
        20.5,
        220,
        0.015,
        0.012,
        0.003,
        'completed',
        NOW() - interval '5 minutes',
        0.15,  -- $0.15 per million input tokens
        0.35   -- $0.35 per million output tokens
    ),
    -- Transaction in payment state (waiting for payment processing)
    (
        gen_random_uuid(),
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'hmac_payment_pending',
        'gpt-3.5-turbo',
        150,
        200,
        NULL,
        180,
        NULL,
        NULL,
        NULL,
        'payment',
        NOW() - interval '30 seconds',
        0.05,  -- $0.05 per million input tokens
        0.15   -- $0.15 per million output tokens
    )
    UNION ALL
    SELECT
        gen_random_uuid(),
        'cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid,
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid,
        'hmac_payment_test_' || generate_series::text,
        'gpt-3.5-turbo',
        50 + (random() * 450)::int,
        75 + (random() * 675)::int,
        NULL::float,
        180,
        NULL::decimal,
        NULL::decimal,
        NULL::decimal,
        'payment',
        NOW() - interval '1 second' * generate_series,
        0.05,  -- $0.05 per million input tokens
        0.15   -- $0.15 per million output tokens
    FROM generate_series(1, 100);
