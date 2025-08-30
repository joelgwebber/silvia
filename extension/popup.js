// Popup script for the Silvia extension

let serverUrl = 'http://localhost:8765';
let authToken = '';
let isConnected = false;

// Load settings on popup open
document.addEventListener('DOMContentLoaded', async () => {
  // Load saved settings
  const settings = await chrome.storage.local.get(['serverUrl', 'authToken']);
  if (settings.serverUrl) {
    serverUrl = settings.serverUrl;
    document.getElementById('server-url').value = serverUrl;
  }
  if (settings.authToken) {
    authToken = settings.authToken;
    document.getElementById('auth-token').value = authToken;
  }
  
  // Check connection status
  await checkConnection();
  
  // Get current tab info
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tab) {
    document.getElementById('page-title').textContent = tab.title || 'Untitled';
    document.getElementById('page-url').textContent = tab.url || '';
  }
  
  // Setup event listeners
  document.getElementById('capture-btn').addEventListener('click', captureContent);
  document.getElementById('save-settings').addEventListener('click', saveSettings);
  document.getElementById('capture-selection').addEventListener('change', toggleSelectionMode);
});

// Check connection to Silvia server
async function checkConnection() {
  const statusEl = document.getElementById('status');
  const statusTextEl = document.getElementById('status-text');
  const captureBtn = document.getElementById('capture-btn');
  
  try {
    const response = await fetch(`${serverUrl}/api/status`, {
      method: 'GET',
      headers: authToken ? { 'Authorization': `Bearer ${authToken}` } : {}
    });
    
    if (response.ok) {
      const data = await response.json();
      isConnected = true;
      statusEl.classList.add('connected');
      statusTextEl.textContent = 'Connected to Silvia';
      captureBtn.disabled = false;
      
      if (data.ingesting) {
        statusTextEl.textContent = 'Silvia is processing...';
        captureBtn.disabled = true;
      }
    } else {
      throw new Error(`Server returned ${response.status}`);
    }
  } catch (error) {
    isConnected = false;
    statusEl.classList.remove('connected');
    statusTextEl.textContent = 'Not connected';
    captureBtn.disabled = true;
    
    showMessage('error', `Cannot connect to Silvia. Make sure it's running on ${serverUrl}`);
  }
}

// Save connection settings
async function saveSettings() {
  serverUrl = document.getElementById('server-url').value;
  authToken = document.getElementById('auth-token').value;
  
  await chrome.storage.local.set({ serverUrl, authToken });
  
  showMessage('success', 'Settings saved');
  
  // Check connection with new settings
  await checkConnection();
}

// Toggle selection mode
function toggleSelectionMode(event) {
  const captureLinks = document.getElementById('capture-links');
  if (event.target.checked) {
    captureLinks.disabled = true;
    captureLinks.checked = false;
  } else {
    captureLinks.disabled = false;
    captureLinks.checked = true;
  }
}

// Capture content from the current page
async function captureContent() {
  const captureBtn = document.getElementById('capture-btn');
  const originalText = captureBtn.textContent;
  
  // Update button to show loading
  captureBtn.disabled = true;
  captureBtn.innerHTML = '<span class="loading"></span> Capturing...';
  
  try {
    // Get current tab
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    
    if (!tab) {
      throw new Error('No active tab found');
    }
    
    // Inject content script if needed and extract content
    const results = await chrome.tabs.sendMessage(tab.id, { action: 'extractContent' });
    
    if (!results) {
      throw new Error('Failed to extract content from page');
    }
    
    // Check capture options
    const captureLinks = document.getElementById('capture-links').checked;
    const captureSelection = document.getElementById('capture-selection').checked;
    const forceUpdate = document.getElementById('force-update').checked;
    
    // Prepare request data
    const requestData = {
      url: results.url,
      title: results.title,
      metadata: results.metadata || {}
    };
    
    // Handle selection-only mode
    if (captureSelection && results.selection) {
      requestData.text = results.selection;
      requestData.html = ''; // Don't send full HTML in selection mode
      requestData.links = []; // No links in selection mode
    } else {
      requestData.html = results.html;
      requestData.text = results.text;
      requestData.links = captureLinks ? results.links : [];
      
      // Debug logging
      console.log('Capture links enabled:', captureLinks);
      console.log('Number of links found:', results.links ? results.links.length : 0);
      if (results.links && results.links.length > 0) {
        console.log('Sample links:', results.links.slice(0, 3));
      }
    }
    
    // Add capture metadata
    requestData.metadata['captured_at'] = new Date().toISOString();
    requestData.metadata['capture_mode'] = captureSelection ? 'selection' : 'full';
    
    // Add force flag if enabled
    if (forceUpdate) {
      requestData.force = true;
      console.log('Force update enabled - will re-process existing source');
    }
    
    // Send to Silvia
    const response = await fetch(`${serverUrl}/api/ingest`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(authToken ? { 'Authorization': `Bearer ${authToken}` } : {})
      },
      body: JSON.stringify(requestData)
    });
    
    const responseData = await response.json();
    
    if (response.ok && responseData.success) {
      showMessage('success', responseData.message || 'Content captured successfully!');
      
      // Show stats if available
      if (responseData.stats) {
        document.getElementById('stats').style.display = 'flex';
        document.getElementById('stat-entities').textContent = responseData.stats.entities || 0;
        document.getElementById('stat-links').textContent = responseData.stats.links || 0;
      }
      
      // Close popup after a delay
      setTimeout(() => {
        window.close();
      }, 2000);
    } else {
      throw new Error(responseData.error || 'Failed to capture content');
    }
    
  } catch (error) {
    console.error('Capture error:', error);
    showMessage('error', error.message || 'Failed to capture content');
  } finally {
    // Restore button
    captureBtn.disabled = false;
    captureBtn.textContent = originalText;
  }
}

// Show message to user
function showMessage(type, text) {
  const messageEl = document.getElementById('message');
  messageEl.className = `message ${type}`;
  messageEl.textContent = text;
  messageEl.style.display = 'block';
  
  // Auto-hide after 5 seconds
  setTimeout(() => {
    messageEl.style.display = 'none';
  }, 5000);
}