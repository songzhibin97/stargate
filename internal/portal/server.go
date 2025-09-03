package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/store"
)

// Server represents the developer portal server
type Server struct {
	config      *config.Config
	store       store.Store
	httpServer  *http.Server
	docFetcher  *DocFetcher
	mux         *http.ServeMux
	mu          sync.RWMutex
	running     bool
}

// NewServer creates a new portal server
func NewServer(cfg *config.Config, store store.Store) (*Server, error) {
	// Create document fetcher
	docFetcher := NewDocFetcher(cfg, store)

	// Create HTTP server
	mux := http.NewServeMux()
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Portal.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	server := &Server{
		config:     cfg,
		store:      store,
		httpServer: httpServer,
		docFetcher: docFetcher,
		mux:        mux,
	}

	// Setup routes
	server.setupRoutes()

	return server, nil
}

// Start starts the portal server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("portal server is already running")
	}

	// Start document fetcher
	if err := s.docFetcher.Start(); err != nil {
		return fmt.Errorf("failed to start document fetcher: %w", err)
	}

	s.running = true

	// Start HTTP server
	go func() {
		log.Printf("Portal server starting on port %d", s.config.Portal.Port)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Portal server error: %v", err)
		}
	}()

	log.Println("Portal server started")
	return nil
}

// Stop stops the portal server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	// Stop document fetcher
	s.docFetcher.Stop()

	// Stop HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Portal server shutdown error: %v", err)
		return err
	}

	log.Println("Portal server stopped")
	return nil
}

// setupRoutes sets up HTTP routes
func (s *Server) setupRoutes() {
	// API routes
	s.mux.HandleFunc("/api/v1/health", s.corsMiddleware(s.handleHealth))
	s.mux.HandleFunc("/api/v1/auth/login", s.corsMiddleware(s.handleLogin))
	s.mux.HandleFunc("/api/v1/auth/logout", s.corsMiddleware(s.handleLogout))
	s.mux.HandleFunc("/api/v1/auth/refresh", s.corsMiddleware(s.handleRefreshToken))

	// Portal API routes (require authentication)
	s.mux.HandleFunc("/api/v1/portal/apis", s.corsMiddleware(s.authMiddleware(s.handleGetAPIs)))
	s.mux.HandleFunc("/api/v1/portal/apis/", s.corsMiddleware(s.authMiddleware(s.handleGetAPIDetail)))
	s.mux.HandleFunc("/api/v1/portal/test", s.corsMiddleware(s.authMiddleware(s.handleTestAPI)))
	s.mux.HandleFunc("/api/v1/portal/dashboard", s.corsMiddleware(s.authMiddleware(s.handleDashboard)))
	s.mux.HandleFunc("/api/v1/portal/search", s.corsMiddleware(s.authMiddleware(s.handleSearch)))

	// Static files (React app)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/developer-portal/dist/static/"))))

	// Default handler for root and other paths (must be last)
	s.mux.HandleFunc("/", s.corsMiddleware(s.handleIndex))
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// authMiddleware validates JWT tokens
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Validate token (simplified - in production, use proper JWT validation)
		token := authHeader
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		// TODO: Implement proper JWT validation
		if token == "" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user context to request
		ctx := context.WithValue(r.Context(), "user", map[string]interface{}{
			"id":    "user123",
			"email": "developer@example.com",
			"role":  "developer",
		})
		r = r.WithContext(ctx)

		next(w, r)
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().Unix(),
		"version":     "1.0.0",
		"doc_fetcher": s.docFetcher.Health(),
	}

	s.writeJSON(w, health)
}

// handleLogin handles user login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement proper authentication
	// For now, return a mock token
	response := map[string]interface{}{
		"token":        "mock-jwt-token-12345",
		"refresh_token": "mock-refresh-token-67890",
		"expires_in":   3600,
		"user": map[string]interface{}{
			"id":    "user123",
			"email": "developer@example.com",
			"name":  "Developer User",
			"role":  "developer",
		},
	}

	s.writeJSON(w, response)
}

// handleLogout handles user logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement token invalidation
	response := map[string]interface{}{
		"message": "Logged out successfully",
	}

	s.writeJSON(w, response)
}

// handleRefreshToken handles token refresh
func (s *Server) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement token refresh logic
	response := map[string]interface{}{
		"token":      "new-mock-jwt-token-12345",
		"expires_in": 3600,
	}

	s.writeJSON(w, response)
}

// handleGetAPIs handles API list requests
func (s *Server) handleGetAPIs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all cached specs
	specs := s.docFetcher.ListCachedSpecs()
	
	apis := make([]map[string]interface{}, 0, len(specs))
	for _, spec := range specs {
		if parsed, err := s.docFetcher.ParseSpec(spec); err == nil {
			apis = append(apis, map[string]interface{}{
				"id":          parsed.RouteID,
				"title":       parsed.Title,
				"description": parsed.Description,
				"version":     parsed.Version,
				"tags":        parsed.Tags,
				"servers":     parsed.Servers,
				"paths_count": len(parsed.Paths),
				"last_updated": spec.LastFetched,
			})
		}
	}

	response := map[string]interface{}{
		"apis":  apis,
		"total": len(apis),
	}

	s.writeJSON(w, response)
}

// handleGetAPIDetail handles API detail requests
func (s *Server) handleGetAPIDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract API ID from URL path
	apiID := r.URL.Path[len("/api/v1/portal/apis/"):]
	if apiID == "" {
		http.Error(w, "API ID required", http.StatusBadRequest)
		return
	}

	// Get cached spec
	spec, exists := s.docFetcher.GetCachedSpec(apiID)
	if !exists {
		http.Error(w, "API not found", http.StatusNotFound)
		return
	}

	// Parse spec
	parsed, err := s.docFetcher.ParseSpec(spec)
	if err != nil {
		http.Error(w, "Failed to parse API spec", http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, parsed)
}

// handleTestAPI handles API testing requests
func (s *Server) handleTestAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement API testing proxy
	response := map[string]interface{}{
		"status":   200,
		"data":     map[string]interface{}{"message": "Test response"},
		"duration": 123,
		"size":     456,
	}

	s.writeJSON(w, response)
}

// handleDashboard handles dashboard data requests
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	specs := s.docFetcher.ListCachedSpecs()
	totalEndpoints := 0
	
	for _, spec := range specs {
		if parsed, err := s.docFetcher.ParseSpec(spec); err == nil {
			totalEndpoints += len(parsed.Paths)
		}
	}

	response := map[string]interface{}{
		"total_apis":      len(specs),
		"total_endpoints": totalEndpoints,
		"recent_tests":    42, // Mock data
		"uptime":          "99.9%",
	}

	s.writeJSON(w, response)
}

// handleSearch handles API search requests
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Search query required", http.StatusBadRequest)
		return
	}

	// TODO: Implement search functionality
	response := map[string]interface{}{
		"results": []interface{}{},
		"total":   0,
		"query":   query,
	}

	s.writeJSON(w, response)
}

// handleIndex serves the React app
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Serve index.html for all non-API routes
	if r.URL.Path != "/" && !isAPIRoute(r.URL.Path) {
		http.ServeFile(w, r, "web/developer-portal/dist/index.html")
		return
	}
	
	http.ServeFile(w, r, "web/developer-portal/dist/index.html")
}

// handleNotFound handles 404 errors
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if isAPIRoute(r.URL.Path) {
		http.Error(w, "API endpoint not found", http.StatusNotFound)
		return
	}
	
	// Serve React app for non-API routes
	http.ServeFile(w, r, "web/developer-portal/dist/index.html")
}

// writeJSON writes JSON response
func (s *Server) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// isAPIRoute checks if the path is an API route
func isAPIRoute(path string) bool {
	return len(path) >= 4 && path[:4] == "/api"
}

// Health returns the server health status
func (s *Server) Health() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":      s.running,
		"doc_fetcher":  s.docFetcher.Health(),
	}
}
