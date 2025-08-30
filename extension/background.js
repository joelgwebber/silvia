// Background service worker for the Silvia extension

// Listen for installation
chrome.runtime.onInstalled.addListener(() => {
  console.log('Silvia extension installed');
  
  // Set default settings
  chrome.storage.local.get(['serverUrl', 'authToken'], (result) => {
    if (!result.serverUrl) {
      chrome.storage.local.set({
        serverUrl: 'http://localhost:8765',
        authToken: ''
      });
    }
  });
  
  // Create context menu for right-click capture
  chrome.contextMenus.create({
    id: 'silvia-capture',
    title: 'Capture to Silvia',
    contexts: ['page', 'selection', 'link']
  });
});

// Handle context menu clicks
chrome.contextMenus.onClicked.addListener((info, tab) => {
  if (info.menuItemId === 'silvia-capture') {
    // For now, we'll send a message to the content script to trigger capture
    // Note: chrome.action.openPopup() doesn't work in service workers
    // Instead, we could implement direct capture or show a notification
    
    if (tab && tab.id) {
      // Send message to content script to extract content
      chrome.tabs.sendMessage(tab.id, { action: 'extractContent' }, (response) => {
        if (chrome.runtime.lastError) {
          console.error('Error extracting content:', chrome.runtime.lastError);
          return;
        }
        
        // Here we could directly send to the server
        // For now, just log that the feature was triggered
        console.log('Context menu capture triggered for:', tab.url);
      });
    }
  }
});