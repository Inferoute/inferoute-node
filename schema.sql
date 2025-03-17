-- Create the inferoute database if it doesn't exist
CREATE DATABASE IF NOT EXISTS inferoute;
USE inferoute;

-- Drop existing tables if they exist (in correct order)
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS hmacs;
DROP TABLE IF EXISTS provider_models;
DROP TABLE IF EXISTS providers;
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS provider_health_history;
DROP TABLE IF EXISTS system_settings;
DROP TABLE IF EXISTS consumers;
DROP TABLE IF EXISTS consumer_models;
DROP TABLE IF EXISTS user_settings;
DROP TABLE IF EXISTS provider_cheating_incidents;
DROP TABLE IF EXISTS average_model_costs;
DROP TABLE IF EXISTS model_pricing_data;

-- Create system_settings table
CREATE TABLE IF NOT EXISTS system_settings (
    setting_key STRING PRIMARY KEY,
    setting_value STRING NOT NULL,
    description STRING NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp()
);

-- Insert default system settings
INSERT INTO system_settings (setting_key, setting_value, description)
VALUES 
('last_processed_transaction_time', '1970-01-01T00:00:00Z', 'Timestamp of the last processed transaction for model pricing data')
ON CONFLICT (setting_key) DO NOTHING;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username STRING UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp()
);

-- User Settings table
CREATE TABLE IF NOT EXISTS user_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    max_input_price_tokens DECIMAL(18,8) NOT NULL DEFAULT 1.0,
    max_output_price_tokens DECIMAL(18,8) NOT NULL DEFAULT 1.0,
    default_to_own_models BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    UNIQUE (user_id),
    CHECK (max_input_price_tokens >= 0),
    CHECK (max_output_price_tokens >= 0)
);


-- Providers table (renamed from provider_status)
CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name STRING NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT false,
    last_health_check TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    health_status STRING NOT NULL DEFAULT 'red' CHECK (health_status IN ('green', 'red')),
    tier INT NOT NULL DEFAULT 3 CHECK (tier IN (1, 2, 3)),
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    api_url STRING,
    provider_type STRING DEFAULT 'ollama' CHECK (provider_type IN ('ollama', 'exolabs', 'llama_cpp')),
    product_name STRING,
    driver_version STRING,
    cuda_version STRING,
    gpu_count INTEGER DEFAULT 1,
    memory_total INTEGER,
    memory_free INTEGER,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add index for faster querying of non-paused providers
CREATE INDEX idx_providers_paused ON providers(paused) WHERE NOT paused;

-- Consumers table
CREATE TABLE IF NOT EXISTS consumers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name STRING NOT NULL,
    max_input_price_tokens DECIMAL(18,8) NOT NULL DEFAULT 1.0,
    max_output_price_tokens DECIMAL(18,8) NOT NULL DEFAULT 1.0,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK (max_input_price_tokens >= 0),
    CHECK (max_output_price_tokens >= 0)
);

-- API Keys table - now linked to providers/consumers instead of users
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES providers(id) ON DELETE CASCADE,
    consumer_id UUID REFERENCES consumers(id) ON DELETE CASCADE,
    api_key STRING UNIQUE NOT NULL,
    description STRING,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK ((provider_id IS NULL AND consumer_id IS NOT NULL) OR (provider_id IS NOT NULL AND consumer_id IS NULL)),
    INDEX (provider_id),
    INDEX (consumer_id)
);

-- Balances table (for tracking user funds)
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

-- Add index for faster balance lookups
CREATE INDEX idx_balances_user_id ON balances(user_id);

-- Provider Models table (for tracking which models each provider supports)
CREATE TABLE IF NOT EXISTS provider_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_name STRING NOT NULL,
    service_type STRING NOT NULL CHECK (service_type IN ('ollama', 'exolabs', 'llama_cpp')),
    input_price_tokens DECIMAL(18,8) NOT NULL,
    output_price_tokens DECIMAL(18,8) NOT NULL,
    average_tps DECIMAL(18,8) DEFAULT 0,
    transaction_count INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    model_created TIMESTAMP,
    model_owned_by STRING,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    UNIQUE (provider_id, model_name),
    INDEX (provider_id)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_id UUID NOT NULL REFERENCES consumers(id),
    provider_id UUID NOT NULL REFERENCES providers(id),
    hmac STRING UNIQUE NOT NULL,
    model_name STRING NOT NULL,
    input_price_tokens DECIMAL(18,8) NOT NULL, 
    output_price_tokens DECIMAL(18,8) NOT NULL,
    total_input_tokens INTEGER,
    total_output_tokens INTEGER,
    tokens_per_second FLOAT,
    latency INTEGER,
    consumer_cost DECIMAL(18,8),
    provider_earnings DECIMAL(18,8),
    service_fee DECIMAL(18,8),
    status STRING NOT NULL CHECK (status IN ('pending', 'payment', 'completed', 'failed', 'cheating_detected', 'canceled')),
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    INDEX (consumer_id),
    INDEX (provider_id),
    INDEX (hmac)
);

-- Create provider_health_history table
CREATE TABLE IF NOT EXISTS provider_health_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    health_status STRING NOT NULL CHECK (health_status IN ('green', 'red')),
    latency_ms INTEGER NOT NULL,
    gpu_utilization INTEGER,
    memory_used INTEGER,
    memory_total INTEGER,
    health_check_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    INDEX (provider_id, health_check_time DESC)
);

-- Create consumer_models table for model-specific price settings
CREATE TABLE IF NOT EXISTS consumer_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_id UUID NOT NULL REFERENCES consumers(id) ON DELETE CASCADE,
    model_name STRING NOT NULL,
    max_input_price_tokens DECIMAL(18,8) NOT NULL,
    max_output_price_tokens DECIMAL(18,8) NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    UNIQUE (consumer_id, model_name),
    CHECK (max_input_price_tokens >= 0),
    CHECK (max_output_price_tokens >= 0)
);

-- Create provider_cheating_incidents table
CREATE TABLE IF NOT EXISTS provider_cheating_incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_id UUID NOT NULL REFERENCES providers(id),
    provider_model_id UUID NOT NULL REFERENCES provider_models(id),
    transaction_hmac TEXT NOT NULL,
    transaction_created_at TIMESTAMP NOT NULL,
    transaction_updated_at TIMESTAMP NOT NULL,
    model_updated_at TIMESTAMP NOT NULL,
    input_price_tokens DECIMAL(10, 6) NOT NULL,
    output_price_tokens DECIMAL(10, 6) NOT NULL,
    total_input_tokens INTEGER NOT NULL,
    total_output_tokens INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_provider_cheating_incidents_provider FOREIGN KEY (provider_id) REFERENCES providers(id),
    CONSTRAINT fk_provider_cheating_incidents_provider_model FOREIGN KEY (provider_model_id) REFERENCES provider_models(id)
);

-- Create average_model_costs table
CREATE TABLE IF NOT EXISTS average_model_costs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_name STRING NOT NULL UNIQUE,
    avg_input_price_tokens DECIMAL(18,8) NOT NULL,
    avg_output_price_tokens DECIMAL(18,8) NOT NULL,
    sample_size INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK (avg_input_price_tokens >= 0),
    CHECK (avg_output_price_tokens >= 0),
    CHECK (sample_size >= 0)
);

-- Insert default model costs
INSERT INTO average_model_costs (model_name, avg_input_price_tokens, avg_output_price_tokens, sample_size)
VALUES ('default', 0.0005, 0.0005, 1)
ON CONFLICT (model_name) DO NOTHING;

-- Create model_pricing_data table for candlestick chart data
CREATE TABLE IF NOT EXISTS model_pricing_data (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_name STRING NOT NULL,
    timestamp TIMESTAMP NOT NULL DEFAULT current_timestamp(),
    input_open DECIMAL(18,8) NOT NULL,
    input_high DECIMAL(18,8) NOT NULL,
    input_low DECIMAL(18,8) NOT NULL,
    input_close DECIMAL(18,8) NOT NULL,
    output_open DECIMAL(18,8) NOT NULL,
    output_high DECIMAL(18,8) NOT NULL,
    output_low DECIMAL(18,8) NOT NULL,
    output_close DECIMAL(18,8) NOT NULL,
    volume_input INTEGER NOT NULL DEFAULT 0,
    volume_output INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    INDEX (model_name, timestamp DESC)
);

-- Insert default model pricing data
INSERT INTO model_pricing_data (
    model_name, timestamp, 
    input_open, input_high, input_low, input_close,
    output_open, output_high, output_low, output_close,
    volume_input, volume_output
)
VALUES (
    'default', '1942-01-01 20:42:42', 
    0.00050000, 0.00050000, 0.00050000, 0.00050000,
    0.00050000, 0.00050000, 0.00050000, 0.00050000,
    42000, 42000
)
ON CONFLICT DO NOTHING;

-- Create triggers to update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = current_timestamp();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply the trigger to all tables
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_providers_updated_at
    BEFORE UPDATE ON providers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_consumers_updated_at
    BEFORE UPDATE ON consumers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_balances_updated_at
    BEFORE UPDATE ON balances
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_provider_models_updated_at
    BEFORE UPDATE ON provider_models
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_transactions_updated_at
    BEFORE UPDATE ON transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_provider_health_history_updated_at
    BEFORE UPDATE ON provider_health_history
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_consumer_models_updated_at
    BEFORE UPDATE ON consumer_models
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE INDEX IF NOT EXISTS idx_provider_cheating_incidents_provider_id ON provider_cheating_incidents(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_cheating_incidents_transaction_hmac ON provider_cheating_incidents(transaction_hmac);
