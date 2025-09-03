# Stargate API Documentation

## Overview

Stargate provides comprehensive RESTful APIs for managing gateway configuration, routes, upstreams, plugins, and developer portal functionality. This documentation covers all available endpoints, authentication methods, and usage examples.

## Base URLs

- **Admin API**: `http://localhost:9090`
- **Portal API**: `http://localhost:8080`

## Authentication

Stargate supports multiple authentication methods:

### 1. API Key Authentication
```http
X-Admin-Key: your-api-key-here
```

### 2. JWT Bearer Token
```http
Authorization: Bearer your-jwt-token-here
```

### 3. Basic Authentication (for login)
Used only for the `/auth/login` endpoint.

## API Endpoints

### Health & Status

#### GET /health
Check API health status.

**Response:**
```json
{
  "status": "healthy",
  "rest_enabled": true,
  "grpc_enabled": false
}
```

#### GET /metrics
Get Prometheus metrics (if enabled).

### Authentication

#### POST /auth/login
Authenticate and receive JWT token.

**Request:**
```json
{
  "username": "admin",
  "password": "password"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": "24h"
}
```

#### POST /auth/api-keys
Generate a new API key.

**Request:**
```json
{
  "name": "my-api-key"
}
```

**Response:**
```json
{
  "api_key": "sk-1234567890abcdef",
  "name": "my-api-key",
  "created": 1640995200
}
```

### Route Management

#### GET /api/v1/routes
List all routes with pagination.

**Query Parameters:**
- `limit` (integer): Maximum number of routes to return (default: 50)
- `offset` (integer): Number of routes to skip (default: 0)

**Response:**
```json
{
  "routes": [
    {
      "id": "route-001",
      "name": "API Route",
      "rules": {
        "hosts": ["api.example.com"],
        "paths": [
          {
            "type": "prefix",
            "value": "/api"
          }
        ],
        "methods": ["GET", "POST"]
      },
      "upstream_id": "upstream-001",
      "priority": 100,
      "metadata": {
        "environment": "production"
      },
      "created_at": 1640995200,
      "updated_at": 1640995200
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

#### POST /api/v1/routes
Create a new route.

**Request:**
```json
{
  "name": "New API Route",
  "rules": {
    "hosts": ["api.example.com"],
    "paths": [
      {
        "type": "prefix",
        "value": "/api/v2"
      }
    ],
    "methods": ["GET", "POST", "PUT", "DELETE"]
  },
  "upstream_id": "upstream-001",
  "priority": 100,
  "metadata": {
    "environment": "production",
    "team": "backend"
  }
}
```

**Response:**
```json
{
  "message": "Route created successfully",
  "route": {
    "id": "route-002",
    "name": "New API Route",
    "rules": {
      "hosts": ["api.example.com"],
      "paths": [
        {
          "type": "prefix",
          "value": "/api/v2"
        }
      ],
      "methods": ["GET", "POST", "PUT", "DELETE"]
    },
    "upstream_id": "upstream-001",
    "priority": 100,
    "metadata": {
      "environment": "production",
      "team": "backend"
    },
    "created_at": 1640995200,
    "updated_at": 1640995200
  }
}
```

#### GET /api/v1/routes/{id}
Get a specific route by ID.

**Response:**
```json
{
  "id": "route-001",
  "name": "API Route",
  "rules": {
    "hosts": ["api.example.com"],
    "paths": [
      {
        "type": "prefix",
        "value": "/api"
      }
    ],
    "methods": ["GET", "POST"]
  },
  "upstream_id": "upstream-001",
  "priority": 100,
  "metadata": {
    "environment": "production"
  },
  "created_at": 1640995200,
  "updated_at": 1640995200
}
```

#### PUT /api/v1/routes/{id}
Update an existing route.

**Request:** Same as POST /api/v1/routes

**Response:**
```json
{
  "message": "Route updated successfully",
  "route": { /* updated route object */ }
}
```

#### DELETE /api/v1/routes/{id}
Delete a route.

**Response:**
```json
{
  "message": "Route deleted successfully"
}
```

### Upstream Management

#### GET /api/v1/upstreams
List all upstreams.

**Response:**
```json
{
  "upstreams": [
    {
      "id": "upstream-001",
      "name": "API Backend",
      "targets": [
        {
          "url": "http://backend1:8080",
          "weight": 100
        },
        {
          "url": "http://backend2:8080",
          "weight": 100
        }
      ],
      "algorithm": "round_robin",
      "created_at": 1640995200,
      "updated_at": 1640995200
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

#### POST /api/v1/upstreams
Create a new upstream.

**Request:**
```json
{
  "name": "New Backend",
  "targets": [
    {
      "url": "http://backend3:8080",
      "weight": 100
    }
  ],
  "algorithm": "round_robin"
}
```

#### GET /api/v1/upstreams/{id}
Get a specific upstream by ID.

#### PUT /api/v1/upstreams/{id}
Update an existing upstream.

#### DELETE /api/v1/upstreams/{id}
Delete an upstream.

### Plugin Management

#### GET /api/v1/plugins
List all plugins.

**Response:**
```json
{
  "plugins": [
    {
      "id": "plugin-001",
      "name": "Rate Limiter",
      "type": "rate_limit",
      "enabled": true,
      "config": {
        "requests_per_minute": 100,
        "burst": 10
      },
      "created_at": 1640995200,
      "updated_at": 1640995200
    }
  ],
  "total": 1,
  "limit": 50,
  "offset": 0
}
```

#### POST /api/v1/plugins
Create a new plugin.

**Request:**
```json
{
  "name": "CORS Plugin",
  "type": "cors",
  "enabled": true,
  "config": {
    "allowed_origins": ["*"],
    "allowed_methods": ["GET", "POST", "PUT", "DELETE"],
    "allowed_headers": ["Content-Type", "Authorization"]
  }
}
```

#### GET /api/v1/plugins/{id}
Get a specific plugin by ID.

#### PUT /api/v1/plugins/{id}
Update an existing plugin.

#### DELETE /api/v1/plugins/{id}
Delete a plugin.

### Configuration Management

#### GET /api/v1/config
Get complete gateway configuration.

**Response:**
```json
{
  "routes": [ /* array of routes */ ],
  "upstreams": [ /* array of upstreams */ ],
  "plugins": [ /* array of plugins */ ],
  "global_config": {
    "timeout": 30,
    "retries": 3
  }
}
```

#### POST /api/v1/config/validate
Validate configuration before applying.

**Request:**
```json
{
  "routes": [ /* array of routes */ ],
  "upstreams": [ /* array of upstreams */ ],
  "plugins": [ /* array of plugins */ ]
}
```

**Response:**
```json
{
  "valid": true,
  "errors": []
}
```

## Portal API Endpoints

### Authentication

#### POST /api/v1/auth/login
Portal user authentication.

#### POST /api/v1/auth/logout
Logout current user.

#### POST /api/v1/auth/refresh
Refresh JWT token.

### API Discovery

#### GET /api/v1/portal/apis
List all available APIs in the portal.

**Response:**
```json
{
  "apis": [
    {
      "id": "api-001",
      "title": "User Management API",
      "description": "API for managing users",
      "version": "1.0.0",
      "tags": ["users", "authentication"],
      "servers": ["https://api.example.com"],
      "paths_count": 15,
      "last_updated": 1640995200
    }
  ]
}
```

#### GET /api/v1/portal/apis/{id}
Get detailed information about a specific API.

**Response:**
```json
{
  "route_id": "api-001",
  "title": "User Management API",
  "description": "API for managing users",
  "version": "1.0.0",
  "tags": ["users", "authentication"],
  "servers": ["https://api.example.com"],
  "paths": [
    {
      "path": "/users",
      "method": "GET",
      "summary": "List users",
      "description": "Get a list of all users"
    },
    {
      "path": "/users",
      "method": "POST",
      "summary": "Create user",
      "description": "Create a new user"
    }
  ],
  "metadata": {
    "contact": "api-team@example.com",
    "license": "MIT"
  }
}
```

### API Testing

#### POST /api/v1/portal/test
Test API endpoints through the portal proxy.

**Request:**
```json
{
  "method": "GET",
  "url": "/users",
  "headers": {
    "Authorization": "Bearer token"
  },
  "body": null
}
```

### Dashboard

#### GET /api/v1/portal/dashboard
Get dashboard statistics.

**Response:**
```json
{
  "total_apis": 5,
  "total_requests": 1234,
  "success_rate": 99.5,
  "avg_response_time": 150,
  "recent_activity": [
    {
      "timestamp": 1640995200,
      "action": "API_CALL",
      "api": "user-api",
      "status": "success"
    }
  ]
}
```

### Search

#### GET /api/v1/portal/search
Search APIs and endpoints.

**Query Parameters:**
- `q` (string): Search query
- `type` (string): Search type (apis, endpoints, all)

**Response:**
```json
{
  "results": [
    {
      "type": "api",
      "id": "user-api",
      "title": "User Management API",
      "description": "API for managing users",
      "score": 0.95
    }
  ],
  "total": 1
}
```

## Application Management

### GET /api/applications
List user applications.

**Query Parameters:**
- `limit` (integer): Maximum number of applications to return
- `offset` (integer): Number of applications to skip

**Response:**
```json
{
  "applications": [
    {
      "id": "app-001",
      "name": "My Mobile App",
      "description": "Mobile application for users",
      "api_key": "ak_1234567890abcdef",
      "status": "active",
      "created_at": "2023-01-01T00:00:00Z",
      "updated_at": "2023-01-01T00:00:00Z"
    }
  ],
  "total": 1,
  "offset": 0,
  "limit": 10,
  "has_more": false
}
```

### POST /api/applications/create
Create a new application.

**Request:**
```json
{
  "name": "New Application",
  "description": "Description of the new application"
}
```

**Response:**
```json
{
  "id": "app-002",
  "name": "New Application",
  "description": "Description of the new application",
  "api_key": "ak_abcdef1234567890",
  "status": "active",
  "created_at": "2023-01-02T00:00:00Z",
  "updated_at": "2023-01-02T00:00:00Z"
}
```

### GET /api/applications/{id}
Get application details.

### PUT /api/applications/{id}
Update application.

### DELETE /api/applications/{id}
Delete application.

## Error Responses

All API endpoints return consistent error responses:

```json
{
  "error": "Error message",
  "status": 400,
  "details": "Additional error details"
}
```

### Common HTTP Status Codes

- `200 OK`: Request successful
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request data
- `401 Unauthorized`: Authentication required
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `409 Conflict`: Resource already exists
- `422 Unprocessable Entity`: Validation failed
- `500 Internal Server Error`: Server error

## Rate Limiting

API requests are subject to rate limiting:

- **Admin API**: 1000 requests per hour per API key
- **Portal API**: 100 requests per minute per user

Rate limit headers are included in responses:
```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1640995200
```

## Pagination

List endpoints support pagination using query parameters:

- `limit`: Maximum number of items to return (default: 50, max: 100)
- `offset`: Number of items to skip (default: 0)

Paginated responses include metadata:
```json
{
  "data": [ /* array of items */ ],
  "total": 150,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

## WebSocket API

The portal supports real-time updates via WebSocket connections:

### Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/portal/ws');
```

### Message Format
```json
{
  "type": "api_update",
  "data": {
    "api_id": "api-001",
    "action": "updated",
    "timestamp": 1640995200
  }
}
```

### Event Types
- `api_update`: API specification updated
- `route_change`: Route configuration changed
- `system_status`: System status change

## SDK and Client Libraries

Official SDKs are available for:

- **JavaScript/Node.js**: `npm install @stargate/sdk`
- **Python**: `pip install stargate-sdk`
- **Go**: `go get github.com/stargate/go-sdk`

### JavaScript Example
```javascript
import { StargateClient } from '@stargate/sdk';

const client = new StargateClient({
  baseURL: 'http://localhost:9090',
  apiKey: 'your-api-key'
});

// List routes
const routes = await client.routes.list();

// Create route
const newRoute = await client.routes.create({
  name: 'My Route',
  rules: {
    hosts: ['api.example.com'],
    paths: [{ type: 'prefix', value: '/api' }]
  },
  upstream_id: 'upstream-001'
});
```

## OpenAPI Specification

The complete OpenAPI 3.0 specification is available at:
- **JSON**: `GET /docs/openapi.json`
- **Interactive UI**: `GET /docs`

## Support

For API support and questions:
- **Documentation**: https://docs.stargate.io
- **GitHub Issues**: https://github.com/stargate/stargate/issues
- **Community**: https://community.stargate.io
