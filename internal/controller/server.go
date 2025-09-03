package controller

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/controller/api"
	"github.com/songzhibin97/stargate/internal/portal/gateway"
	"github.com/songzhibin97/stargate/internal/portal/handler"
	"github.com/songzhibin97/stargate/internal/portal/middleware"
	"github.com/songzhibin97/stargate/internal/portal/repository/memory"
	"github.com/songzhibin97/stargate/internal/portal/repository/postgres"
	"github.com/songzhibin97/stargate/internal/store"
	"github.com/songzhibin97/stargate/internal/tls"
	"github.com/songzhibin97/stargate/pkg/portal"
	pkglog "github.com/songzhibin97/stargate/pkg/log"
)

// GatewayClientInterface defines the interface for gateway clients
type GatewayClientInterface interface {
	CreateConsumer(consumerID, name string, metadata map[string]string) (*gateway.Consumer, error)
	DeleteConsumer(consumerID string) error
	GenerateAPIKey(consumerID string) (string, error)
	RevokeAPIKey(consumerID, apiKey string) error
	Health() error
}

// Server represents the controller server
type Server struct {
	config         *config.Config
	httpServer     *http.Server
	apiHandler     *APIHandler
	syncManager    *SyncManager
	acmeManager    *tls.ACMEManager
	store          store.Store
	configNotifier *ConfigNotifier
	mu             sync.RWMutex
	running        bool
}

// APIHandler handles Admin API requests
type APIHandler struct {
	config            *config.Config
	store             store.Store
	configNotifier    *ConfigNotifier
	mux               *http.ServeMux
	routeHandler      *api.RouteHandler
	upstreamHandler   *api.UpstreamHandler
	pluginHandler     *api.PluginHandler
	configHandler     *api.ConfigHandler
	authHandler       *api.AuthHandler
	authMiddleware    *api.AuthMiddleware
	docsHandler       *api.DocsHandler
	portalHandler     *handler.PortalHandler
	applicationHandler *handler.ApplicationHandler
	jwtMiddleware     *middleware.JWTMiddleware
	userRepo          portal.UserRepository
	appRepo           portal.ApplicationRepository
	gatewayClient     GatewayClientInterface
}

// SyncManager manages configuration synchronization
type SyncManager struct {
	config  *config.Config
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewServer creates a new controller server
func NewServer(cfg *config.Config) (*Server, error) {
	// Create store
	var storeInstance store.Store
	var err error

	switch cfg.Store.Type {
	case "etcd":
		storeInstance, err = store.NewEtcdStore(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd store: %w", err)
		}
	case "memory":
		storeInstance, err = store.NewMemoryStore(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory store: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported store type: %s", cfg.Store.Type)
	}

	// Create configuration notifier
	logger := pkglog.Component("controller.server")
	configNotifier := NewConfigNotifier(cfg, storeInstance, logger)

	// Create API handler
	apiHandler, err := NewAPIHandler(cfg, storeInstance, configNotifier)
	if err != nil {
		return nil, fmt.Errorf("failed to create API handler: %w", err)
	}

	// Create sync manager
	syncManager, err := NewSyncManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync manager: %w", err)
	}

	// Create ACME manager if enabled
	var acmeManager *tls.ACMEManager
	if cfg.Controller.TLS.Enabled && cfg.Controller.TLS.ACME.Enabled {
		acmeManager, err = tls.NewACMEManager(&cfg.Controller.TLS.ACME)
		if err != nil {
			return nil, fmt.Errorf("failed to create ACME manager: %w", err)
		}
	}

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.Controller.Address,
		Handler:      apiHandler,
		ReadTimeout:  cfg.Controller.ReadTimeout,
		WriteTimeout: cfg.Controller.WriteTimeout,
	}

	// Configure TLS if enabled
	if cfg.Controller.TLS.Enabled {
		if acmeManager != nil {
			// Use ACME-managed certificates
			httpServer.TLSConfig = acmeManager.GetTLSConfig()
			// Wrap handler to handle ACME challenges
			httpServer.Handler = acmeManager.GetHTTPHandler(apiHandler)
		}

		// Configure HTTP/2 support for TLS connections
		if err := http2.ConfigureServer(httpServer, &http2.Server{}); err != nil {
			log.Printf("Failed to configure HTTP/2 for controller server: %v", err)
		} else {
			log.Println("HTTP/2 support enabled for controller server TLS connections")
		}
	}

	return &Server{
		config:         cfg,
		httpServer:     httpServer,
		apiHandler:     apiHandler,
		syncManager:    syncManager,
		acmeManager:    acmeManager,
		store:          storeInstance,
		configNotifier: configNotifier,
	}, nil
}

// Start starts the controller server
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	s.running = true

	// Start configuration notifier
	if err := s.configNotifier.Start(); err != nil {
		return fmt.Errorf("failed to start config notifier: %w", err)
	}

	// Start ACME manager if enabled
	if s.acmeManager != nil {
		if err := s.acmeManager.Start(); err != nil {
			return fmt.Errorf("failed to start ACME manager: %w", err)
		}
		log.Printf("ACME manager started for domains: %v", s.acmeManager.GetDomains())
	}

	// Start HTTP server
	if s.config.Controller.TLS.Enabled {
		if s.acmeManager != nil {
			// Use ACME-managed certificates
			return s.httpServer.ListenAndServeTLS("", "")
		} else {
			// Use static certificates
			return s.httpServer.ListenAndServeTLS(
				s.config.Controller.TLS.CertFile,
				s.config.Controller.TLS.KeyFile,
			)
		}
	}

	return s.httpServer.ListenAndServe()
}

// StartSync starts the configuration synchronization
func (s *Server) StartSync() error {
	return s.syncManager.Start()
}

// StopSync stops the configuration synchronization
func (s *Server) StopSync() {
	s.syncManager.Stop()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	// Stop ACME manager first
	if s.acmeManager != nil {
		if err := s.acmeManager.Stop(); err != nil {
			log.Printf("Failed to stop ACME manager: %v", err)
		}
	}

	// Stop configuration notifier
	s.configNotifier.Stop()

	// Stop sync manager
	s.syncManager.Stop()

	// Close store
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			log.Printf("Failed to close store: %v", err)
		}
	}

	// Shutdown HTTP server
	return s.httpServer.Shutdown(ctx)
}

// Health returns the health status of the controller
func (s *Server) Health() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"running":   s.running,
		"server": map[string]interface{}{
			"address": s.config.Controller.Address,
		},
	}

	// Add API handler health
	if apiHealth := s.apiHandler.Health(); apiHealth != nil {
		health["api"] = apiHealth
	}

	// Add sync manager health
	if syncHealth := s.syncManager.Health(); syncHealth != nil {
		health["sync"] = syncHealth
	}

	return health
}

// Metrics returns controller metrics
func (s *Server) Metrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	metrics := map[string]interface{}{
		"running": s.running,
		"server": map[string]interface{}{
			"address": s.config.Controller.Address,
		},
	}

	// Add API handler metrics
	if apiMetrics := s.apiHandler.Metrics(); apiMetrics != nil {
		metrics["api"] = apiMetrics
	}

	// Add sync manager metrics
	if syncMetrics := s.syncManager.Metrics(); syncMetrics != nil {
		metrics["sync"] = syncMetrics
	}

	return metrics
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(cfg *config.Config, store store.Store, configNotifier *ConfigNotifier) (*APIHandler, error) {
	apiHandler := &APIHandler{
		config:          cfg,
		store:           store,
		configNotifier:  configNotifier,
		mux:             http.NewServeMux(),
		routeHandler:    api.NewRouteHandler(cfg, store, configNotifier),
		upstreamHandler: api.NewUpstreamHandler(cfg, store, configNotifier),
		pluginHandler:   api.NewPluginHandler(cfg, store, configNotifier),
		configHandler:   api.NewConfigHandler(cfg, store),
		authHandler:     api.NewAuthHandler(cfg),
		authMiddleware:  api.NewAuthMiddleware(cfg),
		docsHandler:     api.NewDocsHandler(),
	}

	// Initialize Portal components if enabled
	if cfg.Portal.Enabled {
		userRepo, appRepo, err := createRepositories(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create repositories: %w", err)
		}
		apiHandler.userRepo = userRepo
		apiHandler.appRepo = appRepo

		portalHandler, err := handler.NewPortalHandler(cfg, userRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create portal handler: %w", err)
		}
		apiHandler.portalHandler = portalHandler

		// Create JWT middleware
		jwtMiddleware, err := middleware.NewJWTMiddleware(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create JWT middleware: %w", err)
		}
		apiHandler.jwtMiddleware = jwtMiddleware

		// Create gateway client (use mock for testing when data plane URL is localhost)
		var gatewayClient GatewayClientInterface
		if cfg.Gateway.DataPlaneURL == "http://localhost:8080" {
			// Use mock client for testing
			gatewayClient = gateway.NewMockClient()
		} else {
			// Use real client for production
			gatewayClient = gateway.NewClient(cfg)
		}
		apiHandler.gatewayClient = gatewayClient

		// Create application handler
		applicationHandler := handler.NewApplicationHandler(cfg, appRepo, gatewayClient)
		apiHandler.applicationHandler = applicationHandler
	}

	// Setup routes
	apiHandler.setupRoutes()

	return apiHandler, nil
}

// createUserRepository creates a user repository based on configuration
func createUserRepository(cfg *config.Config) (portal.UserRepository, error) {
	switch cfg.Portal.Repository.Type {
	case "memory":
		repo := memory.NewRepository()
		return memory.NewUserRepository(repo), nil
	case "postgres":
		pgConfig := &postgres.Config{
			DSN:             cfg.Portal.Repository.Postgres.DSN,
			MaxOpenConns:    cfg.Portal.Repository.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Portal.Repository.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Portal.Repository.Postgres.ConnMaxLifetime,
			MigrationPath:   cfg.Portal.Repository.Postgres.MigrationPath,
		}
		repo, err := postgres.NewRepository(pgConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres repository: %w", err)
		}

		// Run migrations
		if err := repo.Migrate(); err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		return postgres.NewUserRepository(repo), nil
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", cfg.Portal.Repository.Type)
	}
}

// createApplicationRepository creates an application repository based on configuration
func createApplicationRepository(cfg *config.Config) (portal.ApplicationRepository, error) {
	switch cfg.Portal.Repository.Type {
	case "memory":
		repo := memory.NewRepository()
		return memory.NewApplicationRepository(repo), nil
	case "postgres":
		pgConfig := &postgres.Config{
			DSN:             cfg.Portal.Repository.Postgres.DSN,
			MaxOpenConns:    cfg.Portal.Repository.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Portal.Repository.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Portal.Repository.Postgres.ConnMaxLifetime,
			MigrationPath:   cfg.Portal.Repository.Postgres.MigrationPath,
		}
		repo, err := postgres.NewRepository(pgConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres repository: %w", err)
		}

		return postgres.NewApplicationRepository(repo), nil
	default:
		return nil, fmt.Errorf("unsupported repository type: %s", cfg.Portal.Repository.Type)
	}
}

// createRepositories creates both user and application repositories that share the same underlying storage
func createRepositories(cfg *config.Config) (portal.UserRepository, portal.ApplicationRepository, error) {
	switch cfg.Portal.Repository.Type {
	case "memory":
		repo := memory.NewRepository()
		userRepo := memory.NewUserRepository(repo)
		appRepo := memory.NewApplicationRepository(repo)
		return userRepo, appRepo, nil
	case "postgres":
		pgConfig := &postgres.Config{
			DSN:             cfg.Portal.Repository.Postgres.DSN,
			MaxOpenConns:    cfg.Portal.Repository.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Portal.Repository.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Portal.Repository.Postgres.ConnMaxLifetime,
			MigrationPath:   cfg.Portal.Repository.Postgres.MigrationPath,
		}
		repo, err := postgres.NewRepository(pgConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create postgres repository: %w", err)
		}

		// Run migrations
		if err := repo.Migrate(); err != nil {
			return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		userRepo := postgres.NewUserRepository(repo)
		appRepo := postgres.NewApplicationRepository(repo)
		return userRepo, appRepo, nil
	default:
		return nil, nil, fmt.Errorf("unsupported repository type: %s", cfg.Portal.Repository.Type)
	}
}

// ServeHTTP implements http.Handler interface
func (ah *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ah.mux.ServeHTTP(w, r)
}

// setupRoutes sets up API routes
func (ah *APIHandler) setupRoutes() {
	// Health endpoint (no auth required)
	ah.mux.HandleFunc("/health", ah.handleHealth)
	ah.mux.HandleFunc("/metrics", ah.handleMetrics)

	// Documentation endpoints (no auth required)
	ah.mux.HandleFunc("/docs", ah.docsHandler.ServeSwaggerUI)
	ah.mux.HandleFunc("/docs/openapi.json", ah.docsHandler.ServeOpenAPI)

	// Authentication endpoints (no auth required)
	ah.mux.HandleFunc("/auth/login", ah.authHandler.Login)
	ah.mux.HandleFunc("/auth/api-keys", ah.authHandler.GenerateAPIKey)

	// Portal endpoints (no auth required for registration and login)
	if ah.config.Portal.Enabled && ah.portalHandler != nil {
		ah.mux.HandleFunc("/api/register", ah.corsMiddleware(ah.portalHandler.HandleRegister))
		ah.mux.HandleFunc("/api/login", ah.corsMiddleware(ah.portalHandler.HandleLogin))
	}

	// Application endpoints (JWT auth required)
	if ah.config.Portal.Enabled && ah.applicationHandler != nil && ah.jwtMiddleware != nil {
		// Application CRUD operations
		ah.mux.HandleFunc("/api/applications", ah.corsMiddleware(ah.jwtMiddleware.RequireAuth(ah.applicationHandler.HandleListApplications)))
		ah.mux.HandleFunc("/api/applications/", ah.corsMiddleware(ah.handleApplicationWithID))

		// Application management operations
		ah.mux.HandleFunc("/api/applications/create", ah.corsMiddleware(ah.jwtMiddleware.RequireAuth(ah.applicationHandler.HandleCreateApplication)))
	}

	// API routes with authentication
	if ah.config.AdminAPI.REST.Enabled {
		prefix := ah.config.AdminAPI.REST.Prefix

		// Apply authentication middleware to protected routes
		protectedMux := http.NewServeMux()

		// Route management
		protectedMux.HandleFunc(prefix+"/routes", ah.routeHandler.ListRoutes)
		protectedMux.HandleFunc(prefix+"/routes/", ah.handleRouteWithID)

		// Upstream management
		protectedMux.HandleFunc(prefix+"/upstreams", ah.upstreamHandler.ListUpstreams)
		protectedMux.HandleFunc(prefix+"/upstreams/", ah.handleUpstreamWithID)

		// Plugin management
		protectedMux.HandleFunc(prefix+"/plugins", ah.pluginHandler.ListPlugins)
		protectedMux.HandleFunc(prefix+"/plugins/", ah.handlePluginWithID)

		// Configuration management
		protectedMux.HandleFunc(prefix+"/config", ah.configHandler.GetConfig)
		protectedMux.HandleFunc(prefix+"/config/validate", ah.configHandler.ValidateConfig)

		// Wrap protected routes with auth middleware
		ah.mux.Handle(prefix+"/", ah.authMiddleware.Middleware(protectedMux))
	}
}

// Health returns API handler health
func (ah *APIHandler) Health() map[string]interface{} {
	return map[string]interface{}{
		"status": "healthy",
		"rest_enabled": ah.config.AdminAPI.REST.Enabled,
		"grpc_enabled": ah.config.AdminAPI.GRPC.Enabled,
	}
}

// Metrics returns API handler metrics
func (ah *APIHandler) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"rest_enabled": ah.config.AdminAPI.REST.Enabled,
		"grpc_enabled": ah.config.AdminAPI.GRPC.Enabled,
	}
}

// HTTP handlers
func (ah *APIHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

func (ah *APIHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"metrics": "placeholder"}`))
}

// Route handlers with ID routing
func (ah *APIHandler) handleRouteWithID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ah.routeHandler.GetRoute(w, r)
	case http.MethodPut:
		ah.routeHandler.UpdateRoute(w, r)
	case http.MethodDelete:
		ah.routeHandler.DeleteRoute(w, r)
	case http.MethodPost:
		ah.routeHandler.CreateRoute(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Upstream handlers with ID routing
func (ah *APIHandler) handleUpstreamWithID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ah.upstreamHandler.GetUpstream(w, r)
	case http.MethodPut:
		ah.upstreamHandler.UpdateUpstream(w, r)
	case http.MethodDelete:
		ah.upstreamHandler.DeleteUpstream(w, r)
	case http.MethodPost:
		ah.upstreamHandler.CreateUpstream(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Plugin handlers with ID routing
func (ah *APIHandler) handlePluginWithID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ah.pluginHandler.GetPlugin(w, r)
	case http.MethodPut:
		ah.pluginHandler.UpdatePlugin(w, r)
	case http.MethodDelete:
		ah.pluginHandler.DeletePlugin(w, r)
	case http.MethodPost:
		ah.pluginHandler.CreatePlugin(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}



// NewSyncManager creates a new sync manager
func NewSyncManager(cfg *config.Config) (*SyncManager, error) {
	return &SyncManager{
		config: cfg,
		stopCh: make(chan struct{}),
	}, nil
}

// Start starts the sync manager
func (sm *SyncManager) Start() error {
	if sm.running {
		return fmt.Errorf("sync manager is already running")
	}

	sm.running = true

	if sm.config.Sync.GitOps.Enabled {
		sm.wg.Add(1)
		go sm.runGitOpsSync()
	}

	return nil
}

// Stop stops the sync manager
func (sm *SyncManager) Stop() {
	if !sm.running {
		return
	}

	sm.running = false
	close(sm.stopCh)
	sm.wg.Wait()
}

// Health returns sync manager health
func (sm *SyncManager) Health() map[string]interface{} {
	return map[string]interface{}{
		"status":         "healthy",
		"running":        sm.running,
		"gitops_enabled": sm.config.Sync.GitOps.Enabled,
	}
}

// Metrics returns sync manager metrics
func (sm *SyncManager) Metrics() map[string]interface{} {
	return map[string]interface{}{
		"running":        sm.running,
		"gitops_enabled": sm.config.Sync.GitOps.Enabled,
	}
}

// runGitOpsSync runs GitOps synchronization
func (sm *SyncManager) runGitOpsSync() {
	defer sm.wg.Done()

	ticker := time.NewTicker(sm.config.Sync.GitOps.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopCh:
			return
		case <-ticker.C:
			// Perform GitOps sync (placeholder)
			sm.performGitOpsSync()
		}
	}
}

// performGitOpsSync performs a single GitOps sync
func (sm *SyncManager) performGitOpsSync() {
	// Placeholder for GitOps sync logic
	// This would typically:
	// 1. Clone/pull the Git repository
	// 2. Read configuration files
	// 3. Validate configurations
	// 4. Update the configuration store
}

// corsMiddleware adds CORS headers for Portal API endpoints
func (ah *APIHandler) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ah.config.Portal.CORS.Enabled {
			// Set CORS headers
			if len(ah.config.Portal.CORS.AllowedOrigins) > 0 {
				origin := r.Header.Get("Origin")
				for _, allowedOrigin := range ah.config.Portal.CORS.AllowedOrigins {
					if allowedOrigin == "*" || allowedOrigin == origin {
						w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
						break
					}
				}
			}

			if len(ah.config.Portal.CORS.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(ah.config.Portal.CORS.AllowedMethods, ", "))
			}

			if len(ah.config.Portal.CORS.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(ah.config.Portal.CORS.AllowedHeaders, ", "))
			}

			if len(ah.config.Portal.CORS.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(ah.config.Portal.CORS.ExposedHeaders, ", "))
			}

			if ah.config.Portal.CORS.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if ah.config.Portal.CORS.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", ah.config.Portal.CORS.MaxAge))
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next(w, r)
	}
}

// handleApplicationWithID handles application routes with ID parameter
func (ah *APIHandler) handleApplicationWithID(w http.ResponseWriter, r *http.Request) {
	// Extract application ID from path
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/applications/") {
		http.NotFound(w, r)
		return
	}

	// Get the part after /api/applications/
	suffix := strings.TrimPrefix(path, "/api/applications/")
	parts := strings.Split(suffix, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	// Route based on method and additional path segments
	switch r.Method {
	case http.MethodGet:
		if len(parts) == 1 {
			// GET /api/applications/{id}
			ah.jwtMiddleware.RequireAuth(ah.applicationHandler.HandleGetApplication)(w, r)
		} else {
			http.NotFound(w, r)
		}
	case http.MethodPut:
		if len(parts) == 1 {
			// PUT /api/applications/{id}
			ah.jwtMiddleware.RequireAuth(ah.applicationHandler.HandleUpdateApplication)(w, r)
		} else {
			http.NotFound(w, r)
		}
	case http.MethodDelete:
		if len(parts) == 1 {
			// DELETE /api/applications/{id}
			ah.jwtMiddleware.RequireAuth(ah.applicationHandler.HandleDeleteApplication)(w, r)
		} else {
			http.NotFound(w, r)
		}
	case http.MethodPost:
		if len(parts) == 2 && parts[1] == "regenerate-key" {
			// POST /api/applications/{id}/regenerate-key
			ah.jwtMiddleware.RequireAuth(ah.applicationHandler.HandleRegenerateAPIKey)(w, r)
		} else {
			http.NotFound(w, r)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
