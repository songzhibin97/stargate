#!/bin/bash

# Stargate API Documentation Validation Script
# This script validates that all API endpoints documented are working correctly

set -e

# Configuration
BASE_URL="${STARGATE_BASE_URL:-http://localhost:9090}"
PORTAL_URL="${STARGATE_PORTAL_URL:-http://localhost:8080}"
API_KEY="${STARGATE_API_KEY:-}"
VERBOSE="${VERBOSE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED_TESTS++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED_TESTS++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Test function
test_endpoint() {
    local method=$1
    local endpoint=$2
    local expected_status=$3
    local description=$4
    local headers=$5
    local body=$6
    
    ((TOTAL_TESTS++))
    
    log_info "Testing: $description"
    
    if [ "$VERBOSE" = "true" ]; then
        echo "  Method: $method"
        echo "  Endpoint: $endpoint"
        echo "  Expected Status: $expected_status"
    fi
    
    # Build curl command
    local curl_cmd="curl -s -w '%{http_code}' -o /tmp/response.json"
    
    if [ -n "$headers" ]; then
        curl_cmd="$curl_cmd $headers"
    fi
    
    if [ -n "$body" ]; then
        curl_cmd="$curl_cmd -d '$body'"
    fi
    
    curl_cmd="$curl_cmd -X $method $endpoint"
    
    # Execute request
    local status_code
    status_code=$(eval $curl_cmd)
    
    if [ "$status_code" = "$expected_status" ]; then
        log_success "$description (Status: $status_code)"
        if [ "$VERBOSE" = "true" ] && [ -f "/tmp/response.json" ]; then
            echo "  Response:"
            cat /tmp/response.json | jq . 2>/dev/null || cat /tmp/response.json
            echo
        fi
    else
        log_error "$description (Expected: $expected_status, Got: $status_code)"
        if [ -f "/tmp/response.json" ]; then
            echo "  Response:"
            cat /tmp/response.json
            echo
        fi
    fi
    
    # Clean up
    rm -f /tmp/response.json
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if curl is available
    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    # Check if jq is available (optional)
    if ! command -v jq &> /dev/null; then
        log_warning "jq is not installed - JSON responses will not be formatted"
    fi
    
    # Check if Stargate is running
    if ! curl -s "$BASE_URL/health" > /dev/null; then
        log_error "Stargate server is not running at $BASE_URL"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Generate API key if not provided
generate_api_key() {
    if [ -z "$API_KEY" ]; then
        log_info "No API key provided, attempting to generate one..."
        
        local response
        response=$(curl -s -X POST "$BASE_URL/auth/api-keys" \
            -H "Content-Type: application/json" \
            -d '{"name": "validation-script-key"}' 2>/dev/null || echo "")
        
        if [ -n "$response" ]; then
            API_KEY=$(echo "$response" | jq -r '.api_key' 2>/dev/null || echo "")
            if [ -n "$API_KEY" ] && [ "$API_KEY" != "null" ]; then
                log_success "Generated API key: ${API_KEY:0:10}..."
            else
                log_warning "Could not generate API key automatically"
                log_info "Please set STARGATE_API_KEY environment variable"
            fi
        else
            log_warning "Could not generate API key - server may require authentication"
        fi
    fi
}

# Test health endpoints
test_health_endpoints() {
    log_info "Testing health endpoints..."
    
    test_endpoint "GET" "$BASE_URL/health" "200" "Basic health check" "" ""
    test_endpoint "GET" "$BASE_URL/health?detailed=true" "200" "Detailed health check" "" ""
    test_endpoint "GET" "$BASE_URL/metrics" "200" "Metrics endpoint" "" ""
}

# Test authentication endpoints
test_auth_endpoints() {
    log_info "Testing authentication endpoints..."
    
    # Test login endpoint (may fail if no default credentials)
    test_endpoint "POST" "$BASE_URL/auth/login" "200|401" "Login endpoint" \
        "-H 'Content-Type: application/json'" \
        '{"username": "admin", "password": "password"}'
    
    # Test API key generation (may fail without proper auth)
    if [ -n "$API_KEY" ]; then
        test_endpoint "POST" "$BASE_URL/auth/api-keys" "200|401" "API key generation" \
            "-H 'Content-Type: application/json' -H 'X-Admin-Key: $API_KEY'" \
            '{"name": "test-key"}'
    fi
}

# Test route endpoints
test_route_endpoints() {
    if [ -z "$API_KEY" ]; then
        log_warning "Skipping route tests - no API key available"
        return
    fi
    
    log_info "Testing route endpoints..."
    
    test_endpoint "GET" "$BASE_URL/api/v1/routes" "200" "List routes" \
        "-H 'X-Admin-Key: $API_KEY'" ""
    
    test_endpoint "GET" "$BASE_URL/api/v1/routes?limit=5" "200" "List routes with pagination" \
        "-H 'X-Admin-Key: $API_KEY'" ""
}

# Test upstream endpoints
test_upstream_endpoints() {
    if [ -z "$API_KEY" ]; then
        log_warning "Skipping upstream tests - no API key available"
        return
    fi
    
    log_info "Testing upstream endpoints..."
    
    test_endpoint "GET" "$BASE_URL/api/v1/upstreams" "200" "List upstreams" \
        "-H 'X-Admin-Key: $API_KEY'" ""
}

# Test plugin endpoints
test_plugin_endpoints() {
    if [ -z "$API_KEY" ]; then
        log_warning "Skipping plugin tests - no API key available"
        return
    fi
    
    log_info "Testing plugin endpoints..."
    
    test_endpoint "GET" "$BASE_URL/api/v1/plugins" "200" "List plugins" \
        "-H 'X-Admin-Key: $API_KEY'" ""
}

# Test configuration endpoints
test_config_endpoints() {
    if [ -z "$API_KEY" ]; then
        log_warning "Skipping configuration tests - no API key available"
        return
    fi
    
    log_info "Testing configuration endpoints..."
    
    test_endpoint "GET" "$BASE_URL/api/v1/config" "200" "Get configuration" \
        "-H 'X-Admin-Key: $API_KEY'" ""
    
    test_endpoint "POST" "$BASE_URL/api/v1/config/validate" "200" "Validate configuration" \
        "-H 'X-Admin-Key: $API_KEY' -H 'Content-Type: application/json'" \
        '{"routes": [], "upstreams": [], "plugins": []}'
}

# Test portal endpoints
test_portal_endpoints() {
    log_info "Testing portal endpoints..."
    
    # Test portal health (if portal is running)
    if curl -s "$PORTAL_URL/health" > /dev/null 2>&1; then
        test_endpoint "GET" "$PORTAL_URL/health" "200" "Portal health check" "" ""
        
        # Test portal login
        test_endpoint "POST" "$PORTAL_URL/api/v1/auth/login" "200|401" "Portal login" \
            "-H 'Content-Type: application/json'" \
            '{"username": "developer", "password": "password"}'
    else
        log_warning "Portal server not running at $PORTAL_URL - skipping portal tests"
    fi
}

# Test OpenAPI specification
test_openapi_spec() {
    log_info "Testing OpenAPI specification..."
    
    if [ -f "docs/openapi.yaml" ]; then
        # Check if OpenAPI spec is valid YAML
        if command -v python3 &> /dev/null; then
            if python3 -c "import yaml; yaml.safe_load(open('docs/openapi.yaml'))" 2>/dev/null; then
                log_success "OpenAPI specification is valid YAML"
                ((TOTAL_TESTS++))
                ((PASSED_TESTS++))
            else
                log_error "OpenAPI specification has invalid YAML syntax"
                ((TOTAL_TESTS++))
            fi
        else
            log_warning "Python3 not available - cannot validate OpenAPI YAML syntax"
        fi
    else
        log_error "OpenAPI specification file not found at docs/openapi.yaml"
        ((TOTAL_TESTS++))
    fi
}

# Test documentation files
test_documentation_files() {
    log_info "Testing documentation files..."
    
    local docs=(
        "docs/README.md"
        "docs/api-documentation.md"
        "docs/api-examples.md"
        "docs/api-quickstart.md"
        "docs/api-troubleshooting.md"
        "docs/sdk-documentation.md"
        "docs/openapi.yaml"
        "docs/stargate-api.postman_collection.json"
    )
    
    for doc in "${docs[@]}"; do
        ((TOTAL_TESTS++))
        if [ -f "$doc" ]; then
            log_success "Documentation file exists: $doc"
            ((PASSED_TESTS++))
        else
            log_error "Documentation file missing: $doc"
        fi
    done
}

# Test Postman collection
test_postman_collection() {
    log_info "Testing Postman collection..."
    
    if [ -f "docs/stargate-api.postman_collection.json" ]; then
        # Check if Postman collection is valid JSON
        if command -v jq &> /dev/null; then
            if jq . "docs/stargate-api.postman_collection.json" > /dev/null 2>&1; then
                log_success "Postman collection is valid JSON"
                ((TOTAL_TESTS++))
                ((PASSED_TESTS++))
            else
                log_error "Postman collection has invalid JSON syntax"
                ((TOTAL_TESTS++))
            fi
        else
            log_warning "jq not available - cannot validate Postman collection JSON"
        fi
    else
        log_error "Postman collection file not found"
        ((TOTAL_TESTS++))
    fi
}

# Print summary
print_summary() {
    echo
    echo "=================================="
    echo "API Documentation Validation Summary"
    echo "=================================="
    echo "Total Tests: $TOTAL_TESTS"
    echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed: ${RED}$FAILED_TESTS${NC}"
    echo "Success Rate: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%"
    echo "=================================="
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "${GREEN}All tests passed! API documentation is valid.${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed. Please review the output above.${NC}"
        exit 1
    fi
}

# Main execution
main() {
    echo "Stargate API Documentation Validation"
    echo "======================================"
    echo "Base URL: $BASE_URL"
    echo "Portal URL: $PORTAL_URL"
    echo "Verbose: $VERBOSE"
    echo
    
    check_prerequisites
    generate_api_key
    
    test_health_endpoints
    test_auth_endpoints
    test_route_endpoints
    test_upstream_endpoints
    test_plugin_endpoints
    test_config_endpoints
    test_portal_endpoints
    test_openapi_spec
    test_documentation_files
    test_postman_collection
    
    print_summary
}

# Handle command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -k|--api-key)
            API_KEY="$2"
            shift 2
            ;;
        -u|--base-url)
            BASE_URL="$2"
            shift 2
            ;;
        -p|--portal-url)
            PORTAL_URL="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -v, --verbose       Enable verbose output"
            echo "  -k, --api-key KEY   Use specific API key"
            echo "  -u, --base-url URL  Set base URL (default: http://localhost:9090)"
            echo "  -p, --portal-url URL Set portal URL (default: http://localhost:8080)"
            echo "  -h, --help          Show this help message"
            echo
            echo "Environment variables:"
            echo "  STARGATE_BASE_URL   Base URL for Stargate API"
            echo "  STARGATE_PORTAL_URL Portal URL for Stargate Portal"
            echo "  STARGATE_API_KEY    API key for authentication"
            echo "  VERBOSE             Enable verbose output (true/false)"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use -h or --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main
