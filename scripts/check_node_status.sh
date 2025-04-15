#!/bin/bash

# Function to check if all nodes are bootstrapped and healthy
check_nodes_status() {
    # Run the command and capture its output
    local output
    output=$(./bin/avalanche node status newNodes)
    
    # Check if all nodes are bootstrapped and healthy
    if echo "$output" | grep -q "BOOTSTRAPPED" && echo "$output" | grep -q "OK"; then
        # Count the number of nodes that are both bootstrapped and healthy
        local bootstrapped_count=$(echo "$output" | grep -c "BOOTSTRAPPED")
        local healthy_count=$(echo "$output" | grep -c "OK")
        
        # If the counts match (meaning all nodes are both bootstrapped and healthy)
        if [ "$bootstrapped_count" -eq "$healthy_count" ]; then
            echo "All nodes are bootstrapped and healthy!"
            return 0
        fi
    fi
    return 1
}

echo "Starting node status check..."
echo "Will check every 30 seconds until all nodes are bootstrapped and healthy."

# Loop until all nodes are bootstrapped and healthy
while true; do
    if check_nodes_status; then
        echo "All nodes are ready!"
        exit 0
    fi
    
    echo "Not all nodes are ready yet. Waiting 30 seconds..."
    sleep 30
done 