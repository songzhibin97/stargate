# Stargate API Examples

## Complete Usage Examples

### 1. Setting Up a Basic API Gateway

This example shows how to set up a complete API gateway configuration from scratch.

#### Step 1: Create an Upstream
```bash
curl -X POST http://localhost:9090/api/v1/upstreams \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "User Service Backend",
    "targets": [
      {
        "url": "http://user-service-1:8080",
        "weight": 100
      },
      {
        "url": "http://user-service-2:8080",
        "weight": 100
      }
    ],
    "algorithm": "round_robin",
    "health_check": {
      "enabled": true,
      "path": "/health",
      "interval": 30,
      "timeout": 5
    }
  }'
```

#### Step 2: Create a Route
```bash
curl -X POST http://localhost:9090/api/v1/routes \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "User API Route",
    "rules": {
      "hosts": ["api.mycompany.com"],
      "paths": [
        {
          "type": "prefix",
          "value": "/api/v1/users"
        }
      ],
      "methods": ["GET", "POST", "PUT", "DELETE"]
    },
    "upstream_id": "upstream-001",
    "priority": 100,
    "metadata": {
      "team": "user-team",
      "environment": "production"
    }
  }'
```

#### Step 3: Add Rate Limiting Plugin
```bash
curl -X POST http://localhost:9090/api/v1/plugins \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "User API Rate Limit",
    "type": "rate_limit",
    "enabled": true,
    "route_id": "route-001",
    "config": {
      "requests_per_minute": 1000,
      "requests_per_hour": 10000,
      "burst": 50,
      "key_source": "ip",
      "response_headers": true
    }
  }'
```

#### Step 4: Add Authentication Plugin
```bash
curl -X POST http://localhost:9090/api/v1/plugins \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "User API Auth",
    "type": "auth",
    "enabled": true,
    "route_id": "route-001",
    "config": {
      "auth_type": "jwt",
      "jwt_secret": "your-jwt-secret",
      "jwt_algorithm": "HS256",
      "header_name": "Authorization",
      "token_prefix": "Bearer ",
      "bypass_paths": ["/api/v1/users/login", "/api/v1/users/register"]
    }
  }'
```

### 2. Advanced Route Configuration

#### Regex Path Matching
```json
{
  "name": "Dynamic User Route",
  "rules": {
    "hosts": ["api.mycompany.com"],
    "paths": [
      {
        "type": "regex",
        "value": "^/api/v1/users/[0-9]+(/.*)?$"
      }
    ],
    "methods": ["GET", "PUT", "DELETE"]
  },
  "upstream_id": "upstream-001"
}
```

#### Multiple Host Patterns
```json
{
  "name": "Multi-Domain Route",
  "rules": {
    "hosts": [
      "api.mycompany.com",
      "*.api.mycompany.com",
      "api-staging.mycompany.com"
    ],
    "paths": [
      {
        "type": "prefix",
        "value": "/api"
      }
    ]
  },
  "upstream_id": "upstream-001"
}
```

### 3. Load Balancing Configurations

#### Weighted Round Robin
```json
{
  "name": "Weighted Backend",
  "targets": [
    {
      "url": "http://primary-server:8080",
      "weight": 70
    },
    {
      "url": "http://secondary-server:8080",
      "weight": 30
    }
  ],
  "algorithm": "weighted"
}
```

#### IP Hash Load Balancing
```json
{
  "name": "Session Sticky Backend",
  "targets": [
    {
      "url": "http://server-1:8080",
      "weight": 100
    },
    {
      "url": "http://server-2:8080",
      "weight": 100
    }
  ],
  "algorithm": "ip_hash"
}
```

### 4. Plugin Configurations

#### CORS Plugin
```json
{
  "name": "API CORS",
  "type": "cors",
  "enabled": true,
  "config": {
    "allowed_origins": [
      "https://myapp.com",
      "https://*.myapp.com"
    ],
    "allowed_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    "allowed_headers": [
      "Content-Type",
      "Authorization",
      "X-Requested-With"
    ],
    "exposed_headers": ["X-Total-Count"],
    "allow_credentials": true,
    "max_age": 86400
  }
}
```

#### Circuit Breaker Plugin
```json
{
  "name": "API Circuit Breaker",
  "type": "circuit_breaker",
  "enabled": true,
  "config": {
    "failure_threshold": 5,
    "recovery_timeout": 30,
    "success_threshold": 3,
    "timeout": 10,
    "fallback_response": {
      "status": 503,
      "body": "{\"error\": \"Service temporarily unavailable\"}"
    }
  }
}
```

#### Header Transform Plugin
```json
{
  "name": "Header Transform",
  "type": "header_transform",
  "enabled": true,
  "config": {
    "request": {
      "add": {
        "X-Forwarded-By": "Stargate",
        "X-Request-ID": "${uuid}"
      },
      "remove": ["X-Internal-Token"],
      "replace": {
        "User-Agent": "Stargate-Proxy/1.0"
      }
    },
    "response": {
      "add": {
        "X-Powered-By": "Stargate"
      },
      "remove": ["Server"]
    }
  }
}
```

### 5. Traffic Mirror Plugin
```json
{
  "name": "Traffic Mirror",
  "type": "traffic_mirror",
  "enabled": true,
  "config": {
    "mirror_targets": [
      {
        "url": "http://analytics-service:8080",
        "percentage": 10
      },
      {
        "url": "http://test-environment:8080",
        "percentage": 5,
        "conditions": {
          "headers": {
            "X-Test-User": "true"
          }
        }
      }
    ],
    "async": true,
    "timeout": 5
  }
}
```

### 6. Mock Response Plugin
```json
{
  "name": "API Mock",
  "type": "mock_response",
  "enabled": true,
  "config": {
    "responses": [
      {
        "condition": {
          "path": "/api/v1/users/mock",
          "method": "GET"
        },
        "response": {
          "status": 200,
          "headers": {
            "Content-Type": "application/json"
          },
          "body": "{\"users\": [{\"id\": 1, \"name\": \"Mock User\"}]}"
        }
      }
    ],
    "delay": 100
  }
}
```

### 7. Portal API Usage

#### Authenticate and Get APIs
```bash
# Login
TOKEN=$(curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "developer", "password": "password"}' \
  | jq -r '.token')

# List available APIs
curl -X GET http://localhost:8080/api/v1/portal/apis \
  -H "Authorization: Bearer $TOKEN"

# Get API details
curl -X GET http://localhost:8080/api/v1/portal/apis/user-api \
  -H "Authorization: Bearer $TOKEN"
```

#### Test API Endpoint
```bash
curl -X POST http://localhost:8080/api/v1/portal/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "url": "/api/v1/users",
    "headers": {
      "Authorization": "Bearer user-token"
    }
  }'
```

### 8. Application Management

#### Create Application
```bash
curl -X POST http://localhost:9090/api/applications/create \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Mobile App",
    "description": "iOS and Android mobile application"
  }'
```

#### List Applications
```bash
curl -X GET http://localhost:9090/api/applications?limit=10&offset=0 \
  -H "Authorization: Bearer $JWT_TOKEN"
```

### 9. Configuration Management

#### Export Configuration
```bash
curl -X GET http://localhost:9090/api/v1/config \
  -H "X-Admin-Key: your-api-key" \
  > stargate-config.json
```

#### Validate Configuration
```bash
curl -X POST http://localhost:9090/api/v1/config/validate \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d @stargate-config.json
```

### 10. Monitoring and Observability

#### Get Metrics
```bash
curl -X GET http://localhost:9090/metrics \
  -H "X-Admin-Key: your-api-key"
```

#### Health Check with Details
```bash
curl -X GET http://localhost:9090/health?detailed=true \
  -H "X-Admin-Key: your-api-key"
```

### 11. Batch Operations

#### Create Multiple Routes
```bash
curl -X POST http://localhost:9090/api/v1/routes/batch \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "routes": [
      {
        "name": "Users API",
        "rules": {
          "hosts": ["api.example.com"],
          "paths": [{"type": "prefix", "value": "/users"}]
        },
        "upstream_id": "users-upstream"
      },
      {
        "name": "Orders API",
        "rules": {
          "hosts": ["api.example.com"],
          "paths": [{"type": "prefix", "value": "/orders"}]
        },
        "upstream_id": "orders-upstream"
      }
    ]
  }'
```

### 12. WebSocket Real-time Updates

#### JavaScript Client
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/portal/ws');

ws.onopen = function() {
  console.log('Connected to Stargate WebSocket');
  
  // Subscribe to specific events
  ws.send(JSON.stringify({
    type: 'subscribe',
    events: ['api_update', 'route_change']
  }));
};

ws.onmessage = function(event) {
  const message = JSON.parse(event.data);
  
  switch(message.type) {
    case 'api_update':
      console.log('API updated:', message.data);
      break;
    case 'route_change':
      console.log('Route changed:', message.data);
      break;
  }
};
```

### 13. Error Handling Examples

#### Handling Validation Errors
```bash
# This will return a 400 error with validation details
curl -X POST http://localhost:9090/api/v1/routes \
  -H "X-Admin-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "",
    "rules": {
      "hosts": [],
      "paths": []
    }
  }'

# Response:
# {
#   "error": "Route validation failed",
#   "status": 400,
#   "details": "name is required, at least one host or path must be specified"
# }
```

#### Handling Rate Limits
```bash
# When rate limit is exceeded
# Response headers will include:
# X-RateLimit-Limit: 1000
# X-RateLimit-Remaining: 0
# X-RateLimit-Reset: 1640995200
# 
# Response body:
# {
#   "error": "Rate limit exceeded",
#   "status": 429,
#   "details": "Try again in 60 seconds"
# }
```

### 14. Advanced Authentication

#### Custom JWT Claims
```json
{
  "name": "Advanced JWT Auth",
  "type": "auth",
  "enabled": true,
  "config": {
    "auth_type": "jwt",
    "jwt_secret": "your-secret",
    "required_claims": {
      "role": ["admin", "user"],
      "scope": "api:read"
    },
    "claim_headers": {
      "X-User-ID": "sub",
      "X-User-Role": "role"
    }
  }
}
```

#### API Key with Scopes
```json
{
  "name": "Scoped API Key Auth",
  "type": "auth",
  "enabled": true,
  "config": {
    "auth_type": "api_key",
    "header_name": "X-API-Key",
    "scopes": {
      "read": ["GET"],
      "write": ["POST", "PUT", "DELETE"]
    }
  }
}
```

## Best Practices

### 1. Route Organization
- Use descriptive names for routes
- Group related routes with consistent naming
- Use metadata for categorization
- Set appropriate priorities

### 2. Upstream Configuration
- Always configure health checks
- Use appropriate load balancing algorithms
- Set reasonable timeouts
- Monitor upstream health

### 3. Plugin Usage
- Apply plugins at the appropriate level (global, route, or service)
- Test plugin configurations in staging first
- Monitor plugin performance impact
- Use plugin priorities correctly

### 4. Security
- Always use authentication for production APIs
- Implement rate limiting
- Use HTTPS in production
- Regularly rotate API keys

### 5. Monitoring
- Enable metrics collection
- Set up alerting for critical endpoints
- Monitor response times and error rates
- Use distributed tracing for complex flows
