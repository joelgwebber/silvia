package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"silvia/internal/operations"
)

// Server provides HTTP API using the operations layer
type Server struct {
	port     int
	token    string
	ops      *operations.Operations
	server   *http.Server
	mu       sync.RWMutex
	lastPing time.Time
	busy     bool
}

// NewServer creates a new HTTP server using operations
func NewServer(port int, token string, ops *operations.Operations) *Server {
	return &Server{
		port:     port,
		token:    token,
		ops:      ops,
		lastPing: time.Now(),
	}
}

// Start begins listening for HTTP requests
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Add CORS middleware wrapper
	handler := s.corsMiddleware(mux)

	// Register routes
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/ingest", s.handleIngest)

	// Entity operations
	mux.HandleFunc("/api/entities/search", s.handleSearch)
	mux.HandleFunc("/api/entities/", s.handleEntity)
	mux.HandleFunc("/api/entities/merge", s.handleMerge)
	mux.HandleFunc("/api/entities/rename", s.handleRename)

	// Queue operations
	mux.HandleFunc("/api/queue", s.handleQueue)
	mux.HandleFunc("/api/queue/add", s.handleQueueAdd)
	mux.HandleFunc("/api/queue/remove", s.handleQueueRemove)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", s.port),
		Handler: handler,
	}

	log.Printf("API server v2 starting on http://localhost:%d", s.port)
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// corsMiddleware adds CORS headers for browser extension access
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests from browser extensions
		origin := r.Header.Get("Origin")

		// Chrome/Edge extensions have origins like chrome-extension://[id]
		// Firefox extensions have origins like moz-extension://[id]
		if strings.HasPrefix(origin, "chrome-extension://") ||
			strings.HasPrefix(origin, "moz-extension://") ||
			strings.HasPrefix(origin, "edge-extension://") ||
			origin == "null" || origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if origin == "" || origin == "null" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateToken checks the authorization token
func (s *Server) validateToken(r *http.Request) bool {
	if s.token == "" {
		return true // No token required if not set
	}

	auth := r.Header.Get("Authorization")
	expectedAuth := "Bearer " + s.token
	return auth == expectedAuth
}

// handleStatus returns server status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	s.lastPing = time.Now()
	busy := s.busy
	s.mu.Unlock()

	response := map[string]any{
		"status":    "ok",
		"version":   "2.0.0",
		"busy":      busy,
		"timestamp": time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleIngest processes content ingestion via operations
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate token
	if !s.validateToken(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if busy
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		http.Error(w, "Server busy", http.StatusTooManyRequests)
		return
	}
	s.busy = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.busy = false
		s.mu.Unlock()
	}()

	// Parse request
	var req struct {
		URL   string `json:"url"`
		Force bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Process using operations
	ctx := context.Background()
	result, err := s.ops.Source.IngestSource(ctx, req.URL, req.Force)
	if err != nil {
		log.Printf("Ingestion error: %v", err)
		response := map[string]any{
			"success": false,
			"error":   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Return success response
	response := map[string]any{
		"success":            true,
		"url":                result.SourceURL,
		"archived_path":      result.ArchivedPath,
		"extracted_entities": len(result.ExtractedEntities),
		"extracted_links":    len(result.ExtractedLinks),
		"processing_time":    result.ProcessingTime.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSearch handles entity search
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	result, err := s.ops.Search.SearchEntities(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleEntity handles single entity operations
func (s *Server) handleEntity(w http.ResponseWriter, r *http.Request) {
	// Extract entity ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/entities/")
	if path == "" {
		http.Error(w, "Entity ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		// Read entity
		entity, err := s.ops.Entity.ReadEntity(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entity)

	case "PUT":
		// Update entity
		var req struct {
			Title   string `json:"title,omitempty"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// If title not provided, use existing title
		if req.Title == "" {
			existing, err := s.ops.Entity.ReadEntity(path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			req.Title = existing.Title
		}

		entity, err := s.ops.Entity.UpdateEntity(path, req.Title, req.Content)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entity)

	case "DELETE":
		// Delete entity
		if err := s.ops.Entity.DeleteEntity(path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMerge handles entity merge operations
func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Entity1ID string `json:"entity1_id"`
		Entity2ID string `json:"entity2_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	result, err := s.ops.Entity.MergeEntities(ctx, req.Entity1ID, req.Entity2ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleRename handles entity rename operations
func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OldID string `json:"old_id"`
		NewID string `json:"new_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	result, err := s.ops.Entity.RenameEntity(req.OldID, req.NewID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleQueue handles queue status
func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status, err := s.ops.Queue.GetQueue()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleQueueAdd adds items to queue
func (s *Server) handleQueueAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL         string `json:"url"`
		Priority    int    `json:"priority"`
		FromSource  string `json:"from_source"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := s.ops.Queue.AddToQueue(req.URL, req.Priority, req.FromSource, req.Description); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "added",
		"url":    req.URL,
	})
}

// handleQueueRemove removes items from queue
func (s *Server) handleQueueRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "URL parameter required", http.StatusBadRequest)
		return
	}

	if err := s.ops.Queue.RemoveFromQueue(url); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
