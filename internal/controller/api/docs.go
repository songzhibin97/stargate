package api

import (
	"encoding/json"
	"net/http"
)

// OpenAPISpec represents the OpenAPI 3.0 specification for the Admin API
var OpenAPISpec = map[string]interface{}{
	"openapi": "3.0.0",
	"info": map[string]interface{}{
		"title":       "Stargate Admin API",
		"description": "RESTful API for managing Stargate gateway configuration",
		"version":     "1.0.0",
		"contact": map[string]interface{}{
			"name": "Stargate Team",
		},
	},
	"servers": []map[string]interface{}{
		{
			"url":         "http://localhost:9090",
			"description": "Development server",
		},
	},
	"security": []map[string]interface{}{
		{"ApiKeyAuth": []string{}},
		{"BearerAuth": []string{}},
	},
	"components": map[string]interface{}{
		"securitySchemes": map[string]interface{}{
			"ApiKeyAuth": map[string]interface{}{
				"type": "apiKey",
				"in":   "header",
				"name": "X-Admin-Key",
			},
			"BearerAuth": map[string]interface{}{
				"type":   "http",
				"scheme": "bearer",
				"bearerFormat": "JWT",
			},
		},
		"schemas": map[string]interface{}{
			"Route": map[string]interface{}{
				"type": "object",
				"required": []string{"id", "name", "rules", "upstream_id"},
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the route",
						"example":     "route-001",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable name for the route",
						"example":     "API Route",
					},
					"rules": map[string]interface{}{
						"$ref": "#/components/schemas/RouteRules",
					},
					"upstream_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the upstream service",
						"example":     "upstream-001",
					},
					"priority": map[string]interface{}{
						"type":        "integer",
						"description": "Route priority (higher values have higher priority)",
						"example":     100,
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Additional metadata for the route",
						"additionalProperties": map[string]interface{}{
							"type": "string",
						},
					},
					"created_at": map[string]interface{}{
						"type":        "integer",
						"description": "Unix timestamp of creation",
						"example":     1640995200,
					},
					"updated_at": map[string]interface{}{
						"type":        "integer",
						"description": "Unix timestamp of last update",
						"example":     1640995200,
					},
				},
			},
			"RouteRules": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"hosts": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "List of host patterns to match",
						"example":     []string{"api.example.com", "*.api.example.com"},
					},
					"paths": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/components/schemas/PathRule",
						},
						"description": "List of path matching rules",
					},
					"methods": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
						"description": "List of HTTP methods to match",
						"example":     []string{"GET", "POST"},
					},
				},
			},
			"PathRule": map[string]interface{}{
				"type": "object",
				"required": []string{"type", "value"},
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"exact", "prefix", "regex"},
						"description": "Type of path matching",
						"example":     "prefix",
					},
					"value": map[string]interface{}{
						"type":        "string",
						"description": "Path pattern to match",
						"example":     "/api",
					},
				},
			},
			"Upstream": map[string]interface{}{
				"type": "object",
				"required": []string{"id", "name", "targets"},
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the upstream",
						"example":     "upstream-001",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable name for the upstream",
						"example":     "API Backend",
					},
					"targets": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/components/schemas/Target",
						},
						"description": "List of backend targets",
					},
					"algorithm": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"round_robin", "weighted", "ip_hash"},
						"description": "Load balancing algorithm",
						"example":     "round_robin",
					},
				},
			},
			"Target": map[string]interface{}{
				"type": "object",
				"required": []string{"url"},
				"properties": map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "Target URL",
						"example":     "http://backend1:8080",
					},
					"weight": map[string]interface{}{
						"type":        "integer",
						"description": "Weight for load balancing",
						"example":     100,
					},
				},
			},
			"Plugin": map[string]interface{}{
				"type": "object",
				"required": []string{"id", "name", "type"},
				"properties": map[string]interface{}{
					"id": map[string]interface{}{
						"type":        "string",
						"description": "Unique identifier for the plugin",
						"example":     "plugin-001",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Human-readable name for the plugin",
						"example":     "Rate Limiter",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"enum":        []string{"auth", "rate_limit", "cors", "circuit_breaker", "traffic_mirror", "header_transform", "mock_response", "wasm", "custom"},
						"description": "Type of plugin",
						"example":     "rate_limit",
					},
					"enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the plugin is enabled",
						"example":     true,
					},
					"config": map[string]interface{}{
						"type":        "object",
						"description": "Plugin-specific configuration",
						"additionalProperties": true,
					},
				},
			},
			"Error": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"error": map[string]interface{}{
						"type":        "string",
						"description": "Error message",
					},
					"status": map[string]interface{}{
						"type":        "integer",
						"description": "HTTP status code",
					},
					"details": map[string]interface{}{
						"type":        "string",
						"description": "Additional error details",
					},
				},
			},
		},
	},
	"paths": map[string]interface{}{
		"/health": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Health check",
				"description": "Returns the health status of the API",
				"security":    []map[string]interface{}{},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "API is healthy",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"status": map[string]interface{}{
											"type":    "string",
											"example": "healthy",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"/auth/login": map[string]interface{}{
			"post": map[string]interface{}{
				"summary":     "Authenticate user",
				"description": "Authenticate user and return JWT token",
				"security":    []map[string]interface{}{},
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"required": []string{"username", "password"},
								"properties": map[string]interface{}{
									"username": map[string]interface{}{
										"type":    "string",
										"example": "admin",
									},
									"password": map[string]interface{}{
										"type":    "string",
										"example": "password",
									},
								},
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Authentication successful",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"token": map[string]interface{}{
											"type": "string",
										},
										"token_type": map[string]interface{}{
											"type":    "string",
											"example": "Bearer",
										},
										"expires_in": map[string]interface{}{
											"type":    "string",
											"example": "24h",
										},
									},
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Bad request",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Error",
								},
							},
						},
					},
				},
			},
		},
		"/api/v1/routes": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List routes",
				"description": "Get a list of all routes",
				"parameters": []map[string]interface{}{
					{
						"name":        "limit",
						"in":          "query",
						"description": "Maximum number of routes to return",
						"schema": map[string]interface{}{
							"type":    "integer",
							"default": 50,
						},
					},
					{
						"name":        "offset",
						"in":          "query",
						"description": "Number of routes to skip",
						"schema": map[string]interface{}{
							"type":    "integer",
							"default": 0,
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "List of routes",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"routes": map[string]interface{}{
											"type": "array",
											"items": map[string]interface{}{
												"$ref": "#/components/schemas/Route",
											},
										},
										"total": map[string]interface{}{
											"type": "integer",
										},
										"limit": map[string]interface{}{
											"type": "integer",
										},
										"offset": map[string]interface{}{
											"type": "integer",
										},
									},
								},
							},
						},
					},
				},
			},
			"post": map[string]interface{}{
				"summary":     "Create route",
				"description": "Create a new route",
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/Route",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"201": map[string]interface{}{
						"description": "Route created successfully",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"message": map[string]interface{}{
											"type": "string",
										},
										"route": map[string]interface{}{
											"$ref": "#/components/schemas/Route",
										},
									},
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Bad request",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/Error",
								},
							},
						},
					},
				},
			},
		},
	},
}

// DocsHandler handles API documentation requests
type DocsHandler struct{}

// NewDocsHandler creates a new docs handler
func NewDocsHandler() *DocsHandler {
	return &DocsHandler{}
}

// ServeOpenAPI handles GET /docs/openapi.json
func (dh *DocsHandler) ServeOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(OpenAPISpec)
}

// ServeSwaggerUI handles GET /docs
func (dh *DocsHandler) ServeSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	swaggerHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Stargate Admin API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@3.52.5/swagger-ui.css" />
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin:0;
            background: #fafafa;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@3.52.5/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@3.52.5/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            const ui = SwaggerUIBundle({
                url: '/docs/openapi.json',
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(swaggerHTML))
}
