-- Seed data for testing

-- Clear existing data
TRUNCATE users, api_keys, provider_status, provider_models, provider_health_history, balances CASCADE;

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
INSERT INTO provider_status (provider_id, is_available, health_status, tier, paused) VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', true, 'green', 1, false),   -- Tier 1
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', true, 'green', 2, false),   -- Tier 2
    ('cccccccc-cccc-cccc-cccc-cccccccccccc', true, 'orange', 2, false),  -- Tier 2
    ('dddddddd-dddd-dddd-dddd-dddddddddddd', true, 'green', 3, false),   -- Tier 3
    ('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', true, 'red', 3, false),     -- Tier 3
    ('ffffffff-ffff-ffff-ffff-ffffffffffff', true, 'green', 3, false);   -- Starting at Tier 3

-- Create provider models
INSERT INTO provider_models (id, provider_id, model_name, service_type, input_price_per_token, output_price_per_token, is_active) VALUES
    -- Tier 1 Provider (Premium models)
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'gpt-4-turbo', 'ollama', 0.01, 0.03, true),
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'claude-3-opus', 'ollama', 0.015, 0.035, true),
    (gen_random_uuid(), 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'gemini-pro', 'ollama', 0.008, 0.025, true),

    -- Tier 2 Provider A (Mix of models)
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'gpt-3.5-turbo', 'ollama', 0.005, 0.015, true),
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'claude-2', 'ollama', 0.006, 0.018, true),
    (gen_random_uuid(), 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', 'mistral-medium', 'ollama', 0.004, 0.012, true),

    -- Tier 2 Provider B
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'gpt-3.5-turbo', 'ollama', 0.0045, 0.014, true),
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'mistral-small', 'ollama', 0.003, 0.009, true),
    (gen_random_uuid(), 'cccccccc-cccc-cccc-cccc-cccccccccccc', 'llama-2', 'ollama', 0.002, 0.006, true),

    -- Tier 3 Provider A (Basic models)
    (gen_random_uuid(), 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'mistral-small', 'ollama', 0.002, 0.006, true),
    (gen_random_uuid(), 'dddddddd-dddd-dddd-dddd-dddddddddddd', 'llama-2', 'ollama', 0.0015, 0.0045, true),

    -- Tier 3 Provider B
    (gen_random_uuid(), 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'mistral-small', 'ollama', 0.0018, 0.005, true),
    (gen_random_uuid(), 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'llama-2', 'ollama', 0.001, 0.003, true),

    -- New Provider (Starting with basic models)
    (gen_random_uuid(), 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'mistral-small', 'ollama', 0.002, 0.006, true),
    (gen_random_uuid(), 'ffffffff-ffff-ffff-ffff-ffffffffffff', 'llama-2', 'ollama', 0.0015, 0.0045, true);

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