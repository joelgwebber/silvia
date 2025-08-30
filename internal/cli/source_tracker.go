package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SourceTracker keeps track of processed sources to avoid duplicates
type SourceTracker struct {
	mu         sync.RWMutex
	sources    map[string]*ProcessedSource
	filePath   string
	modified   bool
}

// ProcessedSource represents a source that has been ingested
type ProcessedSource struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	ProcessedAt time.Time `json:"processed_at"`
	Hash        string    `json:"hash"` // Hash of URL for quick lookup
	StoragePath string    `json:"storage_path,omitempty"`
}

// NewSourceTracker creates a new source tracker
func NewSourceTracker(dataDir string) *SourceTracker {
	tracker := &SourceTracker{
		sources:  make(map[string]*ProcessedSource),
		filePath: filepath.Join(dataDir, ".silvia", "processed_sources.json"),
	}
	
	// Load existing data
	tracker.Load()
	
	return tracker
}

// Load reads the processed sources from disk
func (st *SourceTracker) Load() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	data, err := os.ReadFile(st.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's ok
			return nil
		}
		return fmt.Errorf("failed to read source tracker: %w", err)
	}
	
	var sources []*ProcessedSource
	if err := json.Unmarshal(data, &sources); err != nil {
		return fmt.Errorf("failed to parse source tracker: %w", err)
	}
	
	// Rebuild map
	st.sources = make(map[string]*ProcessedSource)
	for _, source := range sources {
		st.sources[source.Hash] = source
	}
	
	return nil
}

// Save writes the processed sources to disk
func (st *SourceTracker) Save() error {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	if !st.modified {
		return nil
	}
	
	// Convert map to slice
	var sources []*ProcessedSource
	for _, source := range st.sources {
		sources = append(sources, source)
	}
	
	data, err := json.MarshalIndent(sources, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal source tracker: %w", err)
	}
	
	// Ensure directory exists
	dir := filepath.Dir(st.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tracker directory: %w", err)
	}
	
	if err := os.WriteFile(st.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write source tracker: %w", err)
	}
	
	st.modified = false
	return nil
}

// IsProcessed checks if a URL has been processed
func (st *SourceTracker) IsProcessed(url string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	hash := hashURL(url)
	_, exists := st.sources[hash]
	return exists
}

// GetProcessedSource returns info about a processed source
func (st *SourceTracker) GetProcessedSource(url string) *ProcessedSource {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	hash := hashURL(url)
	return st.sources[hash]
}

// MarkProcessed records that a URL has been processed
func (st *SourceTracker) MarkProcessed(url, title, storagePath string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	hash := hashURL(url)
	st.sources[hash] = &ProcessedSource{
		URL:         url,
		Title:       title,
		ProcessedAt: time.Now(),
		Hash:        hash,
		StoragePath: storagePath,
	}
	st.modified = true
}

// RemoveProcessed removes a URL from the processed list
func (st *SourceTracker) RemoveProcessed(url string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	hash := hashURL(url)
	if _, exists := st.sources[hash]; exists {
		delete(st.sources, hash)
		st.modified = true
	}
}

// GetAllProcessed returns all processed sources
func (st *SourceTracker) GetAllProcessed() []*ProcessedSource {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	var sources []*ProcessedSource
	for _, source := range st.sources {
		sources = append(sources, source)
	}
	return sources
}

// hashURL creates a consistent hash for a URL
func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", h[:16]) // Use first 16 bytes for shorter hash
}