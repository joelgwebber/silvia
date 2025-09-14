package sources

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"silvia/internal/bsky"
)

// BskyFetcher handles Bluesky URLs
type BskyFetcher struct {
	client *bsky.Client
}

// NewBskyFetcher creates a new Bluesky fetcher
func NewBskyFetcher(client *bsky.Client) *BskyFetcher {
	return &BskyFetcher{
		client: client,
	}
}

// CanHandle checks if this fetcher can handle the URL
func (b *BskyFetcher) CanHandle(sourceURL string) bool {
	return strings.Contains(sourceURL, "bsky.app") || strings.Contains(sourceURL, "bsky.social")
}

// Fetch retrieves and converts a Bluesky post/thread to markdown
func (b *BskyFetcher) Fetch(ctx context.Context, sourceURL string) (*Source, error) {
	if b.client == nil {
		return nil, fmt.Errorf("Bluesky client not configured")
	}

	// Parse URL to extract post ID
	// Format: https://bsky.app/profile/{handle}/post/{postId}
	postIDRe := regexp.MustCompile(`/profile/([^/]+)/post/([^/?]+)`)
	matches := postIDRe.FindStringSubmatch(sourceURL)

	if len(matches) < 3 {
		return nil, fmt.Errorf("invalid Bluesky URL format")
	}

	handle := matches[1]
	postID := matches[2]

	// For now, create a placeholder
	// TODO (Future Enhancement): Implement actual Bluesky API calls to fetch post and thread
	// Currently returns a placeholder that doesn't break functionality

	markdown := fmt.Sprintf(`# Bluesky Post

**Author**: @%s
**Post ID**: %s
**URL**: %s

## Content

[Post content would be fetched here]

## Thread

[Thread replies would be fetched here]

---
*Fetched: %s*
`, handle, postID, sourceURL, time.Now().Format("2006-01-02 15:04:05"))

	source := &Source{
		URL:        sourceURL,
		Title:      fmt.Sprintf("Bluesky post by @%s", handle),
		Content:    markdown,
		RawContent: "",         // Would contain raw API response
		Links:      []string{}, // Would extract from post content
		Metadata: map[string]string{
			"fetched_at": time.Now().Format(time.RFC3339),
			"domain":     "bsky.app",
			"handle":     handle,
			"post_id":    postID,
		},
	}

	return source, nil
}
