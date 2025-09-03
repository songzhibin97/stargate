package portal

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/router"
	"github.com/songzhibin97/stargate/internal/store"
)

// DocFetcher handles fetching and caching of OpenAPI specifications
type DocFetcher struct {
	config     *config.Config
	store      store.Store
	httpClient *http.Client
	mu         sync.RWMutex
	cache      map[string]*CachedSpec
	running    bool
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// CachedSpec represents a cached OpenAPI specification
type CachedSpec struct {
	RouteID     string                 `json:"route_id"`
	URL         string                 `json:"url"`
	Content     map[string]interface{} `json:"content"`
	ETag        string                 `json:"etag"`
	Checksum    string                 `json:"checksum"`
	LastFetched int64                  `json:"last_fetched"`
	LastError   string                 `json:"last_error,omitempty"`
	FetchCount  int                    `json:"fetch_count"`
}

// ParsedAPIInfo represents parsed API information from OpenAPI spec
type ParsedAPIInfo struct {
	RouteID     string            `json:"route_id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Contact     *ContactInfo      `json:"contact,omitempty"`
	License     *LicenseInfo      `json:"license,omitempty"`
	Tags        []APITag          `json:"tags"`
	Paths       []APIPath         `json:"paths"`
	Servers     []APIServer       `json:"servers"`
	Metadata    map[string]string `json:"metadata"`
}

// APITag represents an API tag
type APITag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// APIPath represents an API path
type APIPath struct {
	Path        string      `json:"path"`
	Method      string      `json:"method"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags"`
	Parameters  []Parameter `json:"parameters"`
	Responses   []Response  `json:"responses"`
}

// Parameter represents an API parameter
type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // query, header, path, cookie
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Example     string `json:"example,omitempty"`
}

// Response represents an API response
type Response struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	ContentType string `json:"content_type,omitempty"`
	Example     string `json:"example,omitempty"`
}

// APIServer represents an API server
type APIServer struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// ContactInfo represents contact information
type ContactInfo struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// LicenseInfo represents license information
type LicenseInfo struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

// NewDocFetcher creates a new document fetcher
func NewDocFetcher(cfg *config.Config, store store.Store) *DocFetcher {
	return &DocFetcher{
		config: cfg,
		store:  store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:  make(map[string]*CachedSpec),
		stopCh: make(chan struct{}),
	}
}

// Start starts the document fetcher
func (df *DocFetcher) Start() error {
	df.mu.Lock()
	defer df.mu.Unlock()

	if df.running {
		return fmt.Errorf("doc fetcher is already running")
	}

	df.running = true

	// Load cached specs from store
	if err := df.loadCachedSpecs(); err != nil {
		log.Printf("Failed to load cached specs: %v", err)
	}

	// Start periodic fetching
	df.wg.Add(1)
	go df.periodicFetch()

	log.Println("Document fetcher started")
	return nil
}

// Stop stops the document fetcher
func (df *DocFetcher) Stop() {
	df.mu.Lock()
	defer df.mu.Unlock()

	if !df.running {
		return
	}

	df.running = false
	close(df.stopCh)
	df.wg.Wait()

	log.Println("Document fetcher stopped")
}

// FetchSpec fetches an OpenAPI specification from a URL
func (df *DocFetcher) FetchSpec(routeID, url string) (*CachedSpec, error) {
	df.mu.Lock()
	defer df.mu.Unlock()

	// Check if we have a cached version
	if cached, exists := df.cache[routeID]; exists {
		// Check if we should refetch (e.g., if it's been more than 1 hour)
		if time.Now().Unix()-cached.LastFetched < 3600 {
			return cached, nil
		}
	}

	// Fetch the specification
	spec, err := df.fetchFromURL(url)
	if err != nil {
		// Update error in cache
		if cached, exists := df.cache[routeID]; exists {
			cached.LastError = err.Error()
			cached.LastFetched = time.Now().Unix()
		}
		return nil, fmt.Errorf("failed to fetch spec from %s: %w", url, err)
	}

	// Calculate checksum
	checksum := df.calculateChecksum(spec)

	// Create cached spec
	cachedSpec := &CachedSpec{
		RouteID:     routeID,
		URL:         url,
		Content:     spec,
		Checksum:    checksum,
		LastFetched: time.Now().Unix(),
		FetchCount:  1,
	}

	// Update existing cache entry
	if existing, exists := df.cache[routeID]; exists {
		cachedSpec.FetchCount = existing.FetchCount + 1
		// Only update if content changed
		if existing.Checksum == checksum {
			existing.LastFetched = time.Now().Unix()
			existing.FetchCount++
			return existing, nil
		}
	}

	// Store in cache
	df.cache[routeID] = cachedSpec

	// Persist to store
	if err := df.storeCachedSpec(cachedSpec); err != nil {
		log.Printf("Failed to store cached spec: %v", err)
	}

	log.Printf("Fetched OpenAPI spec for route %s from %s", routeID, url)
	return cachedSpec, nil
}

// fetchFromURL fetches content from a URL
func (df *DocFetcher) fetchFromURL(url string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json, application/yaml, text/yaml")
	req.Header.Set("User-Agent", "Stargate-Portal/1.0")

	resp, err := df.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Try to parse as JSON first
	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		// If JSON parsing fails, try YAML
		// Note: In a real implementation, you'd use a YAML parser here
		return nil, fmt.Errorf("failed to parse spec as JSON or YAML: %w", err)
	}

	return spec, nil
}

// ParseSpec parses a cached OpenAPI specification into structured API info
func (df *DocFetcher) ParseSpec(cached *CachedSpec) (*ParsedAPIInfo, error) {
	spec := cached.Content

	info := &ParsedAPIInfo{
		RouteID:  cached.RouteID,
		Metadata: make(map[string]string),
	}

	// Parse basic info
	if infoObj, ok := spec["info"].(map[string]interface{}); ok {
		if title, ok := infoObj["title"].(string); ok {
			info.Title = title
		}
		if desc, ok := infoObj["description"].(string); ok {
			info.Description = desc
		}
		if version, ok := infoObj["version"].(string); ok {
			info.Version = version
		}

		// Parse contact info
		if contactObj, ok := infoObj["contact"].(map[string]interface{}); ok {
			contact := &ContactInfo{}
			if name, ok := contactObj["name"].(string); ok {
				contact.Name = name
			}
			if email, ok := contactObj["email"].(string); ok {
				contact.Email = email
			}
			if url, ok := contactObj["url"].(string); ok {
				contact.URL = url
			}
			info.Contact = contact
		}

		// Parse license info
		if licenseObj, ok := infoObj["license"].(map[string]interface{}); ok {
			license := &LicenseInfo{}
			if name, ok := licenseObj["name"].(string); ok {
				license.Name = name
			}
			if url, ok := licenseObj["url"].(string); ok {
				license.URL = url
			}
			info.License = license
		}
	}

	// Parse tags
	if tagsArray, ok := spec["tags"].([]interface{}); ok {
		for _, tagItem := range tagsArray {
			if tagObj, ok := tagItem.(map[string]interface{}); ok {
				tag := APITag{}
				if name, ok := tagObj["name"].(string); ok {
					tag.Name = name
				}
				if desc, ok := tagObj["description"].(string); ok {
					tag.Description = desc
				}
				info.Tags = append(info.Tags, tag)
			}
		}
	}

	// Parse servers
	if serversArray, ok := spec["servers"].([]interface{}); ok {
		for _, serverItem := range serversArray {
			if serverObj, ok := serverItem.(map[string]interface{}); ok {
				server := APIServer{}
				if url, ok := serverObj["url"].(string); ok {
					server.URL = url
				}
				if desc, ok := serverObj["description"].(string); ok {
					server.Description = desc
				}
				info.Servers = append(info.Servers, server)
			}
		}
	}

	// Parse paths (simplified - in a real implementation, this would be much more comprehensive)
	if pathsObj, ok := spec["paths"].(map[string]interface{}); ok {
		for path, pathItem := range pathsObj {
			if pathItemObj, ok := pathItem.(map[string]interface{}); ok {
				for method, operation := range pathItemObj {
					if operationObj, ok := operation.(map[string]interface{}); ok {
						apiPath := APIPath{
							Path:   path,
							Method: method,
						}
						if summary, ok := operationObj["summary"].(string); ok {
							apiPath.Summary = summary
						}
						if desc, ok := operationObj["description"].(string); ok {
							apiPath.Description = desc
						}
						info.Paths = append(info.Paths, apiPath)
					}
				}
			}
		}
	}

	return info, nil
}

// calculateChecksum calculates MD5 checksum of the spec content
func (df *DocFetcher) calculateChecksum(spec map[string]interface{}) string {
	data, _ := json.Marshal(spec)
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash)
}

// periodicFetch runs periodic fetching of all registered specs
func (df *DocFetcher) periodicFetch() {
	defer df.wg.Done()

	ticker := time.NewTicker(1 * time.Hour) // Fetch every hour
	defer ticker.Stop()

	for {
		select {
		case <-df.stopCh:
			return
		case <-ticker.C:
			df.fetchAllSpecs()
		}
	}
}

// fetchAllSpecs fetches all registered OpenAPI specs
func (df *DocFetcher) fetchAllSpecs() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Get all routes with OpenAPI specs
	routesData, err := df.store.List(ctx, "routes/")
	if err != nil {
		log.Printf("Failed to list routes: %v", err)
		return
	}

	for _, data := range routesData {
		var route router.RouteRule
		if err := json.Unmarshal(data, &route); err != nil {
			continue
		}

		if route.OpenAPISpec != nil && route.OpenAPISpec.URL != "" {
			if _, err := df.FetchSpec(route.ID, route.OpenAPISpec.URL); err != nil {
				log.Printf("Failed to fetch spec for route %s: %v", route.ID, err)
			}
		}
	}
}

// loadCachedSpecs loads cached specs from store
func (df *DocFetcher) loadCachedSpecs() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	specsData, err := df.store.List(ctx, "portal/specs/")
	if err != nil {
		return err
	}

	for _, data := range specsData {
		var spec CachedSpec
		if err := json.Unmarshal(data, &spec); err != nil {
			continue
		}
		df.cache[spec.RouteID] = &spec
	}

	log.Printf("Loaded %d cached specs", len(df.cache))
	return nil
}

// storeCachedSpec stores a cached spec to the store
func (df *DocFetcher) storeCachedSpec(spec *CachedSpec) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("portal/specs/%s", spec.RouteID)
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	return df.store.Put(ctx, key, data)
}

// GetCachedSpec returns a cached spec by route ID
func (df *DocFetcher) GetCachedSpec(routeID string) (*CachedSpec, bool) {
	df.mu.RLock()
	defer df.mu.RUnlock()

	spec, exists := df.cache[routeID]
	return spec, exists
}

// ListCachedSpecs returns all cached specs
func (df *DocFetcher) ListCachedSpecs() []*CachedSpec {
	df.mu.RLock()
	defer df.mu.RUnlock()

	specs := make([]*CachedSpec, 0, len(df.cache))
	for _, spec := range df.cache {
		specs = append(specs, spec)
	}

	return specs
}

// Health returns the health status of the doc fetcher
func (df *DocFetcher) Health() map[string]interface{} {
	df.mu.RLock()
	defer df.mu.RUnlock()

	return map[string]interface{}{
		"running":      df.running,
		"cached_specs": len(df.cache),
	}
}
