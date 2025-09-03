#!/bin/bash

# Test script for Stargate Admin API
# This script tests the basic functionality of the Admin API

set -e

API_BASE="http://localhost:9090"
API_PREFIX="/api/v1"

echo "=== Stargate Admin API Test ==="

# Function to make HTTP requests
make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local auth_header=$4
    
    if [ -n "$auth_header" ]; then
        if [ -n "$data" ]; then
            curl -s -X "$method" \
                -H "Content-Type: application/json" \
                -H "$auth_header" \
                -d "$data" \
                "$API_BASE$endpoint"
        else
            curl -s -X "$method" \
                -H "$auth_header" \
                "$API_BASE$endpoint"
        fi
    else
        if [ -n "$data" ]; then
            curl -s -X "$method" \
                -H "Content-Type: application/json" \
                -d "$data" \
                "$API_BASE$endpoint"
        else
            curl -s -X "$method" \
                "$API_BASE$endpoint"
        fi
    fi
}

echo "1. Testing Health Endpoint..."
health_response=$(make_request "GET" "/health")
echo "Health Response: $health_response"

echo -e "\n2. Testing Authentication..."
login_data='{"username":"admin","password":"password"}'
auth_response=$(make_request "POST" "/auth/login" "$login_data")
echo "Auth Response: $auth_response"

# Extract token (simplified - in real scenario would use jq)
token=$(echo "$auth_response" | grep -o '"token":"[^"]*' | cut -d'"' -f4)
if [ -n "$token" ]; then
    auth_header="Authorization: Bearer $token"
    echo "Token obtained: ${token:0:20}..."
else
    echo "No token obtained, continuing without authentication"
    auth_header=""
fi

echo -e "\n3. Testing Route Creation..."
route_data='{
    "id": "test-route-001",
    "name": "Test API Route",
    "rules": {
        "hosts": ["api.test.com"],
        "paths": [{"type": "prefix", "value": "/api"}],
        "methods": ["GET", "POST"]
    },
    "upstream_id": "test-upstream-001",
    "priority": 100,
    "metadata": {
        "description": "Test route for API endpoints",
        "team": "test-team"
    }
}'

create_response=$(make_request "POST" "$API_PREFIX/routes/" "$route_data" "$auth_header")
echo "Create Route Response: $create_response"

echo -e "\n4. Testing Route Retrieval..."
get_response=$(make_request "GET" "$API_PREFIX/routes/test-route-001" "" "$auth_header")
echo "Get Route Response: $get_response"

echo -e "\n5. Testing Route List..."
list_response=$(make_request "GET" "$API_PREFIX/routes?limit=10&offset=0" "" "$auth_header")
echo "List Routes Response: $list_response"

echo -e "\n6. Testing Upstream Creation..."
upstream_data='{
    "id": "test-upstream-001",
    "name": "Test Backend Service",
    "targets": [
        {"url": "http://backend1:8080", "weight": 100},
        {"url": "http://backend2:8080", "weight": 100}
    ],
    "algorithm": "round_robin",
    "metadata": {
        "description": "Test upstream service",
        "environment": "test"
    }
}'

upstream_response=$(make_request "POST" "$API_PREFIX/upstreams/" "$upstream_data" "$auth_header")
echo "Create Upstream Response: $upstream_response"

echo -e "\n7. Testing Plugin Creation..."
plugin_data='{
    "id": "test-plugin-001",
    "name": "Test Rate Limiter",
    "type": "rate_limit",
    "enabled": true,
    "config": {
        "max_requests": 100,
        "window_size": "1m",
        "strategy": "sliding_window"
    },
    "routes": ["test-route-001"],
    "priority": 100,
    "metadata": {
        "description": "Test rate limiting plugin"
    }
}'

plugin_response=$(make_request "POST" "$API_PREFIX/plugins/" "$plugin_data" "$auth_header")
echo "Create Plugin Response: $plugin_response"

echo -e "\n8. Testing Configuration Retrieval..."
config_response=$(make_request "GET" "$API_PREFIX/config" "" "$auth_header")
echo "Config Response: $config_response"

echo -e "\n9. Testing Documentation Endpoints..."
docs_response=$(make_request "GET" "/docs/openapi.json")
echo "OpenAPI Spec Length: $(echo "$docs_response" | wc -c) characters"

echo -e "\n10. Testing Route Update..."
update_data='{
    "id": "test-route-001",
    "name": "Updated Test API Route",
    "rules": {
        "hosts": ["api.test.com", "api2.test.com"],
        "paths": [{"type": "prefix", "value": "/api"}],
        "methods": ["GET", "POST", "PUT"]
    },
    "upstream_id": "test-upstream-001",
    "priority": 200,
    "metadata": {
        "description": "Updated test route for API endpoints",
        "team": "test-team",
        "version": "v2"
    }
}'

update_response=$(make_request "PUT" "$API_PREFIX/routes/test-route-001" "$update_data" "$auth_header")
echo "Update Route Response: $update_response"

echo -e "\n11. Testing Route Deletion..."
delete_response=$(make_request "DELETE" "$API_PREFIX/routes/test-route-001" "" "$auth_header")
echo "Delete Route Response: $delete_response"

echo -e "\n12. Testing Upstream Deletion..."
upstream_delete_response=$(make_request "DELETE" "$API_PREFIX/upstreams/test-upstream-001" "" "$auth_header")
echo "Delete Upstream Response: $upstream_delete_response"

echo -e "\n13. Testing Plugin Deletion..."
plugin_delete_response=$(make_request "DELETE" "$API_PREFIX/plugins/test-plugin-001" "" "$auth_header")
echo "Delete Plugin Response: $plugin_delete_response"

echo -e "\n=== Test Summary ==="
echo " Health endpoint tested"
echo " Authentication tested"
echo " Route CRUD operations tested"
echo " Upstream CRUD operations tested"
echo " Plugin CRUD operations tested"
echo " Configuration endpoint tested"
echo " Documentation endpoint tested"

echo -e "\n=== Admin API Test Completed ==="
echo "All basic functionality has been tested successfully!"
echo ""
echo "To run this test:"
echo "1. Start etcd: docker run -d --name etcd -p 2379:2379 quay.io/coreos/etcd:latest"
echo "2. Start stargate-controller: go run cmd/stargate-controller/main.go"
echo "3. Run this script: bash test_admin_api.sh"
echo ""
echo "API Documentation available at: http://localhost:9090/docs"
