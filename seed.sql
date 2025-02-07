-- Seed data for testing

-- Clear existing data
TRUNCATE users, api_keys, provider_status, provider_models, provider_health_history, balances, system_settings CASCADE;

-- Initialize system settings
INSERT INTO system_settings (setting_key, setting_value, description) VALUES
    ('fee_percentage', '5.0', 'Platform fee percentage taken from each transaction'),
    ('max_retry_count', '3', 'Maximum number of retries for failed requests');

-- Create test providers with different reliability patterns
INSERT INTO users (id, type, username) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'provider', 'tier1_provider'),     -- Ultra reliable
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'provider', 'tier2_provider_a'),   -- Very reliable
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', 'provider', 'tier2_provider_b'),   -- Very reliable with occasional issues
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', 'provider', 'tier3_provider_a'),   -- Less reliable
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'provider', 'tier3_provider_b'),   -- Unreliable
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', 'provider', 'new_provider');        -- New provider, no history

-- Create API keys for providers
INSERT INTO api_keys (id, user_id, api_key) VALUES
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'test_key_tier1'),
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'test_key_tier2a'),
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'test_key_tier2b'),
    (gen_random_uuid(), 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'test_key_tier3a'),
    (gen_random_uuid(), 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'test_key_tier3b'),
    (gen_random_uuid(), 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'test_key_new');

-- Initialize provider status
INSERT INTO provider_status (provider_id, is_available, health_status, tier, paused, api_url) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', true, 'green', 1, false, 'https://tier1-provider.example.com/api/compute'),   -- Tier 1
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', true, 'green', 2, false, 'https://tier2a-provider.example.com/api/compute'),   -- Tier 2
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', true, 'orange', 2, false, 'https://tier2b-provider.example.com/api/compute'),  -- Tier 2
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', true, 'green', 3, false, 'https://tier3a-provider.example.com/api/compute'),   -- Tier 3
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', true, 'red', 3, false, 'https://tier3b-provider.example.com/api/compute'),     -- Tier 3
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', true, 'green', 3, false, null);   -- Starting at Tier 3, no URL yet

-- Create provider models
INSERT INTO provider_models (id, provider_id, model_name, service_type, input_price_tokens, output_price_tokens, is_active) VALUES
    -- Tier 1 Provider (Premium models)
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'gpt-4-turbo', 'ollama', 0.15, 0.3, true),
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'claude-3-opus', 'ollama', 0.15, 0.35, true),
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'gemini-pro', 'ollama', 0.8, 0.25, true),

    -- Tier 2 Provider A (Mix of models)
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'gpt-3.5-turbo', 'ollama', 0.5, 0.15, true),
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'claude-2', 'ollama', 0.6, 0.18, true),
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'mistral-medium', 'ollama', 0.4, 0.12, true),

    -- Tier 2 Provider B
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'deepseek-r1:8b', 'ollama', 0.45, 0.14, true),
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'mistral-small', 'ollama', 0.3, 0.9, true),
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'llama-2', 'ollama', 0.2, 0.6, true),

    -- Tier 3 Provider A (Basic models)
    (gen_random_uuid(), 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'deepseek-r1:32b', 'ollama', 0.2, 0.6, true),
    (gen_random_uuid(), 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'llama-2', 'ollama', 0.15, 0.45, true),

    -- Tier 3 Provider B
    (gen_random_uuid(), 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'mistral-small', 'ollama', 0.18, 0.5, true),
    (gen_random_uuid(), 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'llama-2', 'ollama', 0.1, 0.3, true),

    -- New Provider (Starting with basic models)
    (gen_random_uuid(), 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'mistral-small', 'ollama', 0.2, 0.6, true),
    (gen_random_uuid(), 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'llama-2', 'ollama', 0.15, 0.45, true);

-- Create health history for the last 30 days
-- Tier 1 Provider (99.9% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time)
SELECT 
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    CASE 
        WHEN random() > 0.999 THEN 'orange'  -- Only 0.1% chance of non-green
        ELSE 'green' 
    END,
    floor(random() * 20 + 10)::int,  -- 10-30ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720))  -- Last 30 days, hourly checks
WHERE random() < 0.99;  -- 99% check success rate

-- Tier 2 Provider A (97% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time)
SELECT 
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
    CASE 
        WHEN random() > 0.97 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 30 + 15)::int,  -- 15-45ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720))
WHERE random() < 0.98;  -- 98% check success rate

-- Tier 2 Provider B (95% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time)
SELECT 
    'cccccccc-cccc-cccc-cccc-cccccccccccc',
    CASE 
        WHEN random() > 0.95 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 35 + 20)::int,  -- 20-55ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720))
WHERE random() < 0.97;  -- 97% check success rate

-- Tier 3 Provider A (90% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time)
SELECT 
    'dddddddd-dddd-dddd-dddd-dddddddddddd',
    CASE 
        WHEN random() > 0.95 THEN 'red'
        WHEN random() > 0.90 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 40 + 25)::int,  -- 25-65ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720))
WHERE random() < 0.95;  -- 95% check success rate

-- Tier 3 Provider B (85% uptime)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time)
SELECT 
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
    CASE 
        WHEN random() > 0.90 THEN 'red'
        WHEN random() > 0.85 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 50 + 30)::int,  -- 30-80ms latency
    NOW() - (interval '1 hour' * generate_series(0, 720))
WHERE random() < 0.90;  -- 90% check success rate

-- New Provider (Just starting, only a few hours of history)
INSERT INTO provider_health_history (provider_id, health_status, latency_ms, health_check_time)
SELECT 
    'ffffffff-ffff-ffff-ffff-ffffffffffff',
    CASE 
        WHEN random() > 0.98 THEN 'orange'
        ELSE 'green' 
    END,
    floor(random() * 30 + 20)::int,  -- 20-50ms latency
    NOW() - (interval '1 hour' * generate_series(0, 24))  -- Only 24 hours of history
WHERE random() < 0.99;  -- 99% check success rate for initial period

-- Create some test consumers
INSERT INTO users (id, type, username) VALUES
    ('11111111-1111-1111-1111-111111111111', 'consumer', 'test_consumer_a'),
    ('22222222-2222-2222-2222-222222222222', 'consumer', 'test_consumer_b'),
    ('33333333-3333-3333-3333-333333333333', 'consumer', 'test_consumer_c');

-- Create API keys for consumers
INSERT INTO api_keys (id, user_id, api_key) VALUES
    (gen_random_uuid(), '11111111-1111-1111-1111-111111111111', 'test_consumer_key_a'),
    (gen_random_uuid(), '22222222-2222-2222-2222-222222222222', 'test_consumer_key_b'),
    (gen_random_uuid(), '33333333-3333-3333-3333-333333333333', 'test_consumer_key_c');

-- Add some dummy transactions with different states
INSERT INTO transactions (
    id, consumer_id, final_provider_id, providers, hmac, model_name,
    total_input_tokens, total_output_tokens, tokens_per_second, latency,
    consumer_cost, provider_earnings, service_fee, status, created_at,
    input_price_tokens, output_price_tokens
) VALUES
    -- Completed transactions
    (
        gen_random_uuid(),
        '11111111-1111-1111-1111-111111111111',
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        ARRAY['aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid],
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
        '22222222-2222-2222-2222-222222222222',
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        ARRAY['bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid],
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
        '33333333-3333-3333-3333-333333333333',
        'cccccccc-cccc-cccc-cccc-cccccccccccc',
        ARRAY['cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid],
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
        '11111111-1111-1111-1111-111111111111',
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        ARRAY['dddddddd-dddd-dddd-dddd-dddddddddddd'::uuid],
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
        '22222222-2222-2222-2222-222222222222',
        'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        ARRAY[
            'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'::uuid,
            'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid,
            'cccccccc-cccc-cccc-cccc-cccccccccccc'::uuid
        ],
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
        '11111111-1111-1111-1111-111111111111',
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        ARRAY['bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid],
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
        '11111111-1111-1111-1111-111111111111'::uuid,
        'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid,
        ARRAY['bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'::uuid],
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

