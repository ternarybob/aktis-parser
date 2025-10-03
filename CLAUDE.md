# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Aktis Parser is a Go service that scrapes and stores Jira projects/issues and Confluence pages locally using BoltDB. It uses a Chrome extension for authentication, extracting cookies and tokens from an active browser session to make authenticated API requests autonomously.

## Build and Development Commands

### Build the Service
```powershell
.\scripts\build.ps1
```

Build options:
- `-Clean`: Clean build artifacts before building
- `-Test`: Run tests before building
- `-Release`: Build optimized release binary (strips debug symbols)
- `-Run`: Build and run the application in a new terminal
- `-Verbose`: Enable verbose output
- `-Environment <env>`: Specify environment (dev, staging, prod)
- `-Version <version>`: Embed specific version in binary

### Run the Service
```bash
./bin/aktis-parser.exe
```

Service runs on `http://localhost:8080` by default (configurable in `aktis-parser.toml`).

### Test
```bash
go test ./... -v
```

### Dependencies
```bash
go mod tidy
go mod download
```

## Architecture

### Authentication Flow
1. **Extension Extraction**: Chrome extension (`cmd/aktis-chrome-extension/`) extracts authentication state from active Atlassian browser session (cookies, tokens, localStorage, cloudId, atl_token)
2. **Auth Transfer**: Extension POSTs auth data to service `/api/auth` endpoint
3. **Autonomous Scraping**: Service uses auth to make authenticated HTTP requests to Jira/Confluence APIs
4. **Auto-Refresh**: Extension refreshes auth every 30 minutes

### Core Components

**`cmd/service.go`** - Main HTTP server and scraping orchestration:
- `Scraper` struct: Manages HTTP client with cookies, stores auth tokens (cloudId, atlToken), and handles BoltDB operations
- `UpdateAuth()`: Receives auth from extension, configures HTTP client with cookies
- `ScrapeAll()`: Orchestrates full scrape of projects, issues, and Confluence pages
- `ScrapeProjects()`: Fetches Jira projects via REST API v3
- `ScrapeProjectIssues()`: Paginates through project issues (JQL-based search)
- `ScrapeConfluence()` / `ScrapeSpacePages()`: Fetches Confluence spaces and pages

**`internal/common/config.go`** - Configuration management:
- TOML-based config with environment variable overrides
- Auto-discovers config files: `{executable_name}.toml`, `config.toml`, `aktis-parser.toml`
- Config hierarchy: defaults → TOML file → environment variables
- Environment overrides: `DATABASE_PATH`, `LOG_LEVEL`, `SERVER_PORT`, `SCRAPER_BASE_URL`

**`internal/common/version.go`** - Version information:
- Build-time ldflags injection: `Version`, `Build`, `GitCommit`
- Exposed via `/api/version` endpoint

**`internal/common/logging.go`** - Logging setup:
- Multi-writer support (stdout + optional file)
- Configurable structured/unstructured format

**`internal/common/banner.go`** - ASCII startup banner display

### Storage (BoltDB)

Database: `scraper.db` (location configurable in TOML)

Buckets:
- `projects`: Jira projects (key: project key, value: JSON)
- `issues`: Jira issues (key: issue key, value: JSON)
- `confluence_pages`: Confluence pages (key: page ID, value: JSON with body.storage)

### Chrome Extension

Located in `cmd/aktis-chrome-extension/`:
- `background.js`: Extracts auth state, sends to service, handles 30-min refresh alarm
- `content.js`: Content script for page interaction
- `popup.html/popup.js`: Extension popup UI
- `manifest.json`: Extension configuration (version auto-updated by build script)

Extension is automatically built and deployed to `bin/aktis-chrome-extension/` during build.

## API Endpoints

- `POST /api/auth` - Receive auth from extension, trigger background scrape
- `GET /api/scrape` - Manually trigger scraping
- `GET /api/version` - Version/build information
- `GET /api/health` - Health check

## Configuration

Configuration file: `deployments/aktis-parser.toml` (copied to `bin/` during build)

Key settings:
- `parser.port`: HTTP server port (default: 8080)
- `scraper.auth_method`: "extension" (browser-based auth)
- `scraper.rate_limit_ms`: Delay between API requests (default: 500ms)
- `scraper.jira.max_results_per_page`: Pagination size for issues (default: 50)
- `scraper.confluence.max_results_per_page`: Pagination size for pages (default: 25)
- `storage.database_path`: BoltDB file location (default: `./scraper.db`)
- `storage.retention_days`: Data retention (0 = keep forever)
- `logging.level`: debug, info, warn, error

## Build System

**Build script**: `scripts/build.ps1`

Version management (`.version` file):
- Auto-increments patch version on each build
- Separate `server_version` and `extension_version`
- Updates `server_build` timestamp
- Injects version into binary via ldflags

Build process:
1. Stop running `aktis-parser` process
2. Increment versions in `.version` file
3. Tidy and download Go dependencies
4. Build binary with version ldflags to `bin/aktis-parser.exe`
5. Copy config to `bin/aktis-parser.toml` (preserves existing customizations)
6. Update extension `manifest.json` version
7. Deploy extension to `bin/aktis-chrome-extension/`

## Development Workflow

1. **Initial Setup**:
   ```powershell
   .\scripts\build.ps1 -Clean
   ```

2. **Incremental Development**:
   ```powershell
   .\scripts\build.ps1 -Run
   ```

3. **Load Extension in Chrome**:
   - Open `chrome://extensions/`
   - Enable "Developer mode"
   - Click "Load unpacked"
   - Select `bin/aktis-chrome-extension/`

4. **Authenticate**:
   - Log into Jira/Confluence in Chrome
   - Click extension icon while on Atlassian page
   - Service automatically begins scraping

## Code Patterns

- **Error Handling**: Return errors up the call stack; log at handler level
- **HTTP Requests**: `makeRequest()` centralized method handles auth headers, cookies, and 401/403 detection
- **Rate Limiting**: `time.Sleep()` between paginated requests (configurable)
- **Pagination**: Loop with `startAt`/`maxResults` (Jira) or `start`/`limit` (Confluence)
- **Goroutines**: Background scraping triggered by goroutine in auth handler
- **BoltDB Transactions**: Use `db.Update()` for writes, batch multiple puts in single transaction

## Module Path

`github.com/bobmc/aktis-parser`

## Go Version

Go 1.23

## Dependencies

- `github.com/pelletier/go-toml/v2` - TOML configuration parsing
- `go.etcd.io/bbolt` - BoltDB embedded database
## Code Quality Enforcement System

This project uses automated code quality enforcement that runs continuously to ensure code structure compliance and prevent duplicate implementations.

**Language-Specific Enforcement:**
- **TypeScript/JavaScript**: General code structure, naming, and quality standards
- **Go**: Clean architecture patterns, receiver methods, directory structure compliance

### Automated Checks

#### Pre-Write Validation
Before any `Write` operation:
- File length validation (max 500 lines)
- Function length validation (max 80 lines)
- Forbidden pattern detection (TODO, FIXME, console.log)
- Unused import detection
- Private function naming (_prefix)

#### Pre-Edit Duplicate Detection
Before any `Edit` operation:
- Scans entire codebase for existing functions
- Detects duplicate function names and signatures
- **BLOCKS** operation if duplicate found
- Provides exact file:line location of existing function

#### Post-Operation Indexing
After `Write` or `Edit`:
- Updates function index (.claude/function-index.json)
- Maintains registry of all functions with signatures
- Enables fast duplicate detection

### Code Standards

#### Function Structure
- **Max Lines**: 80 (ideal: 20-40)
- **Single Responsibility**: One purpose per function
- **Error Handling**: Comprehensive validation
- **Naming**: Descriptive, intention-revealing

#### File Structure
- **Max Lines**: 500
- **Modular Design**: Extract utilities to shared files
- **Clear Organization**: Logical grouping of related functions

#### Naming Conventions
- **Private Functions**: Prefix with `_` (e.g., `_helperFunction`)
- **Public Functions**: camelCase (e.g., `calculateTotal`)
- **Constants**: UPPER_SNAKE_CASE (e.g., `MAX_RETRIES`)
- **Classes**: PascalCase (e.g., `UserService`)
- **Interfaces**: PascalCase with `I` prefix (e.g., `IUserData`)

#### Forbidden Patterns
- `TODO:` - Complete before committing
- `FIXME:` - Resolve before committing
- `console.log(` - Use proper logging
- Hardcoded credentials
- Unused imports

### Compliance Enforcement

The hooks are **mandatory** and will:
- ❌ **BLOCK** operations that create duplicates
- ⚠️  **WARN** about quality issues
- ✅ **APPROVE** compliant code changes

This ensures:
- No duplicate function implementations
- Consistent code structure
- Maintainable codebase
- Professional code quality

---

## Go Structure Standards

### Required Libraries
- `github.com/ternarybob/arbor` - All logging
- `github.com/ternarybob/banner` - Startup banners
- `github.com/pelletier/go-toml/v2` - TOML config

### Startup Sequence (main.go)
1. Configuration loading (`common.LoadFromFile`)
2. Logger initialization (`common.InitLogger`)
3. Banner display (`common.PrintBanner`)
4. Version management (`common.GetVersion`)
5. Service initialization
6. Handler initialization
7. Information logging

### Directory Structure
```
cmd/<project-name>/          Main entry point
internal/common/             Stateless utility functions - NO receiver methods
internal/services/           Stateful services with receiver methods
internal/handlers/           HTTP handlers (dependency injection)
internal/models/             Data models
internal/interfaces/         Service interface definitions
configs/                     Configuration files
deployments/                 Deployment configurations
  docker/                    Docker deployment
  local/                     Local development configs
scripts/                     Build and deployment scripts
.github/workflows/           CI/CD pipelines
```

### Critical Distinctions

#### `internal/services/` - Stateful Services (Receiver Methods)
```go
// ✅ CORRECT: Service with receiver methods
type UserService struct {
    db     *sql.DB
    logger *arbor.Logger
}

func (s *UserService) CreateUser(ctx context.Context, user *User) error {
    s.logger.Info("Creating user", "email", user.Email)
    return s.db.Save(user)
}
```

#### `internal/common/` - Stateless Utilities (Pure Functions)
```go
// ✅ CORRECT: Stateless pure function
func LoadFromFile(path string) (*Config, error) {
    // No receiver, no state
    return loadConfig(path)
}

// ❌ WRONG: Receiver method in common/
func (c *Config) LoadFromFile(path string) error {
    // This belongs in internal/services/
}
```

### Go-Specific Enforcement

#### Pre-Write/Edit Checks
- **Directory Rules**: Validates correct usage of `internal/common/` (no receivers) vs `internal/services/` (receivers required)
- **Duplicate Functions**: Prevents duplicate function names across codebase
- **Error Handling**: No ignored errors (`_ =`)
- **Logging Standards**: Must use `arbor` logger, no `fmt.Println`/`log.Println`
- **Startup Sequence**: Validates correct order in `main.go`
- **Interface Definitions**: Should be in `internal/interfaces/`

#### Example Violations

**❌ BLOCKED: Receiver method in internal/common/**
```go
// internal/common/config.go
func (c *Config) Load() error {  // ❌ ERROR
    // Common must be stateless!
}
```

**❌ BLOCKED: Stateless function in internal/services/**
```go
// internal/services/user_service.go
func CreateUser(user *User) error {  // ⚠️ WARNING
    // Services should use receiver methods!
}
```

**❌ BLOCKED: Using fmt.Println instead of logger**
```go
fmt.Println("User created")  // ❌ ERROR
logger.Info("User created")  // ✅ CORRECT
```

**❌ BLOCKED: Wrong startup sequence**
```go
common.InitLogger()      // ❌ ERROR
common.LoadFromFile()    // Must be first!
```

### Design Patterns

**Dependency Injection:**
```go
type UserHandler struct {
    userService interfaces.UserService  // Interface, not concrete type
}

func NewUserHandler(userService interfaces.UserService) *UserHandler {
    return &UserHandler{userService: userService}
}
```

**Interface-Based Design:**
```go
// internal/interfaces/user_service.go
type UserService interface {
    CreateUser(ctx context.Context, user *User) error
    GetUser(ctx context.Context, id string) (*User, error)
}
```

### Code Quality Rules
- Single Responsibility Principle
- Proper error handling (return errors, don't ignore)
- Interface-based design
- Table-driven tests
- DRY principle - consolidate duplicate code
- Remove unused/redundant functions
- Use receiver methods on services
- Keep common utilities stateless

---

## Build Agent

The build agent automates project compilation using standardized build scripts.

### Requirements

- **Build Script**: MUST use `./scripts/build.ps1` (PowerShell)
- **Location**: `scripts/build.ps1` OR project root `./scripts/build.ps1`
- **Execution**: Always run from project root directory

### Build Script Parameters

Standard parameters supported:
- `-Environment dev|staging|prod` - Target environment
- `-Clean` - Remove old artifacts before build
- `-Test` - Run tests before building
- `-Release` - Optimized production build
- `-Verbose` - Detailed build output
- `-Run` - Start application after build
- `-Version "X.Y.Z"` - Specific version number

### Build Process

1. **Version Management**
   - Reads `.version` file
   - Auto-increments patch version
   - Embeds version in binary

2. **Dependency Resolution**
   - Go: `go mod tidy && go mod download`
   - Node: `npm install`
   - Restores all dependencies

3. **Compilation**
   - Builds binary with version info
   - Applies build flags (-ldflags)
   - Outputs to `bin/` directory

4. **Post-Build**
   - Copies configuration files
   - Deploys static assets
   - Verifies output artifacts

### Usage Examples

**Standard build:**
```powershell
./scripts/build.ps1
```

**Production build with tests:**
```powershell
./scripts/build.ps1 -Environment prod -Test -Release
```

**Clean build and run:**
```powershell
./scripts/build.ps1 -Clean -Run
```

### Build Verification

After build, verify:
- ✓ Binary exists in `bin/`
- ✓ Version embedded correctly
- ✓ Configuration files copied
- ✓ File size reasonable
- ✓ Executable permissions set

---

## Test Agent

The test agent runs automated tests using standardized test runners.

### Requirements

- **Test Script**: MUST use `run-tests.ps1` (PowerShell)
- **Location**: ONLY `tests/run-tests.ps1` (NOT root directory)
- **Execution**: Run from tests/ directory OR project root

### Test Script Parameters

Standard parameters supported:
- `-Type all|api|ui|unit|integration` - Test category
- `-Test "pattern"` - Run specific test by name
- `-Verbose` - Detailed test output
- `-Coverage` - Generate coverage report

### Test Process

1. **Pre-Test Build**
   - Calls `./scripts/build.ps1` to compile latest code
   - Ensures tests run against current version

2. **Service Management**
   - Starts application/service
   - Waits for readiness
   - Runs tests
   - Stops service when complete

3. **Test Execution**
   - Runs tests by type (api, ui, unit, etc.)
   - Captures output to logs
   - Saves screenshots (UI tests)
   - Generates timestamped results

4. **Result Collection**
   - Creates `tests/results/run_[timestamp]/` directory (REQUIRED)
   - Saves logs per test type
   - Generates summary report

### Usage Examples

**Run all tests:**
```powershell
./tests/run-tests.ps1 -Type all
```

**Run API tests only:**
```powershell
./tests/run-tests.ps1 -Type api
```

**Run specific test:**
```powershell
./tests/run-tests.ps1 -Test "TestUserAuthentication"
```

### Test Results Structure

```
tests/results/
  run_2025-10-03_08-30-00/
    test-results.log          # Overall summary
    api/
      test.log                # API test output
    ui/
      test.log                # UI test output
      screenshot_*.png        # UI screenshots
```

### Test Verification

After tests, check:
- ✓ Exit code (0 = pass, 1 = fail)
- ✓ Total tests run
- ✓ Pass/fail counts
- ✓ Log files created
- ✓ Failure details captured

---

## Agent Usage with Claude Code

### Build Project

Ask Claude Code:
```
Build the project for production with tests
```

Claude Code will:
1. Locate `./scripts/build.ps1`
2. Execute with `-Environment prod -Test -Release`
3. Report build summary

### Run Tests

Ask Claude Code:
```
Run all tests
```

Claude Code will:
1. Locate `./tests/run-tests.ps1`
2. Execute with `-Type all`
3. Report test results summary

### Build and Test

Ask Claude Code:
```
Build and test the project
```

Claude Code will:
1. Execute build first
2. Then run tests
3. Report both summaries
