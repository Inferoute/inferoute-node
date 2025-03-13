#!/bin/bash

# model_cost_updater.sh
# This script updates provider model prices every 45 seconds to simulate market price changes
# It helps test the candlestick chart data collection functionality

# Load environment variables from .env file
if [ -f ../.env ]; then
    source ../.env
else
    echo "Error: .env file not found"
    exit 1
fi

# Database connection parameters from environment variables
DB_HOST=${DATABASE_HOST:-localhost}
DB_PORT=${DATABASE_PORT:-26257}
DB_NAME=${DATABASE_DBNAME:-inferoute}
DB_USER=${DATABASE_USER:-root}
DB_SSL_MODE=${DATABASE_SSLMODE:-disable}

# Function to execute SQL commands
execute_sql() {
    psql "postgresql://${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}" -c "$1"
}

# Function to generate a random price fluctuation (between -10% and +10%)
random_fluctuation() {
    # Generate a random number between -10 and 10
    local fluctuation=$(( (RANDOM % 21) - 10 ))
    echo "scale=4; ${fluctuation} / 100" | bc
}

# Function to update model prices with random fluctuations
update_model_prices() {
    echo "$(date): Updating model prices..."
    
    # Get a list of active provider models
    local models=$(execute_sql "SELECT id, model_name, input_price_tokens, output_price_tokens FROM provider_models WHERE is_active = true LIMIT 10;" -t | tr -d ' ')
    
    # Process each model
    echo "$models" | while IFS='|' read -r id model_name input_price output_price; do
        if [ -z "$id" ]; then
            continue
        fi
        
        # Calculate new prices with random fluctuations
        local input_fluctuation=$(random_fluctuation)
        local output_fluctuation=$(random_fluctuation)
        
        local new_input_price=$(echo "scale=8; ${input_price} * (1 + ${input_fluctuation})" | bc)
        local new_output_price=$(echo "scale=8; ${output_price} * (1 + ${output_fluctuation})" | bc)
        
        # Ensure prices don't go below a minimum threshold
        new_input_price=$(echo "scale=8; if(${new_input_price} < 0.00001) 0.00001 else ${new_input_price}" | bc)
        new_output_price=$(echo "scale=8; if(${new_output_price} < 0.00001) 0.00001 else ${new_output_price}" | bc)
        
        # Update the model prices
        execute_sql "UPDATE provider_models SET input_price_tokens = ${new_input_price}, output_price_tokens = ${new_output_price}, updated_at = NOW() WHERE id = '${id}';"
        
        echo "Updated model ${model_name}: input price ${input_price} -> ${new_input_price}, output price ${output_price} -> ${new_output_price}"
    done
    
    echo "Price update completed."
}

# Main loop
echo "Starting model price updater..."
echo "Press Ctrl+C to stop"

while true; do
    update_model_prices
    sleep 45
done 