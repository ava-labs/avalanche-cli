#!/usr/bin/env bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Function to handle errors
handle_error() {
    echo -e "${RED}Error: $1${NC}"
    exit 1
}

if ! aws sts get-caller-identity >/dev/null 2>&1 ; then
    echo -e "${RED}aws credentials not set. exiting...${NC}"
    exit 1
fi

# Variables
blockchainName="newBlockchain2"
clusterName="newNodes2"
keyPChainAddress="P-fuji1377nx80rx3pzneup5qywgdgdsmzntql7trcqlg"
keyCChainAddress="0x43719cDF4B3CCDE97328Db4C3c2A955EFfCbb8Cf"
keyName="newTestKey"

# Function to create the node cluster
createNodesCluster() {
    echo "Creating node cluster..."
    if ! ./bin/avalanche node create "$clusterName" \
        --region us-west-1 \
        --use-static-ip=false \
        --num-validators 2 \
        --num-apis 1 \
        --fuji \
        --latest-avalanchego-version \
        --aws \
        --node-type=default \
        --enable-monitoring=false; then
        handle_error "Failed to create node cluster"
    fi
    echo -e "${GREEN}Node cluster created successfully${NC}"
}

# Function to create blockchain config
createBlockchainConfig() {
    echo "Creating blockchain config..."
    if ! ./bin/avalanche blockchain create "$blockchainName" \
        --evm \
        --proof-of-authority \
        --validator-manager-owner="$keyCChainAddress" \
        --production-defaults \
        --evm-chain-id=123456 \
        --evm-token=TEST; then
        handle_error "Failed to create blockchain config"
    fi
    echo -e "${GREEN}Blockchain config created successfully${NC}"
}

# Function to deploy blockchain
deployBlockchain() {
    local bootstrap_file=$1
    echo "Deploying blockchain..."
    if ! ./bin/avalanche blockchain deploy "$blockchainName" \
        --fuji \
        --key "$keyName" \
        --use-local-machine=false \
        --bootstrap-filepath="$bootstrap_file"; then
        handle_error "Failed to deploy blockchain"
    fi
    echo -e "${GREEN}Blockchain deployed successfully${NC}"
}

# Function to initialize validator manager
initializeValidatorManager() {
    echo "Initializing validator manager..."
    if ! ./bin/avalanche contract initValidatorManager "$blockchainName" \
        --fuji \
        --key "$keyName"; then
        handle_error "Failed to initialize validator manager"
    fi
    echo -e "${GREEN}Validator manager initialized successfully${NC}"
}

# Function to sync nodes to blockchain
syncNodesToBlockchain() {
    echo "Syncing nodes to blockchain..."
    if ! ./bin/avalanche node sync "$clusterName" "$blockchainName"; then
        handle_error "Failed to sync nodes to blockchain"
    fi
    echo -e "${GREEN}Nodes synced to blockchain successfully${NC}"
}

check_nodes_status() {
    # Run the command and capture its output
    local output
    output=$(./bin/avalanche node status "$clusterName")

    # Check if all nodes are bootstrapped and healthy
    if echo "$output" | grep -q "BOOTSTRAPPED" && echo "$output" | grep -q "OK"; then
        # Count the number of nodes that are both bootstrapped and healthy
        local bootstrapped_count=$(echo "$output" | grep -c "BOOTSTRAPPED")
        local healthy_count=$(echo "$output" | grep -c "OK")

        # If the counts match (meaning all nodes are both bootstrapped and healthy)
        if [ "$bootstrapped_count" -eq "$healthy_count" ]; then
            echo -e "${GREEN}All nodes are bootstrapped and healthy!${NC}"
            return 0
        fi
    fi
    return 1
}

# Function to check subnet status
check_subnet_status() {
    local output
    output=$(./bin/avalanche node status "$clusterName" --blockchain "$blockchainName")

    if echo "$output" | grep -q "OK"; then
        local healthy_count=$(echo "$output" | grep -c "OK")
        local total_nodes=$(echo "$output" | grep -c "NodeID-")

        if [ "$healthy_count" -eq "$total_nodes" ]; then
            echo "All nodes are healthy!"
            return 0
        fi
    fi
    return 1
}

# Function to extract validator information (node ID, public key, and POP)
extract_validator_info() {
    local output
    output=$(./bin/avalanche node ssh "$clusterName" \
      "curl -s -X POST -H \"content-type:application/json\" \
      --data '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"info.getNodeID\"}' \
      127.0.0.1:9650/ext/info")

    IFS=$'\n' read -d '' -r -a lines <<< "$output"

    local result_block=""
    local capture=0
    local json_items=()

    for line in "${lines[@]}"; do
        if [[ "$line" == *"[Validator]"* ]]; then
            capture=1
            continue
        fi

        if [[ $capture -eq 1 ]]; then
            result_block+="$line"
            if [[ "$line" == *"}"* ]]; then
                local item
                item=$(echo "$result_block" | jq -c '{
                    nodeID: .result.nodeID,
                    publicKey: .result.nodePOP.publicKey,
                    pop: .result.nodePOP.proofOfPossession
                }')
                json_items+=("$item")
                result_block=""
                capture=0
            fi
        fi
    done

    # Combine all items into a JSON array and print it
    printf '%s\n' "${json_items[@]}" | jq -s .
}

# Function to create bootstrap validators JSON
create_bootstrap_validators_json() {
    echo "Creating bootstrap validators JSON file..."
    local validator_info=$1

    # Transform the validator info into the required format
    local bootstrap_json
    bootstrap_json=$(echo "$validator_info" | jq -c 'map({
        NodeID: .nodeID,
        Weight: 100,
        Balance: 100000000,
        BLSPublicKey: .publicKey,
        BLSProofOfPossession: .pop,
        ChangeOwnerAddr: "'"$keyPChainAddress"'",
        ValidationID: ""
    })')

    # Write to file
    echo "$bootstrap_json" > bootstrap_validators.json
    echo -e "${GREEN}Bootstrap validators JSON file created successfully${NC}"
}

# Function to destroy node cluster
destroyNodesCluster() {
    echo "Destroying node cluster..."
    if ! ./bin/avalanche node destroy "$clusterName" --authorize-all; then
        echo -e "${RED}Warning: Failed to destroy node cluster${NC}"
        return 1
    fi
    echo -e "${GREEN}Node cluster destroyed successfully${NC}"
    return 0
}

# Function to clean up temporary files
cleanup() {
    echo "Cleaning up resources..."

    # Destroy node cluster
    destroyNodesCluster

    # Clean up temporary files
    echo "Cleaning up temporary files..."
    if [ -f "bootstrap_validators.json" ]; then
        rm "bootstrap_validators.json"
        echo -e "${GREEN}Removed bootstrap_validators.json${NC}"
    fi
    echo -e "${GREEN}Cleanup completed${NC}"
}

# Create the node cluster first
createNodesCluster

echo "Starting node status check..."
echo "Will check every 30 seconds until all nodes are bootstrapped and healthy."

# Loop until all nodes are bootstrapped and healthy with 15 minute timeout
start_time=$(date +%s)
timeout_seconds=900  # 15 minutes

while true; do
    current_time=$(date +%s)
    elapsed_time=$((current_time - start_time))

    if [ "$elapsed_time" -ge "$timeout_seconds" ]; then
        handle_error "Timeout: Nodes did not become healthy within 15 minutes"
    fi

    if check_nodes_status; then
        break
    fi

    echo "Not all nodes are ready yet. Waiting 30 seconds... (Elapsed time: $((elapsed_time / 60)) minutes)"
    sleep 30
done

echo "Extracting validator information..."
validator_info=$(extract_validator_info)
echo "Validator information:"
echo "$validator_info" | jq .

# Create bootstrap validators JSON file
create_bootstrap_validators_json "$validator_info"

# Create blockchain config
createBlockchainConfig

# Deploy blockchain with bootstrap file
deployBlockchain "bootstrap_validators.json"

# Sync nodes to blockchain
syncNodesToBlockchain

echo "Starting subnet status check..."
echo "Will check every 10 seconds until all nodes are healthy (5 minute timeout)."

# Loop until all nodes are healthy with 5 minute timeout
start_time=$(date +%s)
timeout_seconds=300  # 5 minutes

while true; do
    current_time=$(date +%s)
    elapsed_time=$((current_time - start_time))

    if [ "$elapsed_time" -ge "$timeout_seconds" ]; then
        handle_error "Timeout: Nodes did not become healthy within 5 minutes"
    fi

    if check_subnet_status; then
        echo -e "${GREEN}All nodes are healthy!${NC}"
        break
    fi

    echo "Not all nodes are healthy yet. Waiting 10 seconds... (Elapsed time: $((elapsed_time / 60)) minutes)"
    sleep 10
done

# Initialize validator manager
initializeValidatorManager

# Clean up temporary files
#cleanup

echo -e "${GREEN}All operations completed successfully${NC}"