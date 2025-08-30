// Content script for extracting page information

// Extract structured content from the page
function extractPageContent() {
  // Get basic page info
  const pageInfo = {
    url: window.location.href,
    title: document.title || '',
    html: document.documentElement.outerHTML,
    text: document.body ? document.body.innerText : '',
    metadata: {},
    links: [],
    selection: window.getSelection().toString()
  };
  
  // Extract metadata
  const metaTags = document.querySelectorAll('meta');
  metaTags.forEach(meta => {
    const name = meta.getAttribute('name') || meta.getAttribute('property');
    const content = meta.getAttribute('content');
    if (name && content) {
      pageInfo.metadata[name] = content;
    }
  });
  
  // Extract author if available
  const author = pageInfo.metadata['author'] || 
                 pageInfo.metadata['article:author'] || 
                 pageInfo.metadata['twitter:creator'];
  if (author) {
    pageInfo.metadata['author'] = author;
  }
  
  // Extract publication date
  const date = pageInfo.metadata['article:published_time'] || 
               pageInfo.metadata['datePublished'] ||
               pageInfo.metadata['date'];
  if (date) {
    pageInfo.metadata['date'] = date;
  }
  
  // Extract publication/site name
  const publication = pageInfo.metadata['og:site_name'] || 
                     pageInfo.metadata['publisher'] ||
                     pageInfo.metadata['twitter:site'];
  if (publication) {
    pageInfo.metadata['publication'] = publication;
  }
  
  // Extract links with context
  const links = document.querySelectorAll('a[href]');
  const seenUrls = new Set();
  
  links.forEach(link => {
    const href = link.href;
    
    // Skip internal fragments, javascript, and mailto links
    if (href.startsWith('#') || 
        href.startsWith('javascript:') || 
        href.startsWith('mailto:') ||
        seenUrls.has(href)) {
      return;
    }
    
    seenUrls.add(href);
    
    // Get link text and surrounding context
    const linkText = link.textContent.trim();
    
    // Try to get surrounding text context
    let context = '';
    const parent = link.parentElement;
    if (parent) {
      // Get text before and after the link
      const parentText = parent.textContent;
      const linkIndex = parentText.indexOf(linkText);
      
      if (linkIndex !== -1) {
        const beforeStart = Math.max(0, linkIndex - 100);
        const afterEnd = Math.min(parentText.length, linkIndex + linkText.length + 100);
        
        const before = parentText.substring(beforeStart, linkIndex).trim();
        const after = parentText.substring(linkIndex + linkText.length, afterEnd).trim();
        
        context = (before + ' ' + linkText + ' ' + after).trim();
      } else {
        context = linkText;
      }
    }
    
    pageInfo.links.push({
      url: href,
      text: linkText || href,
      context: context || linkText
    });
  });
  
  // Special handling for Bluesky posts
  if (window.location.hostname.includes('bsky.app') || 
      window.location.hostname.includes('bsky.social')) {
    pageInfo.metadata['platform'] = 'bluesky';
    
    // Try to extract post author and content
    const authorElem = document.querySelector('[data-testid="profileHeaderDisplayName"]');
    if (authorElem) {
      pageInfo.metadata['bsky_author'] = authorElem.textContent;
    }
  }
  
  // Special handling for Twitter/X
  if (window.location.hostname.includes('twitter.com') || 
      window.location.hostname.includes('x.com')) {
    pageInfo.metadata['platform'] = 'twitter';
  }
  
  return pageInfo;
}

// Listen for messages from popup
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
  if (request.action === 'extractContent') {
    const content = extractPageContent();
    sendResponse(content);
  } else if (request.action === 'getSelection') {
    const selection = window.getSelection().toString();
    sendResponse({ selection });
  }
  return true; // Keep message channel open for async response
});