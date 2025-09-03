package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/songzhibin97/stargate/internal/config"
	"github.com/songzhibin97/stargate/internal/middleware"
)

// TestSimpleMockResponse tests the mock response middleware with a minimal setup
func TestSimpleMockResponse(t *testing.T) {
	// Create simple configuration
	cfg := &config.MockResponseConfig{
		Enabled: true,
		Rules: []config.MockRule{
			{
				ID:      "simple-test",
				Name:    "Simple Test",
				Enabled: true,
				Priority: 100,
				Conditions: config.MockConditions{
					Paths: []config.MockPathMatcher{
						{Type: "exact", Value: "/test"},
					},
				},
				Response: config.MockResponse{
					StatusCode: 200,
					Body:       `{"test": "success"}`,
				},
			},
		},
	}

	// Create middleware
	middleware, err := middleware.NewMockResponseMiddleware(cfg)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	// Create test handler that should not be called
	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	})

	// Create middleware chain
	handler := middleware.Handler()(testHandler)

	// Create request
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	// Execute middleware
	handler.ServeHTTP(rr, req)

	// Verify mock response was served
	if handlerCalled {
		t.Error("Handler should not have been called when mock is served")
	}

	if rr.Code != 200 {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if body := rr.Body.String(); body != `{"test": "success"}` {
		t.Errorf("Expected body %s, got %s", `{"test": "success"}`, body)
	}
}

// TestMockResponseDisabled tests that middleware is bypassed when disabled
func TestMockResponseDisabled(t *testing.T) {
	cfg := &config.MockResponseConfig{
		Enabled: false, // Disabled
		Rules: []config.MockRule{
			{
				ID:      "disabled-rule",
				Name:    "Disabled Rule",
				Enabled: true,
				Priority: 100,
				Conditions: config.MockConditions{
					Paths: []config.MockPathMatcher{
						{Type: "exact", Value: "/test"},
					},
				},
				Response: config.MockResponse{
					StatusCode: 200,
					Body:       `{"should": "not_appear"}`,
				},
			},
		},
	}

	middleware, err := middleware.NewMockResponseMiddleware(cfg)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	})

	handler := middleware.Handler()(testHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler should have been called when middleware is disabled")
	}

	if rr.Body.String() != "original response" {
		t.Error("Expected original response when middleware is disabled")
	}
}

// TestMockResponseNoMatch tests that original handler is called when no rules match
func TestMockResponseNoMatch(t *testing.T) {
	cfg := &config.MockResponseConfig{
		Enabled: true,
		Rules: []config.MockRule{
			{
				ID:      "no-match-rule",
				Name:    "No Match Rule",
				Enabled: true,
				Priority: 100,
				Conditions: config.MockConditions{
					Paths: []config.MockPathMatcher{
						{Type: "exact", Value: "/other"},
					},
				},
				Response: config.MockResponse{
					StatusCode: 200,
					Body:       `{"should": "not_appear"}`,
				},
			},
		},
	}

	middleware, err := middleware.NewMockResponseMiddleware(cfg)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	var handlerCalled bool
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	})

	handler := middleware.Handler()(testHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Handler should have been called when no rules match")
	}

	if rr.Body.String() != "original response" {
		t.Error("Expected original response when no rules match")
	}
}
