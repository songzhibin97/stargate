#!/bin/bash

# Stargate Verification Script
# This script verifies the Stargate installation and configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
STARGATE_NODE_PORT=${STARGATE_NODE_PORT:-8080}
STARGATE_CONTROLLER_PORT=${STARGATE_CONTROLLER_PORT:-9090}
ETCD_PORT=${ETCD_PORT:-2379}
TIMEOUT=${TIMEOUT:-30}

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check if a port is open
check_port() {
    local host=$1
    local port=$2
    local timeout=${3:-5}
    
    if command_exists nc; then
        nc -z -w${timeout} ${host} ${port} 2>/dev/null
    elif command_exists telnet; then
        timeout ${timeout} telnet ${host} ${port} 2>/dev/null | grep -q "Connected"
    else
        # Fallback using /dev/tcp (bash built-in)
        timeout ${timeout} bash -c "exec 3<>/dev/tcp/${host}/${port}" 2>/dev/null
    fi
}

# Make HTTP request
http_request() {
    local url=$1
    local method=${2:-GET}
    local timeout=${3:-10}
    
    if command_exists curl; then
        curl -s -m ${timeout} -X ${method} "${url}"
    elif command_exists wget; then
        wget -q -T ${timeout} -O - "${url}"
    else
        log_error "Neither curl nor wget is available"
        return 1
    fi
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing_tools=()
    
    # Check for required tools
    if ! command_exists go; then
        missing_tools+=("go")
    fi
    
    if ! command_exists curl && ! command_exists wget; then
        missing_tools+=("curl or wget")
    fi
    
    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        return 1
    fi
    
    log_success "Prerequisites check passed"
}

# Verify Go environment
verify_go_environment() {
    log_info "Verifying Go environment..."
    
    local go_version=$(go version)
    log_info "Go version: ${go_version}"
    
    # Check Go modules
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod not found. Please run this script from the project root."
        return 1
    fi
    
    # Verify dependencies
    log_info "Verifying Go dependencies..."
    go mod verify
    if [[ $? -eq 0 ]]; then
        log_success "Go dependencies verified"
    else
        log_error "Go dependencies verification failed"
        return 1
    fi
    
    log_success "Go environment verified"
}

# Verify project structure
verify_project_structure() {
    log_info "Verifying project structure..."
    
    local required_dirs=(
        "api"
        "build"
        "cmd"
        "configs"
        "docs"
        "internal"
        "pkg"
        "scripts"
    )
    
    local missing_dirs=()
    
    for dir in "${required_dirs[@]}"; do
        if [[ ! -d "$dir" ]]; then
            missing_dirs+=("$dir")
        fi
    done
    
    if [[ ${#missing_dirs[@]} -gt 0 ]]; then
        log_error "Missing required directories: ${missing_dirs[*]}"
        return 1
    fi
    
    # Check for required files
    local required_files=(
        "cmd/stargate-node/main.go"
        "cmd/stargate-controller/main.go"
        "configs/stargate-node.defaults.yaml"
        "configs/stargate-controller.defaults.yaml"
        "api/stargate/v1/admin.proto"
        "pkg/plugin/interface.go"
    )
    
    local missing_files=()
    
    for file in "${required_files[@]}"; do
        if [[ ! -f "$file" ]]; then
            missing_files+=("$file")
        fi
    done
    
    if [[ ${#missing_files[@]} -gt 0 ]]; then
        log_error "Missing required files: ${missing_files[*]}"
        return 1
    fi
    
    log_success "Project structure verified"
}

# Verify build
verify_build() {
    log_info "Verifying build..."
    
    # Build the project
    log_info "Building stargate-node..."
    go build -o /tmp/stargate-node ./cmd/stargate-node
    if [[ $? -ne 0 ]]; then
        log_error "Failed to build stargate-node"
        return 1
    fi
    
    log_info "Building stargate-controller..."
    go build -o /tmp/stargate-controller ./cmd/stargate-controller
    if [[ $? -ne 0 ]]; then
        log_error "Failed to build stargate-controller"
        return 1
    fi
    
    # Test version output
    log_info "Testing version output..."
    /tmp/stargate-node -version >/dev/null 2>&1
    /tmp/stargate-controller -version >/dev/null 2>&1
    
    # Cleanup
    rm -f /tmp/stargate-node /tmp/stargate-controller
    
    log_success "Build verification passed"
}

# Verify configuration files
verify_configuration() {
    log_info "Verifying configuration files..."
    
    local config_files=(
        "configs/stargate-node.defaults.yaml"
        "configs/stargate-controller.defaults.yaml"
    )
    
    for config_file in "${config_files[@]}"; do
        if [[ ! -f "$config_file" ]]; then
            log_error "Configuration file not found: $config_file"
            return 1
        fi
        
        # Basic YAML syntax check
        if command_exists python3; then
            python3 -c "import yaml; yaml.safe_load(open('$config_file'))" 2>/dev/null
            if [[ $? -ne 0 ]]; then
                log_error "Invalid YAML syntax in $config_file"
                return 1
            fi
        elif command_exists yq; then
            yq eval '.' "$config_file" >/dev/null 2>&1
            if [[ $? -ne 0 ]]; then
                log_error "Invalid YAML syntax in $config_file"
                return 1
            fi
        else
            log_warning "No YAML validator found, skipping syntax check for $config_file"
        fi
        
        log_success "Configuration file verified: $config_file"
    done
    
    log_success "Configuration verification passed"
}

# Check running services
check_services() {
    log_info "Checking running services..."
    
    # Check etcd
    log_info "Checking etcd connectivity..."
    if check_port localhost ${ETCD_PORT}; then
        log_success "etcd is running on port ${ETCD_PORT}"
    else
        log_warning "etcd is not running on port ${ETCD_PORT}"
    fi
    
    # Check stargate-controller
    log_info "Checking stargate-controller..."
    if check_port localhost ${STARGATE_CONTROLLER_PORT}; then
        log_success "stargate-controller is running on port ${STARGATE_CONTROLLER_PORT}"
        
        # Test health endpoint
        local health_response=$(http_request "http://localhost:${STARGATE_CONTROLLER_PORT}/health" GET 5 2>/dev/null)
        if [[ $? -eq 0 ]]; then
            log_success "stargate-controller health check passed"
        else
            log_warning "stargate-controller health check failed"
        fi
    else
        log_warning "stargate-controller is not running on port ${STARGATE_CONTROLLER_PORT}"
    fi
    
    # Check stargate-node
    log_info "Checking stargate-node..."
    if check_port localhost ${STARGATE_NODE_PORT}; then
        log_success "stargate-node is running on port ${STARGATE_NODE_PORT}"
        
        # Test health endpoint
        local health_response=$(http_request "http://localhost:${STARGATE_NODE_PORT}/health" GET 5 2>/dev/null)
        if [[ $? -eq 0 ]]; then
            log_success "stargate-node health check passed"
        else
            log_warning "stargate-node health check failed"
        fi
    else
        log_warning "stargate-node is not running on port ${STARGATE_NODE_PORT}"
    fi
}

# Run tests
run_tests() {
    log_info "Running tests..."
    
    # Unit tests
    log_info "Running unit tests..."
    go test -v ./... -short
    if [[ $? -eq 0 ]]; then
        log_success "Unit tests passed"
    else
        log_error "Unit tests failed"
        return 1
    fi
    
    # Integration tests (if available)
    if [[ -d "test/integration" ]]; then
        log_info "Running integration tests..."
        go test -v ./test/integration/...
        if [[ $? -eq 0 ]]; then
            log_success "Integration tests passed"
        else
            log_warning "Integration tests failed"
        fi
    fi
    
    log_success "Tests completed"
}

# Generate verification report
generate_report() {
    log_info "Generating verification report..."
    
    local report_file="verification-report-$(date +%Y%m%d-%H%M%S).txt"
    
    {
        echo "Stargate Verification Report"
        echo "Generated: $(date)"
        echo "=========================="
        echo
        echo "Go Version: $(go version)"
        echo "Project Root: $(pwd)"
        echo
        echo "Directory Structure:"
        find . -type d -name ".*" -prune -o -type d -print | head -20
        echo
        echo "Configuration Files:"
        ls -la configs/
        echo
        echo "Build Artifacts:"
        ls -la dist/ 2>/dev/null || echo "No build artifacts found"
        echo
    } > "$report_file"
    
    log_success "Verification report generated: $report_file"
}

# Main verification function
main() {
    local command=${1:-"all"}
    
    log_info "Starting Stargate verification..."
    
    case $command in
        "all")
            check_prerequisites
            verify_go_environment
            verify_project_structure
            verify_configuration
            verify_build
            run_tests
            check_services
            generate_report
            ;;
        "structure")
            verify_project_structure
            ;;
        "build")
            verify_build
            ;;
        "config")
            verify_configuration
            ;;
        "services")
            check_services
            ;;
        "tests")
            run_tests
            ;;
        *)
            echo "Usage: $0 [all|structure|build|config|services|tests]"
            echo "  all       - Run all verifications (default)"
            echo "  structure - Verify project structure"
            echo "  build     - Verify build process"
            echo "  config    - Verify configuration files"
            echo "  services  - Check running services"
            echo "  tests     - Run tests"
            exit 1
            ;;
    esac
    
    log_success "Stargate verification completed!"
}

# Run main function with all arguments
main "$@"
