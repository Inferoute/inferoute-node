-- Create the inferoute database if it doesn't exist
CREATE DATABASE IF NOT EXISTS inferoute;
USE inferoute;

-- Drop existing tables if they exist (in correct order)
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS hmacs;
DROP TABLE IF EXISTS provider_models;
DROP TABLE IF EXISTS provider_status;
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS provider_health_history;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type STRING NOT NULL CHECK (type IN ('consumer', 'provider')),
    username STRING UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp()
);

-- Balances table (for tracking user funds)
CREATE TABLE IF NOT EXISTS balances (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    available_amount DECIMAL(18,8) NOT NULL DEFAULT 0,
    held_amount DECIMAL(18,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    CHECK (available_amount >= 0),
    CHECK (held_amount >= 0)
);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key STRING UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    INDEX (user_id)
);

-- Provider Status table (for health checks and availability)
CREATE TABLE IF NOT EXISTS provider_status (
    provider_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    is_available BOOLEAN NOT NULL DEFAULT false,
    last_health_check TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    health_status STRING NOT NULL DEFAULT 'red' CHECK (health_status IN ('green', 'orange', 'red')),
    tier INT NOT NULL DEFAULT 3 CHECK (tier IN (1, 2, 3)),
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add index for faster querying of non-paused providers
CREATE INDEX idx_provider_status_paused ON provider_status(paused) WHERE NOT paused;

-- Provider Models table (for tracking which models each provider supports)
CREATE TABLE IF NOT EXISTS provider_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_name STRING NOT NULL,
    service_type STRING NOT NULL CHECK (service_type IN ('ollama', 'exolabs', 'llama_cpp')),
    input_price_per_token DECIMAL(18,8) NOT NULL,
    output_price_per_token DECIMAL(18,8) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    UNIQUE (provider_id, model_name),
    INDEX (provider_id)
);

-- Transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_id UUID NOT NULL REFERENCES users(id),
    final_provider_id UUID NOT NULL REFERENCES users(id),
    providers UUID[] NOT NULL,
    hmac STRING UNIQUE NOT NULL,
    model_name STRING NOT NULL,
    total_input_tokens INTEGER NOT NULL,
    total_output_tokens INTEGER NOT NULL,
    tokens_per_second FLOAT NOT NULL,
    latency INTEGER NOT NULL,
    consumer_cost DECIMAL(18,8) NOT NULL,
    provider_earnings DECIMAL(18,8) NOT NULL,
    status STRING NOT NULL CHECK (status IN ('pending', 'completed', 'failed')),
    created_at TIMESTAMP DEFAULT current_timestamp(),
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    INDEX (consumer_id),
    INDEX (final_provider_id),
    INDEX (hmac)
);

-- Create provider_health_history table
CREATE TABLE IF NOT EXISTS provider_health_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    health_status STRING NOT NULL CHECK (health_status IN ('green', 'orange', 'red')),
    latency_ms INTEGER NOT NULL,
    health_check_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT current_timestamp(),
    INDEX (provider_id, health_check_time DESC)
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

CREATE TRIGGER update_balances_updated_at
    BEFORE UPDATE ON balances
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER update_provider_status_updated_at
    BEFORE UPDATE ON provider_status
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