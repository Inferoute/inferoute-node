-- Drop index
DROP INDEX IF EXISTS idx_api_keys_lookup;

-- Drop lookup_key column
ALTER TABLE api_keys DROP COLUMN lookup_key; 