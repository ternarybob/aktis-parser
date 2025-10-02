# Aktis Parser Chrome Extension

This Chrome extension captures authentication data from your active Jira/Confluence session and sends it to the Aktis Parser service.

## Installation

1. Build the project using `.\scripts\build.ps1`
2. Open Chrome and navigate to `chrome://extensions/`
3. Enable "Developer mode" in the top right
4. Click "Load unpacked"
5. Select the `bin/aktis-chrome-extension` directory

## Usage

1. Start the Aktis Parser service (it runs on `http://localhost:8080`)
2. Navigate to your Jira or Confluence instance and log in
3. Click the Aktis Parser extension icon in Chrome
4. Click "Capture Auth Data"
5. The extension will capture your authentication and send it to the service
6. The service will automatically start scraping

## Security

- Authentication data is only sent to `localhost:8080`
- No data is sent to external servers
- All communication is local to your machine
