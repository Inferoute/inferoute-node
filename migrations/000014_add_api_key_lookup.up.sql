-- Add lookup_key column
ALTER TABLE api_keys ADD COLUMN lookup_key VARCHAR(8);

-- Create index on lookup_key
CREATE INDEX idx_api_keys_lookup ON api_keys(lookup_key);

-- Update existing keys with NULL lookup_key to be inactive
UPDATE api_keys SET is_active = false WHERE lookup_key IS NULL; 