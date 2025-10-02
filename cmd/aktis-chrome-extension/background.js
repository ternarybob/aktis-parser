// Background service worker for Aktis Parser extension

console.log('Aktis Parser extension loaded');

// Open side panel when extension icon is clicked
chrome.action.onClicked.addListener(async (tab) => {
  // Open the side panel for the current window
  await chrome.sidePanel.open({ windowId: tab.windowId });
});

// Listen for messages from popup/sidepanel
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'captureAuth') {
    captureAuthData()
      .then(authData => {
        sendResponse({ success: true, data: authData });
      })
      .catch(error => {
        sendResponse({ success: false, error: error.message });
      });
    return true; // Keep message channel open for async response
  }
});

// Capture authentication data from current tab
async function captureAuthData() {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });

  if (!tab || !tab.url) {
    throw new Error('No active tab found');
  }

  const url = new URL(tab.url);
  const baseURL = `${url.protocol}//${url.host}`;

  // Get cookies for the domain
  const cookies = await chrome.cookies.getAll({ url: baseURL });

  // Extract tokens from cookies
  const tokens = {};
  for (const cookie of cookies) {
    if (cookie.name.includes('cloud') || cookie.name.includes('atl')) {
      tokens[cookie.name] = cookie.value;
    }
  }

  // Get user agent
  const userAgent = navigator.userAgent;

  // Return auth data
  return {
    cookies: cookies,
    tokens: tokens,
    userAgent: userAgent,
    baseUrl: baseURL,
    timestamp: Date.now()
  };
}
