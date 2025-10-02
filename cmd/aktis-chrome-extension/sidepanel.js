// Sidepanel script for Aktis Parser extension

const DEFAULT_SERVER_URL = 'http://localhost:8080';

// Initialize
document.addEventListener('DOMContentLoaded', async () => {
  await loadSettings();
  await checkServerStatus();
  await updatePageInfo();

  // Set up event listeners
  document.getElementById('capture-auth-btn').addEventListener('click', captureAuth);
  document.getElementById('refresh-status-btn').addEventListener('click', refreshStatus);
  document.getElementById('save-settings-btn').addEventListener('click', saveSettings);

  // Auto-refresh status every 30 seconds
  setInterval(checkServerStatus, 30000);
});

// Load settings from storage
async function loadSettings() {
  const result = await chrome.storage.sync.get(['serverUrl']);
  const serverUrl = result.serverUrl || DEFAULT_SERVER_URL;
  document.getElementById('server-url').value = serverUrl;
}

// Save settings to storage
async function saveSettings() {
  const serverUrl = document.getElementById('server-url').value;
  await chrome.storage.sync.set({ serverUrl });
  showSuccess('Settings saved successfully');
}

// Check server status
async function checkServerStatus() {
  const serverUrl = document.getElementById('server-url').value;
  const statusElement = document.getElementById('server-status');
  const versionElement = document.getElementById('version-info');

  try {
    const response = await fetch(`${serverUrl}/api/health`);
    if (response.ok) {
      statusElement.textContent = 'Online';
      statusElement.className = 'status-value online';

      // Get server version
      const versionResponse = await fetch(`${serverUrl}/api/version`);
      if (versionResponse.ok) {
        const versionData = await versionResponse.json();
        versionElement.textContent = `Extension: v0.1.5 | Server: v${versionData.version}`;
      }
    } else {
      throw new Error('Server returned error');
    }
  } catch (error) {
    statusElement.textContent = 'Offline';
    statusElement.className = 'status-value offline';
    versionElement.textContent = 'Extension: v0.1.5 | Server: offline';
  }
}

// Update current page info
async function updatePageInfo() {
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (tab && tab.url) {
      const url = new URL(tab.url);
      document.getElementById('page-url').textContent = url.hostname;
    }
  } catch (error) {
    console.error('Error getting page info:', error);
  }
}

// Capture authentication
async function captureAuth() {
  const button = document.getElementById('capture-auth-btn');
  button.disabled = true;
  button.textContent = 'Capturing...';

  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

    if (!tab || !tab.url) {
      throw new Error('No active tab found');
    }

    const url = new URL(tab.url);
    const baseURL = `${url.protocol}//${url.host}`;

    // Get cookies
    const cookies = await chrome.cookies.getAll({ url: baseURL });

    // Extract tokens from cookies
    const tokens = {};
    for (const cookie of cookies) {
      if (cookie.name.includes('cloud') || cookie.name.includes('atl')) {
        tokens[cookie.name] = cookie.value;
      }
    }

    // Build auth data
    const authData = {
      cookies: cookies,
      tokens: tokens,
      userAgent: navigator.userAgent,
      baseUrl: baseURL,
      timestamp: Date.now()
    };

    // Send to server
    const serverUrl = document.getElementById('server-url').value;
    const response = await fetch(`${serverUrl}/api/auth`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(authData)
    });

    if (!response.ok) {
      throw new Error(`Server error: ${response.status}`);
    }

    const result = await response.json();

    // Update last capture time
    const now = new Date().toLocaleString();
    document.getElementById('last-capture').textContent = now;
    await chrome.storage.sync.set({ lastCapture: now });

    showSuccess(result.message || 'Authentication captured! Scraping started.');

  } catch (error) {
    showError(`Error: ${error.message}`);
  } finally {
    button.disabled = false;
    button.textContent = 'Capture Authentication';
  }
}

// Refresh status
async function refreshStatus() {
  await checkServerStatus();
  await updatePageInfo();

  // Update last capture time from storage
  const result = await chrome.storage.sync.get(['lastCapture']);
  if (result.lastCapture) {
    document.getElementById('last-capture').textContent = result.lastCapture;
  }

  showSuccess('Status refreshed');
}

// Show success message
function showSuccess(message) {
  const element = document.getElementById('success-message');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => {
    element.style.display = 'none';
  }, 3000);
}

// Show error message
function showError(message) {
  const element = document.getElementById('error-message');
  element.textContent = message;
  element.style.display = 'block';
  setTimeout(() => {
    element.style.display = 'none';
  }, 5000);
}
