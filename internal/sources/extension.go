package sources

import (
	"regexp"
	"strings"
)

// ConvertHTMLToMarkdown is exported for use by extension ingestion
func (w *WebFetcher) ConvertHTMLToMarkdown(html string) string {
	return htmlToMarkdown(html)
}

// ExtractTitleFromHTML is exported for use by extension ingestion
func (w *WebFetcher) ExtractTitleFromHTML(html string) string {
	return extractTitle(html)
}

// ExtractHTMLLinksWithContext extracts links with surrounding context
func ExtractHTMLLinksWithContext(html string) map[string]string {
	linkContext := make(map[string]string)
	
	// Extract links with their surrounding text
	// Pattern: text before <a href="...">link text</a> text after
	linkRe := regexp.MustCompile(`(?is)([^<]{0,100})<a[^>]+href="([^"]+)"[^>]*>([^<]+)</a>([^<]{0,100})`)
	matches := linkRe.FindAllStringSubmatch(html, -1)
	
	for _, match := range matches {
		if len(match) >= 4 {
			beforeText := stripHTMLTags(match[1])
			url := match[2]
			linkText := stripHTMLTags(match[3])
			afterText := stripHTMLTags(match[4])
			
			// Skip fragments and javascript
			if strings.HasPrefix(url, "#") || strings.HasPrefix(url, "javascript:") || strings.HasPrefix(url, "mailto:") {
				continue
			}
			
			// Build context
			context := strings.TrimSpace(beforeText + " " + linkText + " " + afterText)
			
			// Clean up excessive whitespace
			context = regexp.MustCompile(`\s+`).ReplaceAllString(context, " ")
			
			if context != "" && context != linkText {
				linkContext[url] = context
			} else if linkText != "" {
				linkContext[url] = linkText
			}
		}
	}
	
	return linkContext
}