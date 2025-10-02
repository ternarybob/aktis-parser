#!/bin/bash

# -----------------------------------------------------------------------
# Build Script for Aktis Parser (macOS)
# -----------------------------------------------------------------------

set -e  # Exit on any error

# Default values
ENVIRONMENT="dev"
VERSION=""
CLEAN=false
TEST=false
VERBOSE=false
RELEASE=false
RUN=false

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Help function
show_help() {
    cat << EOF
Build Script for Aktis Parser

SYNOPSIS:
    ./scripts/build.sh [OPTIONS]

DESCRIPTION:
    This script builds the Aktis Parser service for local development and testing.
    Outputs the executable to the project's bin directory.

OPTIONS:
    -e, --environment ENV    Target environment for build (dev, staging, prod)
    -v, --version VERSION    Version to embed in the binary (defaults to .version file or git commit hash)
    -c, --clean             Clean build artifacts before building
    -t, --test              Run tests before building
    --verbose               Enable verbose output
    -r, --release           Build optimized release binary
    --run                   Run the application in a new terminal after successful build
    -h, --help              Show this help message

EXAMPLES:
    ./scripts/build.sh
        Build aktis parser for development

    ./scripts/build.sh --release
        Build optimized release version

    ./scripts/build.sh --environment prod --version "1.0.0"
        Build for production with specific version

    ./scripts/build.sh --run
        Build and run the application in a new terminal
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -t|--test)
            TEST=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        -r|--release)
            RELEASE=true
            shift
            ;;
        --run)
            RUN=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Get git commit
GIT_COMMIT=""
if git rev-parse --short HEAD >/dev/null 2>&1; then
    GIT_COMMIT=$(git rev-parse --short HEAD)
else
    GIT_COMMIT="unknown"
fi

print_color $CYAN "Aktis Parser Build Script"
print_color $CYAN "========================="

# Setup paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
VERSION_FILE_PATH="$PROJECT_ROOT/.version"
BIN_DIR="$PROJECT_ROOT/bin"
OUTPUT_PATH="$BIN_DIR/aktis-parser"

print_color $GRAY "Project Root: $PROJECT_ROOT"
print_color $GRAY "Environment: $ENVIRONMENT"
print_color $GRAY "Git Commit: $GIT_COMMIT"

# Handle version file creation and maintenance
BUILD_TIMESTAMP=$(date +"%m-%d-%H-%M-%S")

if [[ ! -f "$VERSION_FILE_PATH" ]]; then
    # Create .version file if it doesn't exist
    cat > "$VERSION_FILE_PATH" << EOF
server_version: 0.1.0
server_build: $BUILD_TIMESTAMP
extension_version: 0.1.0
EOF
    print_color $GREEN "Created .version file with version 0.1.0"
else
# Read current version and increment patch versions
    # Use standard variables instead of associative arrays
    server_version=""
    extension_version=""
    while IFS= read -r line; do
        if [[ $line =~ ^server_version:[[:space:]]*(.+)$ ]]; then
            current_server_version="${BASH_REMATCH[1]// /}"
            # Parse version (format: major.minor.patch)
            if [[ $current_server_version =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
                major="${BASH_REMATCH[1]}"
                minor="${BASH_REMATCH[2]}"
                patch="${BASH_REMATCH[3]}"
                
                # Increment patch version
                ((patch++))
                server_version="$major.$minor.$patch"
                
                print_color $GREEN "Incremented server version: $current_server_version -> $server_version"
            else
                server_version="$current_server_version"
                print_color $YELLOW "Server version format not recognized, keeping: $current_server_version"
            fi
        elif [[ $line =~ ^extension_version:[[:space:]]*(.+)$ ]]; then
            current_ext_version="${BASH_REMATCH[1]// /}"
            # Parse version (format: major.minor.patch)
            if [[ $current_ext_version =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
                major="${BASH_REMATCH[1]}"
                minor="${BASH_REMATCH[2]}"
                patch="${BASH_REMATCH[3]}"
                
                # Increment patch version
                ((patch++))
                extension_version="$major.$minor.$patch"
                
                print_color $GREEN "Incremented extension version: $current_ext_version -> $extension_version"
            else
                extension_version="$current_ext_version"
                print_color $YELLOW "Extension version format not recognized, keeping: $current_ext_version"
            fi
        fi
    done < "$VERSION_FILE_PATH"

    # Write updated version file
    cat > "$VERSION_FILE_PATH" << EOF
server_version: $server_version
server_build: $BUILD_TIMESTAMP
extension_version: $extension_version
EOF
    print_color $GREEN "Updated build timestamp to: $BUILD_TIMESTAMP"
fi

# Read version information from .version file
# Re-read the final values from file
final_server_version=""
final_server_build=""
final_extension_version=""
while IFS= read -r line; do
    if [[ $line =~ ^server_version:[[:space:]]*(.+)$ ]]; then
        final_server_version="${BASH_REMATCH[1]// /}"
    elif [[ $line =~ ^server_build:[[:space:]]*(.+)$ ]]; then
        final_server_build="${BASH_REMATCH[1]// /}"
    elif [[ $line =~ ^extension_version:[[:space:]]*(.+)$ ]]; then
        final_extension_version="${BASH_REMATCH[1]// /}"
    fi
done < "$VERSION_FILE_PATH"

print_color $CYAN "Using server version: $final_server_version, build: $final_server_build"
print_color $CYAN "Using extension version: $final_extension_version"

# Clean build artifacts if requested
if [[ "$CLEAN" == true ]]; then
    print_color $YELLOW "Cleaning build artifacts..."
    if [[ -d "$BIN_DIR" ]]; then
        rm -rf "$BIN_DIR"
    fi
    if [[ -f "go.sum" ]]; then
        rm -f "go.sum"
    fi
fi

# Create bin directory
mkdir -p "$BIN_DIR"

# Run tests if requested
if [[ "$TEST" == true ]]; then
    print_color $YELLOW "Running tests..."
    if ! go test ./... -v; then
        print_color $RED "Tests failed!"
        exit 1
    fi
    print_color $GREEN "Tests passed!"
fi

# Stop executing process if it's running
PROCESS_NAME="aktis-parser"
if pgrep -x "$PROCESS_NAME" > /dev/null; then
    print_color $YELLOW "Stopping existing Aktis Parser process..."
    pkill -x "$PROCESS_NAME" || true
    sleep 1  # Give process time to fully terminate
    print_color $GREEN "Process stopped successfully"
else
    print_color $GRAY "No Aktis Parser process found running"
fi

# Tidy and download dependencies
print_color $YELLOW "Tidying dependencies..."
if ! go mod tidy; then
    print_color $RED "Failed to tidy dependencies!"
    exit 1
fi

print_color $YELLOW "Downloading dependencies..."
if ! go mod download; then
    print_color $RED "Failed to download dependencies!"
    exit 1
fi

# Build flags
MODULE="github.com/bobmc/aktis-parser/internal/common"
BUILD_FLAGS=(
    "-X" "$MODULE.Version=$final_server_version"
    "-X" "$MODULE.Build=$final_server_build"
    "-X" "$MODULE.GitCommit=$GIT_COMMIT"
)

if [[ "$RELEASE" == true ]]; then
    BUILD_FLAGS+=("-w" "-s")  # Strip debug info and symbol table
fi

LDFLAGS="${BUILD_FLAGS[@]}"

# Build command
print_color $YELLOW "Building aktis-parser..."

export CGO_ENABLED=0
if [[ "$RELEASE" == true ]]; then
    export GOOS=darwin
    export GOARCH=arm64  # For Apple Silicon Macs, use amd64 for Intel Macs
fi

BUILD_ARGS=(
    "build"
    "-ldflags=$LDFLAGS"
    "-o" "$OUTPUT_PATH"
    "./cmd/aktis-parser/main.go"
)

# Change to project root for build
cd "$PROJECT_ROOT"

if [[ "$VERBOSE" == true ]]; then
    BUILD_ARGS+=("-v")
fi

print_color $GRAY "Build command: go ${BUILD_ARGS[*]}"

if ! go "${BUILD_ARGS[@]}"; then
    print_color $RED "Build failed!"
    exit 1
fi

# Copy configuration file to bin directory
CONFIG_SOURCE_PATH="$PROJECT_ROOT/deployments/aktis-parser.toml"
CONFIG_DEST_PATH="$BIN_DIR/aktis-parser.toml"

if [[ -f "$CONFIG_SOURCE_PATH" ]]; then
    if [[ ! -f "$CONFIG_DEST_PATH" ]]; then
        cp "$CONFIG_SOURCE_PATH" "$CONFIG_DEST_PATH"
        print_color $GREEN "Copied configuration: deployments/aktis-parser.toml -> bin/"
    else
        print_color $CYAN "Using existing bin/aktis-parser.toml (preserving customizations)"
    fi
fi

# Copy pages directory to bin directory
PAGES_SOURCE_PATH="$PROJECT_ROOT/pages"
PAGES_DEST_PATH="$BIN_DIR/pages"

if [[ -d "$PAGES_SOURCE_PATH" ]]; then
    if [[ -d "$PAGES_DEST_PATH" ]]; then
        rm -rf "$PAGES_DEST_PATH"
    fi
    cp -r "$PAGES_SOURCE_PATH" "$PAGES_DEST_PATH"
    print_color $GREEN "Copied pages: pages/ -> bin/pages/"
fi

# Build and deploy Chrome Extension
print_color $YELLOW "\nBuilding Chrome Extension..."

EXTENSION_SOURCE_PATH="$PROJECT_ROOT/cmd/aktis-chrome-extension"
EXTENSION_DEST_PATH="$BIN_DIR/aktis-chrome-extension"

# Check if extension source exists
if [[ -d "$EXTENSION_SOURCE_PATH" ]]; then
    # Update manifest.json with extension version
    MANIFEST_PATH="$EXTENSION_SOURCE_PATH/manifest.json"
    if [[ -f "$MANIFEST_PATH" ]]; then
        # Use jq if available, otherwise use sed
        if command -v jq >/dev/null 2>&1; then
            jq --arg version "$final_extension_version" '.version = $version' "$MANIFEST_PATH" > "$MANIFEST_PATH.tmp" && mv "$MANIFEST_PATH.tmp" "$MANIFEST_PATH"
        else
            # Fallback to sed for simple version update
            sed -i '' "s/\"version\": \"[^\"]*\"/\"version\": \"$final_extension_version\"/g" "$MANIFEST_PATH"
        fi
        print_color $GREEN "Updated manifest.json to version $final_extension_version"
    fi

    # Create extension directory in bin
    if [[ -d "$EXTENSION_DEST_PATH" ]]; then
        rm -rf "$EXTENSION_DEST_PATH"
    fi
    mkdir -p "$EXTENSION_DEST_PATH"

    # Copy extension files (exclude create-icons.ps1 as it's a dev tool)
    EXTENSION_FILES=(
        "manifest.json"
        "background.js"
        "content.js"
        "popup.html"
        "popup.js"
        "sidepanel.html"
        "sidepanel.js"
        "README.md"
    )

    for file in "${EXTENSION_FILES[@]}"; do
        SOURCE_PATH="$EXTENSION_SOURCE_PATH/$file"
        if [[ -f "$SOURCE_PATH" ]]; then
            cp "$SOURCE_PATH" "$EXTENSION_DEST_PATH/"
        else
            print_color $YELLOW "Extension file not found: $file"
        fi
    done

    # Copy icons directory
    ICONS_SOURCE_PATH="$EXTENSION_SOURCE_PATH/icons"
    ICONS_DEST_PATH="$EXTENSION_DEST_PATH/icons"

    if [[ -d "$ICONS_SOURCE_PATH" ]]; then
        cp -r "$ICONS_SOURCE_PATH" "$ICONS_DEST_PATH"
        print_color $GREEN "Copied extension icons: icons/ -> bin/aktis-chrome-extension/icons/"
    else
        # Icons don't exist, check if there are icons in the project root
        ROOT_ICONS_PATH="$PROJECT_ROOT/icons"
        if [[ -d "$ROOT_ICONS_PATH" ]]; then
            cp -r "$ROOT_ICONS_PATH" "$ICONS_DEST_PATH"
            print_color $GREEN "Copied extension icons from project root"
        else
            print_color $YELLOW "Icons not found, extension may not have icons"
        fi
    fi

    print_color $GREEN "Deployed Chrome Extension: bin/aktis-chrome-extension/"
else
    print_color $YELLOW "Chrome extension source not found at: $EXTENSION_SOURCE_PATH"
fi

# Verify executable was created
if [[ ! -f "$OUTPUT_PATH" ]]; then
    print_color $RED "Build completed but executable not found: $OUTPUT_PATH"
    exit 1
fi

# Get file info for binary
FILE_SIZE_MB=$(du -m "$OUTPUT_PATH" | cut -f1)

print_color $CYAN "\n==== Build Summary ===="
print_color $GREEN "Status: SUCCESS"
print_color $GREEN "Environment: $ENVIRONMENT"
print_color $GREEN "Server Version: $final_server_version"
print_color $GREEN "Server Build: $final_server_build"
print_color $GREEN "Extension Version: $final_extension_version"
print_color $GREEN "Server Output: $OUTPUT_PATH (${FILE_SIZE_MB} MB)"
print_color $GREEN "Extension Output: $EXTENSION_DEST_PATH"
print_color $GREEN "Build Time: $(date '+%Y-%m-%d %H:%M:%S')"

if [[ "$TEST" == true ]]; then
    print_color $GREEN "Tests: EXECUTED"
fi

if [[ "$CLEAN" == true ]]; then
    print_color $GREEN "Clean: EXECUTED"
fi

print_color $GREEN "\nBuild completed successfully!"
print_color $CYAN "Server: $OUTPUT_PATH"
print_color $CYAN "Extension: $EXTENSION_DEST_PATH"

# Run application if --run flag is set
if [[ "$RUN" == true ]]; then
    print_color $YELLOW "\n==== Starting Application ===="
    
    # Start in a new terminal tab (macOS Terminal app)
    osascript -e "tell application \"Terminal\" to do script \"cd '$PROJECT_ROOT' && '$OUTPUT_PATH'\""
    
    print_color $GREEN "Application started in new terminal tab"
    print_color $CYAN "Working Directory: $PROJECT_ROOT"
    print_color $CYAN "Executable: $OUTPUT_PATH"
else
    print_color $YELLOW "\nTo run:"
    print_color $WHITE "./bin/aktis-parser"
    print_color $YELLOW "\nTo load extension in Chrome:"
    print_color $WHITE "1. Open chrome://extensions/"
    print_color $WHITE "2. Enable 'Developer mode'"
    print_color $WHITE "3. Click 'Load unpacked' and select: $EXTENSION_DEST_PATH"
fi