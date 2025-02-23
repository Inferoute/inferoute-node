#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to handle errors
handle_error() {
    echo -e "${RED}Error: $1${NC}"
    exit 1
}



# Drop and recreate database
echo -e "${GREEN}Dropping and recreating database...${NC}"
cockroach sql --insecure --host=localhost --execute="DROP DATABASE IF EXISTS inferoute CASCADE; CREATE DATABASE inferoute;" || handle_error "Failed to reset database"

# Apply schema
echo -e "${GREEN}Applying schema...${NC}"
cockroach sql --insecure --host=localhost --database=inferoute < schema.sql || handle_error "Failed to apply schema"

# Apply seed data
echo -e "${GREEN}Applying seed data...${NC}"
cockroach sql --insecure --host=localhost --database=inferoute < seed.sql || handle_error "Failed to apply seed data"

echo -e "${GREEN}Database reset and seeded successfully!${NC}"

# Print some verification queries
echo -e "${GREEN}Verifying data...${NC}"
echo "1. Checking user counts:"
cockroach sql --insecure --host=localhost --database=inferoute --execute="SELECT COUNT(*) FROM users;"

echo -e "\n2. Checking provider models:"
cockroach sql --insecure --host=localhost --database=inferoute --execute="SELECT u.username, COUNT(pm.*) as model_count FROM users u LEFT JOIN provider_models pm ON u.id = pm.provider_id GROUP BY u.username;"

echo -e "\n3. Checking health history:"
cockroach sql --insecure --host=localhost --database=inferoute --execute="SELECT u.username, COUNT(phh.*) as health_checks FROM users u LEFT JOIN provider_health_history phh ON u.id = phh.provider_id GROUP BY u.username;" 