#!/bin/bash

# Stargate Build Script
# This script builds the Stargate binaries for different platforms

set -e

# Configuration
PROJECT_NAME="stargate"
VERSION=${VERSION:-"v1.0.0"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION=$(go version | awk '{print $3}')

# Build flags
LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT} -s -w"
BUILD_FLAGS="-trimpath"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Go version
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed"
        exit 1
    fi
    
    log_info "Go version: ${GO_VERSION}"
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod not found. Please run this script from the project root."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Clean previous builds
clean() {
    log_info "Cleaning previous builds..."
    rm -rf dist/
    mkdir -p dist/
    log_success "Clean completed"
}

# Download dependencies
download_deps() {
    log_info "Downloading dependencies..."
    go mod download
    go mod tidy
    log_success "Dependencies downloaded"
}

# Run tests
run_tests() {
    log_info "Running tests..."
    go test -v -race -coverprofile=coverage.out ./...
    if [[ $? -eq 0 ]]; then
        log_success "All tests passed"
    else
        log_error "Tests failed"
        exit 1
    fi
}

# Build for single platform
build_binary() {
    local component=$1
    local goos=$2
    local goarch=$3
    
    local output_name="${component}"
    if [[ "${goos}" == "windows" ]]; then
        output_name="${component}.exe"
    fi
    
    local output_path="dist/${goos}-${goarch}/${output_name}"
    
    log_info "Building ${component} for ${goos}/${goarch}..."
    
    GOOS=${goos} GOARCH=${goarch} go build \
        ${BUILD_FLAGS} \
        -ldflags "${LDFLAGS}" \
        -o "${output_path}" \
        "./cmd/${component}"
    
    if [[ $? -eq 0 ]]; then
        log_success "Built ${component} for ${goos}/${goarch}: ${output_path}"
    else
        log_error "Failed to build ${component} for ${goos}/${goarch}"
        exit 1
    fi
}

# Build all components for all platforms
build_all() {
    local platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
    )
    
    local components=(
        "stargate-node"
        "stargate-controller"
    )
    
    log_info "Building all components for all platforms..."
    
    for platform in "${platforms[@]}"; do
        local goos=$(echo $platform | cut -d'/' -f1)
        local goarch=$(echo $platform | cut -d'/' -f2)
        
        mkdir -p "dist/${goos}-${goarch}"
        
        for component in "${components[@]}"; do
            build_binary "${component}" "${goos}" "${goarch}"
        done
    done
    
    log_success "All builds completed"
}

# Build for current platform only
build_local() {
    local goos=$(go env GOOS)
    local goarch=$(go env GOARCH)
    
    log_info "Building for current platform (${goos}/${goarch})..."
    
    mkdir -p "dist/${goos}-${goarch}"
    
    build_binary "stargate-node" "${goos}" "${goarch}"
    build_binary "stargate-controller" "${goos}" "${goarch}"
    
    log_success "Local build completed"
}

# Create release archives
create_archives() {
    log_info "Creating release archives..."
    
    cd dist/
    
    for dir in */; do
        if [[ -d "$dir" ]]; then
            platform=$(basename "$dir")
            archive_name="${PROJECT_NAME}-${VERSION}-${platform}"
            
            if [[ "$platform" == *"windows"* ]]; then
                zip -r "${archive_name}.zip" "$dir"
                log_success "Created ${archive_name}.zip"
            else
                tar -czf "${archive_name}.tar.gz" "$dir"
                log_success "Created ${archive_name}.tar.gz"
            fi
        fi
    done
    
    cd ..
    log_success "Archives created"
}

# Generate checksums
generate_checksums() {
    log_info "Generating checksums..."
    
    cd dist/
    
    # Generate SHA256 checksums
    if command -v sha256sum &> /dev/null; then
        sha256sum *.tar.gz *.zip 2>/dev/null > checksums.txt || true
    elif command -v shasum &> /dev/null; then
        shasum -a 256 *.tar.gz *.zip 2>/dev/null > checksums.txt || true
    else
        log_warning "No checksum utility found, skipping checksum generation"
        cd ..
        return
    fi
    
    cd ..
    log_success "Checksums generated"
}

# Show build info
show_build_info() {
    log_info "Build Information:"
    echo "  Project: ${PROJECT_NAME}"
    echo "  Version: ${VERSION}"
    echo "  Build Time: ${BUILD_TIME}"
    echo "  Git Commit: ${GIT_COMMIT}"
    echo "  Go Version: ${GO_VERSION}"
}

# Main function
main() {
    local command=${1:-"local"}
    
    show_build_info
    check_prerequisites
    
    case $command in
        "all")
            clean
            download_deps
            run_tests
            build_all
            create_archives
            generate_checksums
            ;;
        "local")
            clean
            download_deps
            build_local
            ;;
        "test")
            download_deps
            run_tests
            ;;
        "clean")
            clean
            ;;
        *)
            echo "Usage: $0 [all|local|test|clean]"
            echo "  all   - Build for all platforms and create release archives"
            echo "  local - Build for current platform only (default)"
            echo "  test  - Run tests only"
            echo "  clean - Clean build artifacts"
            exit 1
            ;;
    esac
    
    log_success "Build script completed successfully!"
}

# Run main function with all arguments
main "$@"
