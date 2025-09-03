package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"github.com/songzhibin97/stargate/internal/config"
)

// ACMEManager manages ACME certificates
type ACMEManager struct {
	config   *config.ACMEConfig
	manager  *autocert.Manager
	mu       sync.RWMutex
	started  bool
	stopChan chan struct{}
}

// NewACMEManager creates a new ACME certificate manager
func NewACMEManager(cfg *config.ACMEConfig) (*ACMEManager, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, fmt.Errorf("ACME configuration is disabled or nil")
	}

	// Validate required fields
	if len(cfg.Domains) == 0 {
		return nil, fmt.Errorf("ACME domains list cannot be empty")
	}

	if cfg.Email == "" {
		return nil, fmt.Errorf("ACME email is required")
	}

	if !cfg.AcceptTOS {
		return nil, fmt.Errorf("ACME Terms of Service must be accepted")
	}

	// Set default cache directory if not specified
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = "./acme-cache"
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create ACME cache directory: %w", err)
	}

	// Create autocert manager
	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(cfg.Domains...),
		Email:      cfg.Email,
	}

	// Note: Custom directory URL configuration would require additional setup
	// For staging environment, this would need to be configured differently
	// in newer versions of autocert

	return &ACMEManager{
		config:   cfg,
		manager:  manager,
		stopChan: make(chan struct{}),
	}, nil
}

// Start starts the ACME manager
func (am *ACMEManager) Start() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.started {
		return fmt.Errorf("ACME manager is already started")
	}

	am.started = true
	log.Printf("ACME manager started for domains: %v", am.config.Domains)

	// Start certificate renewal monitoring
	go am.renewalMonitor()

	return nil
}

// Stop stops the ACME manager
func (am *ACMEManager) Stop() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.started {
		return nil
	}

	close(am.stopChan)
	am.started = false
	log.Println("ACME manager stopped")

	return nil
}

// GetTLSConfig returns a TLS configuration with ACME certificate management
func (am *ACMEManager) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: am.manager.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
		MinVersion:     tls.VersionTLS12,
	}
}

// GetHTTPHandler returns an HTTP handler for ACME challenges
func (am *ACMEManager) GetHTTPHandler(next http.Handler) http.Handler {
	return am.manager.HTTPHandler(next)
}

// renewalMonitor monitors certificate expiration and triggers renewal
func (am *ACMEManager) renewalMonitor() {
	ticker := time.NewTicker(24 * time.Hour) // Check daily
	defer ticker.Stop()

	for {
		select {
		case <-am.stopChan:
			return
		case <-ticker.C:
			am.checkAndRenewCertificates()
		}
	}
}

// checkAndRenewCertificates checks certificate expiration and renews if needed
func (am *ACMEManager) checkAndRenewCertificates() {
	for _, domain := range am.config.Domains {
		if err := am.checkCertificate(domain); err != nil {
			log.Printf("Certificate check failed for domain %s: %v", domain, err)
		}
	}
}

// checkCertificate checks if a certificate needs renewal
func (am *ACMEManager) checkCertificate(domain string) error {
	// Get certificate from manager
	cert, err := am.manager.GetCertificate(&tls.ClientHelloInfo{
		ServerName: domain,
	})
	if err != nil {
		log.Printf("Failed to get certificate for domain %s: %v", domain, err)
		return err
	}

	// Check expiration
	if len(cert.Certificate) > 0 {
		// Parse the certificate to check expiration
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}

		timeUntilExpiry := time.Until(x509Cert.NotAfter)
		if timeUntilExpiry < 30*24*time.Hour { // Renew if expires within 30 days
			log.Printf("Certificate for domain %s expires in %v, triggering renewal", domain, timeUntilExpiry)

			// Trigger renewal by requesting a new certificate
			_, err := am.manager.GetCertificate(&tls.ClientHelloInfo{
				ServerName: domain,
			})
			if err != nil {
				return fmt.Errorf("failed to renew certificate: %w", err)
			}

			log.Printf("Certificate renewed successfully for domain %s", domain)
		}
	}

	return nil
}

// GetCacheDir returns the ACME cache directory
func (am *ACMEManager) GetCacheDir() string {
	return am.config.CacheDir
}

// GetDomains returns the configured domains
func (am *ACMEManager) GetDomains() []string {
	return am.config.Domains
}

// IsEnabled returns whether ACME is enabled
func (am *ACMEManager) IsEnabled() bool {
	return am.config.Enabled
}

// Metrics returns ACME manager metrics
func (am *ACMEManager) Metrics() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	metrics := map[string]interface{}{
		"enabled":    am.config.Enabled,
		"started":    am.started,
		"domains":    len(am.config.Domains),
		"cache_dir":  am.config.CacheDir,
		"email":      am.config.Email,
	}

	// Add certificate status for each domain
	domainStatus := make(map[string]interface{})
	for _, domain := range am.config.Domains {
		status := am.getCertificateStatus(domain)
		domainStatus[domain] = status
	}
	metrics["domain_status"] = domainStatus

	return metrics
}

// getCertificateStatus returns the status of a certificate for a domain
func (am *ACMEManager) getCertificateStatus(domain string) map[string]interface{} {
	status := map[string]interface{}{
		"domain": domain,
		"valid":  false,
	}

	// Try to get certificate
	cert, err := am.manager.GetCertificate(&tls.ClientHelloInfo{
		ServerName: domain,
	})
	if err != nil {
		status["error"] = err.Error()
		return status
	}

	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err == nil {
			status["valid"] = true
			status["not_before"] = x509Cert.NotBefore
			status["not_after"] = x509Cert.NotAfter
			status["expires_in_days"] = int(time.Until(x509Cert.NotAfter).Hours() / 24)
		}
	}

	return status
}

// ValidateConfig validates ACME configuration
func ValidateConfig(cfg *config.ACMEConfig) error {
	if cfg == nil {
		return fmt.Errorf("ACME configuration is nil")
	}

	if !cfg.Enabled {
		return nil // No validation needed if disabled
	}

	if len(cfg.Domains) == 0 {
		return fmt.Errorf("ACME domains list cannot be empty")
	}

	if cfg.Email == "" {
		return fmt.Errorf("ACME email is required")
	}

	if !cfg.AcceptTOS {
		return fmt.Errorf("ACME Terms of Service must be accepted")
	}

	// Validate cache directory
	if cfg.CacheDir != "" {
		if !filepath.IsAbs(cfg.CacheDir) {
			return fmt.Errorf("ACME cache directory must be an absolute path")
		}
	}

	return nil
}
