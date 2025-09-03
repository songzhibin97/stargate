# Stargate API Quick Start Guide

## Overview

This guide will help you get started with the Stargate API in just a few minutes. You'll learn how to authenticate, create your first route, and test it.

## Prerequisites

- Stargate server running (see [Installation Guide](installation.md))
- `curl` or similar HTTP client
- Basic understanding of REST APIs

## Step 1: Verify API Access

First, check that the Stargate API is running:

```bash
curl http://localhost:9090/health
```

Expected response:
```json
{
  "status": "healthy",
  "timestamp": 1640995200
}
```

## Step 2: Authentication

### Option A: Generate API Key

Generate an API key for authentication:

```bash
curl -X POST http://localhost:9090/auth/api-keys \
  -H "Content-Type: application/json" \
  -d '{"name": "quickstart-key"}'
```

Response:
```json
{
  "api_key": "sk-1234567890abcdef",
  "name": "quickstart-key",
  "created": 1640995200
}
```

Save the API key for subsequent requests:
```bash
export STARGATE_API_KEY="sk-1234567890abcdef"
```

### Option B: JWT Authentication

For portal access, authenticate with username/password:

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "password"
  }'
```

Response:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": "24h"
}
```

Save the token:
```bash
export STARGATE_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Step 3: Create Your First Upstream

An upstream defines the backend services that will handle requests:

```bash
curl -X POST http://localhost:9090/api/v1/upstreams \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Backend Service",
    "targets": [
      {
        "url": "http://httpbin.org",
        "weight": 100
      }
    ],
    "algorithm": "round_robin",
    "health_check": {
      "enabled": true,
      "path": "/status/200",
      "interval": 30,
      "timeout": 5
    }
  }'
```

Response:
```json
{
  "message": "Upstream created successfully",
  "upstream": {
    "id": "upstream-001",
    "name": "My Backend Service",
    "targets": [
      {
        "url": "http://httpbin.org",
        "weight": 100
      }
    ],
    "algorithm": "round_robin",
    "created_at": 1640995200,
    "updated_at": 1640995200
  }
}
```

Note the `upstream.id` for the next step.

## Step 4: Create Your First Route

A route defines how incoming requests are matched and forwarded to upstreams:

```bash
curl -X POST http://localhost:9090/api/v1/routes \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My First Route",
    "rules": {
      "hosts": ["localhost"],
      "paths": [
        {
          "type": "prefix",
          "value": "/api"
        }
      ],
      "methods": ["GET", "POST"]
    },
    "upstream_id": "upstream-001",
    "priority": 100
  }'
```

Response:
```json
{
  "message": "Route created successfully",
  "route": {
    "id": "route-001",
    "name": "My First Route",
    "rules": {
      "hosts": ["localhost"],
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
    "created_at": 1640995200,
    "updated_at": 1640995200
  }
}
```

## Step 5: Test Your Route

Now test that your route is working by making a request through the gateway:

```bash
curl -H "Host: localhost" http://localhost:8080/api/get
```

This request will:
1. Match your route (host: localhost, path: /api/*)
2. Forward to httpbin.org/get
3. Return the response

Expected response from httpbin.org:
```json
{
  "args": {},
  "headers": {
    "Accept": "*/*",
    "Host": "httpbin.org",
    "User-Agent": "curl/7.68.0"
  },
  "origin": "...",
  "url": "https://httpbin.org/get"
}
```

## Step 6: Add Rate Limiting

Let's add a rate limiting plugin to your route:

```bash
curl -X POST http://localhost:9090/api/v1/plugins \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Rate Limit for My Route",
    "type": "rate_limit",
    "route_id": "route-001",
    "enabled": true,
    "config": {
      "requests_per_minute": 60,
      "burst": 10,
      "key_source": "ip"
    }
  }'
```

Response:
```json
{
  "message": "Plugin created successfully",
  "plugin": {
    "id": "plugin-001",
    "name": "Rate Limit for My Route",
    "type": "rate_limit",
    "route_id": "route-001",
    "enabled": true,
    "config": {
      "requests_per_minute": 60,
      "burst": 10,
      "key_source": "ip"
    },
    "created_at": 1640995200
  }
}
```

## Step 7: Test Rate Limiting

Make multiple requests quickly to test the rate limiting:

```bash
for i in {1..15}; do
  curl -H "Host: localhost" http://localhost:8080/api/get
  echo "Request $i completed"
done
```

After the 10th request (burst limit), you should start seeing rate limit responses:

```json
{
  "error": "Rate limit exceeded",
  "status": 429,
  "details": "Too many requests from this IP address"
}
```

## Step 8: Monitor Your Gateway

Check the health and metrics of your gateway:

```bash
# Detailed health check
curl -H "X-Admin-Key: $STARGATE_API_KEY" \
  "http://localhost:9090/health?detailed=true"

# Get Prometheus metrics
curl -H "X-Admin-Key: $STARGATE_API_KEY" \
  http://localhost:9090/metrics
```

## Step 9: Explore the Portal

If you have the developer portal enabled, visit:

```
http://localhost:8080
```

Login with your credentials and explore:
- API documentation
- Interactive API testing
- Request analytics
- Application management

## Common Next Steps

### 1. Add Authentication

Add JWT authentication to your route:

```bash
curl -X POST http://localhost:9090/api/v1/plugins \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "JWT Auth",
    "type": "auth",
    "route_id": "route-001",
    "enabled": true,
    "config": {
      "auth_type": "jwt",
      "jwt_secret": "your-secret-key",
      "header_name": "Authorization",
      "token_prefix": "Bearer "
    }
  }'
```

### 2. Add CORS Support

Enable CORS for browser requests:

```bash
curl -X POST http://localhost:9090/api/v1/plugins \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CORS Support",
    "type": "cors",
    "route_id": "route-001",
    "enabled": true,
    "config": {
      "allowed_origins": ["*"],
      "allowed_methods": ["GET", "POST", "PUT", "DELETE"],
      "allowed_headers": ["Content-Type", "Authorization"],
      "allow_credentials": true
    }
  }'
```

### 3. Add Multiple Backends

Update your upstream to include multiple backend servers:

```bash
curl -X PUT http://localhost:9090/api/v1/upstreams/upstream-001 \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Load Balanced Backend",
    "targets": [
      {
        "url": "http://backend1.example.com:8080",
        "weight": 70
      },
      {
        "url": "http://backend2.example.com:8080",
        "weight": 30
      }
    ],
    "algorithm": "weighted",
    "health_check": {
      "enabled": true,
      "path": "/health",
      "interval": 30,
      "timeout": 5
    }
  }'
```

### 4. Set Up Path-Based Routing

Create different routes for different API versions:

```bash
# API v1 route
curl -X POST http://localhost:9090/api/v1/routes \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API v1 Route",
    "rules": {
      "hosts": ["api.mycompany.com"],
      "paths": [{"type": "prefix", "value": "/v1"}]
    },
    "upstream_id": "v1-upstream",
    "priority": 200
  }'

# API v2 route
curl -X POST http://localhost:9090/api/v1/routes \
  -H "X-Admin-Key: $STARGATE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API v2 Route",
    "rules": {
      "hosts": ["api.mycompany.com"],
      "paths": [{"type": "prefix", "value": "/v2"}]
    },
    "upstream_id": "v2-upstream",
    "priority": 200
  }'
```

## Troubleshooting

### Route Not Matching

If your requests aren't matching your route:

1. Check the host header:
   ```bash
   curl -v -H "Host: localhost" http://localhost:8080/api/get
   ```

2. Verify route configuration:
   ```bash
   curl -H "X-Admin-Key: $STARGATE_API_KEY" \
     http://localhost:9090/api/v1/routes/route-001
   ```

3. Check route priority and conflicts:
   ```bash
   curl -H "X-Admin-Key: $STARGATE_API_KEY" \
     http://localhost:9090/api/v1/routes | jq '.routes | sort_by(.priority)'
   ```

### Upstream Connection Issues

If you're getting 502 Bad Gateway errors:

1. Check upstream health:
   ```bash
   curl -H "X-Admin-Key: $STARGATE_API_KEY" \
     http://localhost:9090/api/v1/upstreams/upstream-001
   ```

2. Test backend directly:
   ```bash
   curl http://httpbin.org/get
   ```

3. Check upstream configuration:
   ```bash
   curl -H "X-Admin-Key: $STARGATE_API_KEY" \
     http://localhost:9090/api/v1/upstreams
   ```

### Plugin Issues

If plugins aren't working as expected:

1. Verify plugin is enabled:
   ```bash
   curl -H "X-Admin-Key: $STARGATE_API_KEY" \
     http://localhost:9090/api/v1/plugins/plugin-001
   ```

2. Check plugin configuration:
   ```bash
   curl -H "X-Admin-Key: $STARGATE_API_KEY" \
     http://localhost:9090/api/v1/plugins | jq '.plugins[] | select(.route_id == "route-001")'
   ```

## Next Steps

- Read the [Complete API Documentation](api-documentation.md)
- Explore [Advanced Examples](api-examples.md)
- Check out the [SDK Documentation](sdk-documentation.md)
- Learn about [Plugin Development](plugin-development.md)
- Set up [Monitoring and Observability](monitoring.md)

## Getting Help

- **Documentation**: https://docs.stargate.io
- **GitHub Issues**: https://github.com/stargate/stargate/issues
- **Community Forum**: https://community.stargate.io
- **Discord**: https://discord.gg/stargate

Congratulations! You've successfully set up your first Stargate API gateway with routing, rate limiting, and monitoring. You're now ready to build more complex API gateway configurations.
