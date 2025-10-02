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
