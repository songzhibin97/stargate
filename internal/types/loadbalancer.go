package types

// Target represents a backend target
type Target struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Weight  int    `json:"weight"`
	Healthy bool   `json:"healthy"`
}

// Upstream represents an upstream service
type Upstream struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Algorithm   string            `json:"algorithm"`
	Targets     []*Target         `json:"targets"`
	HealthCheck *HealthCheck      `json:"health_check"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

// HealthCheck represents health check configuration
type HealthCheck struct {
	Type               string `json:"type"`
	Path               string `json:"path"`
	Interval           int    `json:"interval"`
	Timeout            int    `json:"timeout"`
	HealthyThreshold   int    `json:"healthy_threshold"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
}

// LoadBalancer interface for load balancing
type LoadBalancer interface {
	Select(upstream *Upstream) (*Target, error)
	UpdateUpstream(upstream *Upstream) error
	RemoveUpstream(id string) error
	Health() map[string]interface{}
}
