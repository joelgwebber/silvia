package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// WebFetcher handles generic web URLs
type WebFetcher struct {
	client *http.Client
}

// NewWebFetcher creates a new web fetcher
func NewWebFetcher() *WebFetcher {
	return &WebFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CanHandle checks if this fetcher can handle the URL
func (w *WebFetcher) CanHandle(sourceURL string) bool {
	return strings.HasPrefix(sourceURL, "http://") || strings.HasPrefix(sourceURL, "https://")
}

// Fetch retrieves and converts a web page to markdown
func (w *WebFetcher) Fetch(ctx context.Context, sourceURL string) (*Source, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set user agent to avoid blocks
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Silvia/1.0; +https://github.com/silvia)")
	
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	html := string(body)
	
	// Extract title
	title := extractTitle(html)
	
	// Convert to markdown
	markdown := htmlToMarkdown(html)
	
	// Extract links
	links := extractHTMLLinks(html)
	
	source := &Source{
		URL:        sourceURL,
		Title:      title,
		Content:    markdown,
		RawContent: html,
		Links:      links,
		Metadata: map[string]string{
			"fetched_at": time.Now().Format(time.RFC3339),
			"domain":     ExtractDomain(sourceURL),
		},
	}
	
	return source, nil
}

// extractTitle extracts the title from HTML
func extractTitle(html string) string {
	// Try <title> tag
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	if matches := titleRe.FindStringSubmatch(html); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Try og:title meta tag
	ogTitleRe := regexp.MustCompile(`(?i)<meta\s+property="og:title"\s+content="([^"]+)"`)
	if matches := ogTitleRe.FindStringSubmatch(html); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	
	// Try h1
	h1Re := regexp.MustCompile(`(?i)<h1[^>]*>([^<]+)</h1>`)
	if matches := h1Re.FindStringSubmatch(html); len(matches) > 1 {
		return strings.TrimSpace(stripHTMLTags(matches[1]))
	}
	
	return "Untitled"
}

// htmlToMarkdown converts HTML to markdown
func htmlToMarkdown(html string) string {
	// Remove script and style tags
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")
	
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")
	
	// Extract body content if present
	bodyRe := regexp.MustCompile(`(?is)<body[^>]*>(.*)</body>`)
	if matches := bodyRe.FindStringSubmatch(html); len(matches) > 1 {
		html = matches[1]
	}
	
	// Convert headers
	html = regexp.MustCompile(`(?i)<h1[^>]*>`).ReplaceAllString(html, "\n# ")
	html = regexp.MustCompile(`(?i)</h1>`).ReplaceAllString(html, "\n\n")
	
	html = regexp.MustCompile(`(?i)<h2[^>]*>`).ReplaceAllString(html, "\n## ")
	html = regexp.MustCompile(`(?i)</h2>`).ReplaceAllString(html, "\n\n")
	
	html = regexp.MustCompile(`(?i)<h3[^>]*>`).ReplaceAllString(html, "\n### ")
	html = regexp.MustCompile(`(?i)</h3>`).ReplaceAllString(html, "\n\n")
	
	// Convert paragraphs
	html = regexp.MustCompile(`(?i)<p[^>]*>`).ReplaceAllString(html, "\n\n")
	html = regexp.MustCompile(`(?i)</p>`).ReplaceAllString(html, "\n\n")
	
	// Convert line breaks
	html = regexp.MustCompile(`(?i)<br[^>]*>`).ReplaceAllString(html, "\n")
	
	// Convert links (keep URL)
	linkRe := regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"[^>]*>([^<]+)</a>`)
	html = linkRe.ReplaceAllString(html, "[$2]($1)")
	
	// Convert bold
	html = regexp.MustCompile(`(?i)<b[^>]*>`).ReplaceAllString(html, "**")
	html = regexp.MustCompile(`(?i)</b>`).ReplaceAllString(html, "**")
	html = regexp.MustCompile(`(?i)<strong[^>]*>`).ReplaceAllString(html, "**")
	html = regexp.MustCompile(`(?i)</strong>`).ReplaceAllString(html, "**")
	
	// Convert italic
	html = regexp.MustCompile(`(?i)<i[^>]*>`).ReplaceAllString(html, "*")
	html = regexp.MustCompile(`(?i)</i>`).ReplaceAllString(html, "*")
	html = regexp.MustCompile(`(?i)<em[^>]*>`).ReplaceAllString(html, "*")
	html = regexp.MustCompile(`(?i)</em>`).ReplaceAllString(html, "*")
	
	// Convert lists
	html = regexp.MustCompile(`(?i)<li[^>]*>`).ReplaceAllString(html, "\n- ")
	html = regexp.MustCompile(`(?i)</li>`).ReplaceAllString(html, "")
	
	// Strip remaining HTML tags
	html = stripHTMLTags(html)
	
	// Clean up excessive whitespace
	html = regexp.MustCompile(`\n{3,}`).ReplaceAllString(html, "\n\n")
	html = strings.TrimSpace(html)
	
	return html
}

// stripHTMLTags removes all HTML tags
func stripHTMLTags(html string) string {
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, "")
	
	// Decode common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&rsquo;", "'")
	text = strings.ReplaceAll(text, "&lsquo;", "'")
	text = strings.ReplaceAll(text, "&rdquo;", "\"")
	text = strings.ReplaceAll(text, "&ldquo;", "\"")
	text = strings.ReplaceAll(text, "&mdash;", "—")
	text = strings.ReplaceAll(text, "&ndash;", "–")
	
	return text
}

// extractHTMLLinks extracts all links from HTML
func extractHTMLLinks(html string) []string {
	var links []string
	seen := make(map[string]bool)
	
	// Extract href attributes
	hrefRe := regexp.MustCompile(`(?i)href="([^"]+)"`)
	matches := hrefRe.FindAllStringSubmatch(html, -1)
	
	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			// Skip fragments and javascript
			if strings.HasPrefix(link, "#") || strings.HasPrefix(link, "javascript:") {
				continue
			}
			// Skip mailto
			if strings.HasPrefix(link, "mailto:") {
				continue
			}
			
			if !seen[link] {
				links = append(links, link)
				seen[link] = true
			}
		}
	}
	
	return links
}