package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// Source represents a fetched and processed source
type Source struct {
	URL        string
	Title      string
	Content    string   // Markdown content
	RawContent string   // Original HTML/text
	Links      []string // Extracted links
	Metadata   map[string]string
}

// Fetcher interface for different source types
type Fetcher interface {
	Fetch(ctx context.Context, sourceURL string) (*Source, error)
	CanHandle(sourceURL string) bool
}

// Manager handles fetching from various sources
type Manager struct {
	fetchers []Fetcher
}

// NewManager creates a new source manager
func NewManager() *Manager {
	m := &Manager{}
	// Register fetchers in priority order
	m.fetchers = []Fetcher{
		NewBskyFetcher(nil), // Will need bsky client
		NewWebFetcher(),
	}
	return m
}

// Fetch retrieves and processes a source
func (m *Manager) Fetch(ctx context.Context, sourceURL string) (*Source, error) {
	// Find appropriate fetcher
	for _, fetcher := range m.fetchers {
		if fetcher.CanHandle(sourceURL) {
			return fetcher.Fetch(ctx, sourceURL)
		}
	}

	return nil, fmt.Errorf("no fetcher available for URL: %s", sourceURL)
}

// ExtractDomain gets the domain from a URL
func ExtractDomain(sourceURL string) string {
	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}

	host := u.Host
	// Remove www. prefix
	host = strings.TrimPrefix(host, "www.")

	return host
}

// ExtractLinks finds all URLs in content
func ExtractLinks(content string) []string {
	var links []string
	seen := make(map[string]bool)

	// Simple regex-based extraction (can be improved)
	// Looking for http(s) URLs
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		words := strings.Fields(line)
		for _, word := range words {
			if strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://") {
				// Clean up common trailing punctuation
				word = strings.TrimSuffix(word, ".")
				word = strings.TrimSuffix(word, ",")
				word = strings.TrimSuffix(word, ")")
				word = strings.TrimSuffix(word, "]")

				if !seen[word] {
					links = append(links, word)
					seen[word] = true
				}
			}
		}
	}

	return links
}
