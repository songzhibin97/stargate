package trafficmirror

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/stargate/internal/config"
)

func TestMiddleware_Disabled(t *testing.T) {
	config := &config.TrafficMirrorConfig{
		Enabled: false,
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "success" {
		t.Errorf("Expected body 'success', got '%s'", w.Body.String())
	}
}

func TestMiddleware_BasicMirroring(t *testing.T) {
	// Create a mock mirror target server
	mirrorReceived := make(chan *http.Request, 1)
	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorReceived <- r
		w.WriteHeader(http.StatusOK)
	}))
	defer mirrorServer.Close()

	config := &config.TrafficMirrorConfig{
		Enabled:           true,
		LogMirrorRequests: true,
		Mirrors: []*config.MirrorTargetConfig{
			{
				ID:         "test-mirror",
				Name:       "Test Mirror",
				URL:        mirrorServer.URL,
				SampleRate: 1.0, // Mirror all requests
				Timeout:    5 * time.Second,
				Enabled:    true,
			},
		},
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("original response"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Test-Header", "test-value")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check original response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "original response" {
		t.Errorf("Expected body 'original response', got '%s'", w.Body.String())
	}

	// Wait for mirror request
	select {
	case mirrorReq := <-mirrorReceived:
		if mirrorReq.Method != "GET" {
			t.Errorf("Expected mirror method GET, got %s", mirrorReq.Method)
		}

		if mirrorReq.URL.Path != "/api/test" {
			t.Errorf("Expected mirror path /api/test, got %s", mirrorReq.URL.Path)
		}

		if mirrorReq.Header.Get("X-Test-Header") != "test-value" {
			t.Errorf("Expected mirror header X-Test-Header: test-value, got %s", mirrorReq.Header.Get("X-Test-Header"))
		}

		if mirrorReq.Header.Get("X-Mirror-Source") != "stargate" {
			t.Errorf("Expected X-Mirror-Source header, got %s", mirrorReq.Header.Get("X-Mirror-Source"))
		}

	case <-time.After(2 * time.Second):
		t.Error("Mirror request not received within timeout")
	}
}

func TestMiddleware_SampleRate(t *testing.T) {
	mirrorCount := 0
	var mu sync.Mutex
	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		mirrorCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer mirrorServer.Close()

	config := &config.TrafficMirrorConfig{
		Enabled: true,
		Mirrors: []*config.MirrorTargetConfig{
			{
				ID:         "sample-mirror",
				Name:       "Sample Mirror",
				URL:        mirrorServer.URL,
				SampleRate: 0.5, // Mirror 50% of requests
				Timeout:    5 * time.Second,
				Enabled:    true,
			},
		},
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send multiple requests
	totalRequests := 100
	for i := 0; i < totalRequests; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Wait for all mirror requests to complete
	time.Sleep(1 * time.Second)

	mu.Lock()
	finalMirrorCount := mirrorCount
	mu.Unlock()

	// Check that approximately 50% of requests were mirrored (allow some variance)
	// Note: The simple random generation might not be perfectly distributed
	expectedMin := int(float64(totalRequests) * 0.2) // 20%
	expectedMax := int(float64(totalRequests) * 0.8) // 80%

	if finalMirrorCount < expectedMin || finalMirrorCount > expectedMax {
		t.Errorf("Expected mirror count between %d and %d, got %d", expectedMin, expectedMax, finalMirrorCount)
	}

	t.Logf("Mirrored %d out of %d requests (%.1f%%)", finalMirrorCount, totalRequests, float64(finalMirrorCount)/float64(totalRequests)*100)
}

func TestMiddleware_PostRequestWithBody(t *testing.T) {
	var mirrorBody []byte
	mirrorReceived := make(chan bool, 1)
	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mirrorBody = body
		mirrorReceived <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer mirrorServer.Close()

	config := &config.TrafficMirrorConfig{
		Enabled: true,
		Mirrors: []*config.MirrorTargetConfig{
			{
				ID:         "post-mirror",
				Name:       "POST Mirror",
				URL:        mirrorServer.URL,
				SampleRate: 1.0,
				Timeout:    5 * time.Second,
				Enabled:    true,
			},
		},
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body in main handler
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("received: " + string(body)))
	}))

	requestBody := `{"message": "test data"}`
	req := httptest.NewRequest("POST", "/api/data", strings.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check original response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	expectedResponse := "received: " + requestBody
	if w.Body.String() != expectedResponse {
		t.Errorf("Expected body '%s', got '%s'", expectedResponse, w.Body.String())
	}

	// Wait for mirror request
	select {
	case <-mirrorReceived:
		if string(mirrorBody) != requestBody {
			t.Errorf("Expected mirror body '%s', got '%s'", requestBody, string(mirrorBody))
		}
	case <-time.After(2 * time.Second):
		t.Error("Mirror request not received within timeout")
	}
}

func TestMiddleware_MultipleTargets(t *testing.T) {
	mirror1Received := make(chan bool, 1)
	mirror2Received := make(chan bool, 1)

	mirror1Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirror1Received <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer mirror1Server.Close()

	mirror2Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirror2Received <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer mirror2Server.Close()

	config := &config.TrafficMirrorConfig{
		Enabled: true,
		Mirrors: []*config.MirrorTargetConfig{
			{
				ID:         "mirror1",
				Name:       "Mirror 1",
				URL:        mirror1Server.URL,
				SampleRate: 1.0,
				Timeout:    5 * time.Second,
				Enabled:    true,
			},
			{
				ID:         "mirror2",
				Name:       "Mirror 2",
				URL:        mirror2Server.URL,
				SampleRate: 1.0,
				Timeout:    5 * time.Second,
				Enabled:    true,
			},
		},
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that both mirrors received the request
	select {
	case <-mirror1Received:
		t.Log("Mirror 1 received request")
	case <-time.After(2 * time.Second):
		t.Error("Mirror 1 did not receive request within timeout")
	}

	select {
	case <-mirror2Received:
		t.Log("Mirror 2 received request")
	case <-time.After(2 * time.Second):
		t.Error("Mirror 2 did not receive request within timeout")
	}
}

func TestMiddleware_RouteFiltering(t *testing.T) {
	mirrorReceived := make(chan string, 2)
	mirrorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorReceived <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer mirrorServer.Close()

	config := &config.TrafficMirrorConfig{
		Enabled: true,
		Mirrors: []*config.MirrorTargetConfig{
			{
				ID:         "filtered-mirror",
				Name:       "Filtered Mirror",
				URL:        mirrorServer.URL,
				SampleRate: 1.0,
				Timeout:    5 * time.Second,
				Enabled:    true,
				Metadata: map[string]string{
					"route_filter": "api-route",
				},
			},
		},
	}

	middleware, err := NewMiddleware(config)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}

	handler := middleware.Handler()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Request that should be mirrored (matching route)
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	ctx1 := context.WithValue(req1.Context(), "route_id", "api-route")
	req1 = req1.WithContext(ctx1)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// Request that should NOT be mirrored (non-matching route)
	req2 := httptest.NewRequest("GET", "/other/test", nil)
	ctx2 := context.WithValue(req2.Context(), "route_id", "other-route")
	req2 = req2.WithContext(ctx2)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	// Check that only the first request was mirrored
	select {
	case path := <-mirrorReceived:
		if path != "/api/test" {
			t.Errorf("Expected mirrored path /api/test, got %s", path)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected mirror request not received")
	}

	// Ensure no additional mirror requests
	select {
	case path := <-mirrorReceived:
		t.Errorf("Unexpected mirror request for path %s", path)
	case <-time.After(500 * time.Millisecond):
		// Expected - no additional requests
	}
}
