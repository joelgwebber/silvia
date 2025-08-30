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
)

// IngestHandler is a function that handles ingestion requests
type IngestHandler func(ctx context.Context, url string, html string, title string, links []LinkInfo, metadata map[string]string, force bool) error

// Server provides HTTP API for browser extension
type Server struct {
	port          int
	token         string
	ingestHandler IngestHandler
	server        *http.Server
	mu            sync.RWMutex
	lastPing      time.Time
	ingesting     bool
}

// NewServer creates a new HTTP server for extension API
func NewServer(port int, token string, handler IngestHandler) *Server {
	return &Server{
		port:          port,
		token:         token,
		ingestHandler: handler,
		lastPing:      time.Now(),
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
	
	s.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", s.port),
		Handler: handler,
	}
	
	log.Printf("Extension API server starting on http://localhost:%d", s.port)
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
		// For development, we'll allow all extension origins
		if strings.HasPrefix(origin, "chrome-extension://") || 
		   strings.HasPrefix(origin, "moz-extension://") ||
		   strings.HasPrefix(origin, "edge-extension://") ||
		   origin == "null" || origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if origin == "" || origin == "null" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

// handleStatus returns server status for connection verification
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	s.mu.Lock()
	s.lastPing = time.Now()
	ingesting := s.ingesting
	s.mu.Unlock()
	
	response := map[string]interface{}{
		"status":    "ok",
		"version":   "1.0.0",
		"ingesting": ingesting,
		"timestamp": time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// IngestRequest represents a content ingestion request from the extension
type IngestRequest struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	HTML        string            `json:"html"`
	Text        string            `json:"text"`
	Links       []LinkInfo        `json:"links"`
	Metadata    map[string]string `json:"metadata"`
	Selection   string            `json:"selection,omitempty"`
	Force       bool              `json:"force,omitempty"`
}

// LinkInfo represents a link extracted by the extension
type LinkInfo struct {
	URL     string `json:"url"`
	Text    string `json:"text"`
	Context string `json:"context"`
}

// handleIngest processes content from the browser extension
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
	
	// Check if already ingesting
	s.mu.Lock()
	if s.ingesting {
		s.mu.Unlock()
		http.Error(w, "Already processing another request", http.StatusTooManyRequests)
		return
	}
	s.ingesting = true
	s.mu.Unlock()
	
	defer func() {
		s.mu.Lock()
		s.ingesting = false
		s.mu.Unlock()
	}()
	
	// Parse request
	var req IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}
	
	// Validate required fields
	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}
	
	// Process the ingestion
	ctx := context.Background()
	
	// Convert LinkInfo to the format expected by the handler
	links := make([]LinkInfo, len(req.Links))
	for i, link := range req.Links {
		links[i] = link
	}
	
	// Call the ingestion handler with force flag
	err := s.ingestHandler(ctx, req.URL, req.HTML, req.Title, links, req.Metadata, req.Force)
	
	if err != nil {
		log.Printf("Ingestion error: %v", err)
		response := map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Return success response
	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully ingested: %s", req.Title),
		"url":     req.URL,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}