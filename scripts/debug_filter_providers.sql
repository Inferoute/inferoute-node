-- Debug why GET /api/health/providers/filter?max_cost=0.9&model_name=deepseek-r1:8b returns null
-- Run: cockroach sql --insecure --host=localhost --database=inferoute -f scripts/debug_filter_providers.sql
-- Or with Docker: docker exec -it cockroachdb cockroach sql --insecure -d inferoute < scripts/debug_filter_providers.sql
-- (from repo root; or copy-paste sections into cockroach sql)

-- 1. Providers (must have health_status != red, is_available=true, NOT paused)
SELECT id, user_id, name, is_available, health_status, paused, tier
FROM providers
ORDER BY tier, name;

-- 2. Provider models for deepseek-r1:8b (must have is_active=true, input/output <= 0.9)
SELECT pm.provider_id, p.name, pm.model_name, pm.input_price_tokens, pm.output_price_tokens, pm.is_active,
       pm.input_price_tokens <= 0.9 AND pm.output_price_tokens <= 0.9 AS within_max_cost
FROM provider_models pm
JOIN providers p ON p.id = pm.provider_id
WHERE pm.model_name = 'deepseek-r1:8b' OR pm.model_name = 'deepseek-r1:8b:latest'
ORDER BY pm.provider_id;

-- 3. Exact filter query (same params: tier=NULL, model=deepseek-r1:8b, max_cost=0.9)
WITH healthy_providers AS (
    SELECT p.id AS provider_id, p.tier, p.health_status,
           u.username, COALESCE(p.api_url, '') AS api_url
    FROM providers p
    JOIN users u ON u.id = p.user_id
    WHERE p.health_status != 'red'
      AND p.is_available = true
      AND NOT p.paused
      AND (NULL::int IS NULL OR p.tier = NULL)
),
ranked_models AS (
    SELECT
        hp.provider_id,
        hp.username,
        hp.tier,
        hp.health_status,
        hp.api_url,
        pm.model_name,
        pm.input_price_tokens,
        pm.output_price_tokens,
        pm.average_tps,
        RANK() OVER (
            PARTITION BY hp.provider_id
            ORDER BY (pm.input_price_tokens + pm.output_price_tokens) ASC
        ) AS cost_rank
    FROM healthy_providers hp
    JOIN provider_models pm ON pm.provider_id = hp.provider_id
    WHERE pm.is_active = true
      AND pm.input_price_tokens <= 0.9
      AND pm.output_price_tokens <= 0.9
      AND ('deepseek-r1:8b' = '' OR pm.model_name = 'deepseek-r1:8b' OR pm.model_name = 'deepseek-r1:8b' || ':latest')
)
SELECT provider_id, username, tier, health_status, api_url, model_name,
       input_price_tokens, output_price_tokens, average_tps
FROM ranked_models
WHERE ('deepseek-r1:8b' != '' OR cost_rank = 1)
ORDER BY tier ASC, average_tps DESC;

-- 4. All model names (in case of typo)
SELECT DISTINCT model_name FROM provider_models ORDER BY model_name;
