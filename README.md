# aktis-parser

A Go service that scrapes and stores Jira projects, issues, and Confluence pages locally using BoltDB.

## Features

- Scrapes Jira projects and issues via REST API
- Scrapes Confluence spaces and pages
- Local BoltDB storage
- Browser extension authentication integration
- Rate-limited API requests
- Background scraping

## Build

```bash
go mod tidy
go build -o bin/aktis-parser.exe ./cmd/service.go
```

## Usage

1. Start the service:
   ```bash
   ./bin/aktis-parser.exe
   ```

2. Service runs on `http://localhost:8080`

3. Install browser extension and authenticate to Jira/Confluence

4. Extension sends auth data to `/api/auth` endpoint

5. Service automatically scrapes in background

## API Endpoints

- `POST /api/auth` - Update authentication and start scraping
- `GET /api/scrape` - Manually trigger scraping

## Storage

Data is stored in `scraper.db` (BoltDB) with three buckets:
- `projects` - Jira projects
- `issues` - Jira issues
- `confluence_pages` - Confluence pages

┌─────────────────────────────────────┐
│  Extension (one-time/refresh)       │
│  1. User logs in normally (2FA)     │
│  2. Extract auth state              │
│  3. POST to service                 │
└──────────────┬──────────────────────┘
               │ (cookies, tokens, headers)
               ↓
┌─────────────────────────────────────┐
│  Service (Go) - fully autonomous    │
│  • Makes authenticated HTTP calls   │
│  • Crawls projects, issues, pages   │
│  • Extracts & stores data           │
│  • Runs continuously/scheduled      │
└──────────────┬──────────────────────┘
               ↓
           BoltDB

// extension/background.js

const SERVICE_URL = 'http://localhost:8080';

// Extract complete auth state
async function extractAuthState() {
    console.log('Extracting auth state...');
    
    // 1. Get all Atlassian cookies
    const cookies = await chrome.cookies.getAll({
        domain: '.atlassian.net'
    });
    
    // 2. Get tokens from a Jira/Confluence page
    const [tab] = await chrome.tabs.query({
        url: '*://*.atlassian.net/*',
        active: true
    });
    
    if (!tab) {
        console.log('No Atlassian tab found, please open Jira or Confluence');
        return;
    }
    
    // 3. Extract localStorage, sessionStorage, and page-specific tokens
    const [result] = await chrome.scripting.executeScript({
        target: { tabId: tab.id },
        func: () => {
            // Get storage
            const storage = {
                localStorage: Object.keys(localStorage).reduce((acc, key) => {
                    acc[key] = localStorage.getItem(key);
                    return acc;
                }, {}),
                sessionStorage: Object.keys(sessionStorage).reduce((acc, key) => {
                    acc[key] = sessionStorage.getItem(key);
                    return acc;
                }, {})
            };
            
            // Try to find JWT or cloud session tokens
            const scripts = Array.from(document.querySelectorAll('script'));
            let cloudId = null;
            let atlToken = null;
            
            scripts.forEach(script => {
                const content = script.textContent;
                // Look for cloudId
                const cloudMatch = content.match(/cloudId["']?\s*:\s*["']([^"']+)/);
                if (cloudMatch) cloudId = cloudMatch[1];
                
                // Look for atl_token
                const tokenMatch = content.match(/atl_token["']?\s*:\s*["']([^"']+)/);
                if (tokenMatch) atlToken = tokenMatch[1];
            });
            
            return {
                storage,
                cloudId,
                atlToken,
                href: window.location.href
            };
        }
    });
    
    // 4. Compile complete auth package
    const authData = {
        cookies: cookies,
        tokens: result.result,
        userAgent: navigator.userAgent,
        baseUrl: new URL(tab.url).origin,
        timestamp: Date.now()
    };
    
    // 5. Send to service
    try {
        const response = await fetch(`${SERVICE_URL}/api/auth`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(authData)
        });
        
        if (response.ok) {
            console.log('✓ Auth sent to service successfully');
            chrome.notifications.create({
                type: 'basic',
                iconUrl: 'icon.png',
                title: 'Scraper Authenticated',
                message: 'Service is now scraping your Jira/Confluence data'
            });
        }
    } catch (error) {
        console.error('Failed to send auth:', error);
    }
}

// Trigger auth extraction
chrome.action.onClicked.addListener(() => {
    extractAuthState();
});

// Auto-refresh auth every 30 minutes
chrome.alarms.create('refreshAuth', { periodInMinutes: 30 });
chrome.alarms.onAlarm.addListener((alarm) => {
    if (alarm.name === 'refreshAuth') {
        extractAuthState();
    }
});

// Also extract on extension install
chrome.runtime.onInstalled.addListener(() => {
    console.log('Extension installed. Click icon when logged into Jira/Confluence');
});