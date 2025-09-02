package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// FetchError provides detailed error information for fetch failures
type FetchError struct {
	URL        string
	StatusCode int
	Message    string
	NeedsAuth  bool
	Err        error
}

func (e *FetchError) Error() string {
	if e.NeedsAuth {
		return fmt.Sprintf("%s: %s (authentication required)", e.URL, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.URL, e.Message)
}

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
		return nil, &FetchError{
			URL:     sourceURL,
			Err:     err,
			Message: "failed to fetch URL",
		}
	}
	defer resp.Body.Close()

	// Check for authentication/paywall indicators
	if resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusPaymentRequired ||
		resp.StatusCode == http.StatusForbidden {
		return nil, &FetchError{
			URL:        sourceURL,
			StatusCode: resp.StatusCode,
			Message:    "authentication required",
			NeedsAuth:  true,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &FetchError{
			URL:        sourceURL,
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
		}
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	html := string(body)

	// Check for common paywall/login indicators in content
	lowerHTML := strings.ToLower(html)
	if (strings.Contains(lowerHTML, "please log in") ||
		strings.Contains(lowerHTML, "please sign in") ||
		strings.Contains(lowerHTML, "subscribe to continue") ||
		strings.Contains(lowerHTML, "create a free account") ||
		strings.Contains(lowerHTML, "you've reached your limit") ||
		strings.Contains(lowerHTML, "exclusive content for subscribers") ||
		strings.Contains(lowerHTML, "cloudflare") && strings.Contains(lowerHTML, "checking your browser")) &&
		len(html) < 50000 { // Paywalls tend to be small
		return nil, &FetchError{
			URL:       sourceURL,
			Message:   "detected paywall or authentication page",
			NeedsAuth: true,
		}
	}

	// Extract title
	title := extractTitle(html)

	// Convert to markdown
	markdown := htmlToMarkdown(html)

	// Extract links
	links := extractHTMLLinks(html)

	// Extract metadata
	metadata := map[string]string{
		"fetched_at": time.Now().Format(time.RFC3339),
		"domain":     ExtractDomain(sourceURL),
	}

	// Extract author if available
	if author := extractAuthor(html); author != "" {
		metadata["author"] = author
	}

	// Extract publication date if available
	if pubDate := extractPublicationDate(html); pubDate != "" {
		metadata["date"] = pubDate
	}

	// Extract publication/site name if available
	if publication := extractPublication(html); publication != "" {
		metadata["publication"] = publication
	}

	source := &Source{
		URL:        sourceURL,
		Title:      title,
		Content:    markdown,
		RawContent: html,
		Links:      links,
		Metadata:   metadata,
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
	// Remove script and style tags first
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")

	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")

	// Remove comments
	html = regexp.MustCompile(`(?is)<!--.*?-->`).ReplaceAllString(html, "")

	// Extract body content if present
	bodyRe := regexp.MustCompile(`(?is)<body[^>]*>(.*)</body>`)
	if matches := bodyRe.FindStringSubmatch(html); len(matches) > 1 {
		html = matches[1]
	}

	// IMPORTANT: Convert links FIRST, before other conversions
	// This ensures we catch all links before they might be affected by other transformations

	// Handle all <a> tags - use a more robust approach
	// Match any <a> tag with href, capturing everything until the closing </a>
	linkRe := regexp.MustCompile(`(?is)<a\s+[^>]*href\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a>`)
	var linkReplacements []struct {
		original    string
		replacement string
	}

	// Find all links first
	matches := linkRe.FindAllStringSubmatch(html, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			original := match[0]
			href := match[1]
			content := match[2]

			// Clean the content of nested HTML tags
			content = stripHTMLTags(content)
			content = strings.TrimSpace(content)

			// Skip empty links
			if content == "" {
				continue
			}

			// Store the replacement
			linkReplacements = append(linkReplacements, struct {
				original    string
				replacement string
			}{
				original:    original,
				replacement: fmt.Sprintf("[%s](%s)", content, href),
			})
		}
	}

	// Apply all link replacements
	for _, lr := range linkReplacements {
		html = strings.ReplaceAll(html, lr.original, lr.replacement)
	}

	// Now convert headers
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

// FetchFromClipboard creates a Source from clipboard content
func (w *WebFetcher) FetchFromClipboard(sourceURL string) (*Source, error) {
	// Get clipboard content based on OS
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "linux":
		// Try xclip first, then xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--output")
		} else {
			return nil, fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	default:
		return nil, fmt.Errorf("clipboard access not supported on %s", runtime.GOOS)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %w", err)
	}

	content := string(output)
	if len(content) == 0 {
		return nil, fmt.Errorf("clipboard is empty")
	}

	// Try to extract a title from the content
	title := "Manual capture"
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && len(lines[0]) > 0 && len(lines[0]) < 200 {
		// Use first line as title if it's reasonable
		title = strings.TrimSpace(lines[0])
	}

	// Check if content looks like HTML
	isHTML := strings.Contains(content, "<html") || strings.Contains(content, "<body") ||
		strings.Contains(content, "<article") || strings.Contains(content, "<p>")

	var markdown string
	var links []string

	if isHTML {
		// Process as HTML
		markdown = htmlToMarkdown(content)
		links = extractHTMLLinks(content)

		// Try to extract title from HTML
		if extractedTitle := extractTitle(content); extractedTitle != "Untitled" {
			title = extractedTitle
		}
	} else {
		// Treat as plain text/markdown
		markdown = content
		// Extract URLs from plain text
		urlRe := regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)
		matches := urlRe.FindAllString(content, -1)
		seen := make(map[string]bool)
		for _, match := range matches {
			if !seen[match] {
				links = append(links, match)
				seen[match] = true
			}
		}
	}

	// Extract metadata
	metadata := map[string]string{
		"fetched_at":     time.Now().Format(time.RFC3339),
		"domain":         ExtractDomain(sourceURL),
		"capture_method": "clipboard",
	}

	// Try to extract metadata from content if it looks like HTML
	if strings.Contains(content, "<") && strings.Contains(content, ">") {
		if author := extractAuthor(content); author != "" {
			metadata["author"] = author
		}
		if pubDate := extractPublicationDate(content); pubDate != "" {
			metadata["date"] = pubDate
		}
		if publication := extractPublication(content); publication != "" {
			metadata["publication"] = publication
		}
	}

	source := &Source{
		URL:        sourceURL,
		Title:      title,
		Content:    markdown,
		RawContent: content,
		Links:      links,
		Metadata:   metadata,
	}

	return source, nil
}

// OpenInBrowser opens a URL in the default browser
func OpenInBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// extractAuthor attempts to extract the author from HTML metadata
func extractAuthor(html string) string {
	// Try various meta tags for author
	patterns := []string{
		`(?i)<meta\s+name="author"\s+content="([^"]+)"`,
		`(?i)<meta\s+property="article:author"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="byl"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="DC\.Creator"\s+content="([^"]+)"`,
		`(?i)<span[^>]*class="[^"]*author[^"]*"[^>]*>([^<]+)</span>`,
		`(?i)<div[^>]*class="[^"]*byline[^"]*"[^>]*>(?:By\s+)?([^<]+)</div>`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			author := strings.TrimSpace(matches[1])
			// Clean up common prefixes
			author = strings.TrimPrefix(author, "By ")
			author = strings.TrimPrefix(author, "by ")
			return author
		}
	}

	return ""
}

// extractPublicationDate attempts to extract the publication date from HTML metadata
func extractPublicationDate(html string) string {
	// Try various meta tags for publication date
	patterns := []string{
		`(?i)<meta\s+property="article:published_time"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="publish_date"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="DC\.Date"\s+content="([^"]+)"`,
		`(?i)<time[^>]*datetime="([^"]+)"`,
		`(?i)<meta\s+property="datePublished"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="publication_date"\s+content="([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			dateStr := strings.TrimSpace(matches[1])
			// Try to parse and format the date consistently
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				return t.Format("January 2006")
			} else if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				return t.Format("January 2006")
			}
			// Return as-is if we can't parse it
			return dateStr
		}
	}

	return ""
}

// extractPublication attempts to extract the publication/site name from HTML metadata
func extractPublication(html string) string {
	// Try various meta tags for publication
	patterns := []string{
		`(?i)<meta\s+property="og:site_name"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="publisher"\s+content="([^"]+)"`,
		`(?i)<meta\s+property="article:publisher"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="DC\.Publisher"\s+content="([^"]+)"`,
		`(?i)<meta\s+name="twitter:site"\s+content="([^"]+)"`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(html); len(matches) > 1 {
			pub := strings.TrimSpace(matches[1])
			// Clean up Twitter handles
			pub = strings.TrimPrefix(pub, "@")
			return pub
		}
	}

	return ""
}
