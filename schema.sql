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

-- Create system_settings table
CREATE TABLE IF NOT EXISTS system_settings (
    setting_key STRING PRIMARY KEY,
    setting_value STRING NOT NULL,
    description STRING NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp()
);

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type STRING NOT NULL CHECK (type IN ('consumer', 'provider')),
    username STRING UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp()
);

-- Providers table (renamed from provider_status)
CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name STRING NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT false,
    last_health_check TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    health_status STRING NOT NULL DEFAULT 'red' CHECK (health_status IN ('green', 'orange', 'red')),
    tier INT NOT NULL DEFAULT 3 CHECK (tier IN (1, 2, 3)),
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    api_url STRING,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, name)
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
    CHECK (max_output_price_tokens >= 0),
    UNIQUE (user_id, name)
);

-- API Keys table - now linked to providers/consumers instead of users
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES providers(id) ON DELETE CASCADE,
    consumer_id UUID REFERENCES consumers(id) ON DELETE CASCADE,
    api_key STRING UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK ((provider_id IS NULL AND consumer_id IS NOT NULL) OR (provider_id IS NOT NULL AND consumer_id IS NULL)),
    INDEX (provider_id),
    INDEX (consumer_id)
);

-- Balances table (for tracking consumer/provider funds)
CREATE TABLE IF NOT EXISTS balances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES providers(id) ON DELETE CASCADE,
    consumer_id UUID REFERENCES consumers(id) ON DELETE CASCADE,
    available_amount DECIMAL(18,8) NOT NULL DEFAULT 0,
    held_amount DECIMAL(18,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK ((provider_id IS NULL AND consumer_id IS NOT NULL) OR (provider_id IS NOT NULL AND consumer_id IS NULL)),
    CHECK (available_amount >= 0),
    CHECK (held_amount >= 0)
);

-- Provider Models table (for tracking which models each provider supports)
CREATE TABLE IF NOT EXISTS provider_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    model_name STRING NOT NULL,
    service_type STRING NOT NULL CHECK (service_type IN ('ollama', 'exolabs', 'llama_cpp')),
    input_price_tokens DECIMAL(18,8) NOT NULL,
    output_price_tokens DECIMAL(18,8) NOT NULL,
    is_active BOOLEAN DEFAULT true,
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
    status STRING NOT NULL CHECK (status IN ('pending', 'payment', 'completed', 'failed')),
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
    health_status STRING NOT NULL CHECK (health_status IN ('green', 'orange', 'red')),
    latency_ms INTEGER NOT NULL,
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