# Stargate SDK Documentation

## Official SDKs

Stargate provides official SDKs for popular programming languages to simplify integration with the API.

## JavaScript/Node.js SDK

### Installation
```bash
npm install @stargate/sdk
# or
yarn add @stargate/sdk
```

### Basic Usage
```javascript
import { StargateClient } from '@stargate/sdk';

const client = new StargateClient({
  baseURL: 'http://localhost:9090',
  apiKey: 'your-api-key',
  timeout: 30000
});

// List routes
const routes = await client.routes.list();
console.log(routes);

// Create a route
const newRoute = await client.routes.create({
  name: 'My API Route',
  rules: {
    hosts: ['api.example.com'],
    paths: [{ type: 'prefix', value: '/api' }],
    methods: ['GET', 'POST']
  },
  upstream_id: 'upstream-001'
});
```

### Advanced Configuration
```javascript
const client = new StargateClient({
  baseURL: 'http://localhost:9090',
  apiKey: 'your-api-key',
  timeout: 30000,
  retries: 3,
  retryDelay: 1000,
  headers: {
    'User-Agent': 'MyApp/1.0'
  }
});

// Enable debug logging
client.setDebug(true);

// Custom error handling
client.on('error', (error) => {
  console.error('Stargate API Error:', error);
});
```

### Route Management
```javascript
// List routes with pagination
const routes = await client.routes.list({
  limit: 10,
  offset: 0
});

// Get specific route
const route = await client.routes.get('route-001');

// Update route
const updatedRoute = await client.routes.update('route-001', {
  name: 'Updated Route Name',
  priority: 200
});

// Delete route
await client.routes.delete('route-001');

// Batch operations
const batchResult = await client.routes.createBatch([
  {
    name: 'Route 1',
    rules: { hosts: ['api1.example.com'] },
    upstream_id: 'upstream-001'
  },
  {
    name: 'Route 2',
    rules: { hosts: ['api2.example.com'] },
    upstream_id: 'upstream-002'
  }
]);
```

### Upstream Management
```javascript
// Create upstream with health checks
const upstream = await client.upstreams.create({
  name: 'Backend Service',
  targets: [
    { url: 'http://backend1:8080', weight: 100 },
    { url: 'http://backend2:8080', weight: 100 }
  ],
  algorithm: 'round_robin',
  health_check: {
    enabled: true,
    path: '/health',
    interval: 30,
    timeout: 5
  }
});

// Get upstream health status
const health = await client.upstreams.getHealth('upstream-001');
```

### Plugin Management
```javascript
// Create rate limiting plugin
const plugin = await client.plugins.create({
  name: 'API Rate Limit',
  type: 'rate_limit',
  route_id: 'route-001',
  config: {
    requests_per_minute: 1000,
    burst: 50
  }
});

// List plugins by type
const authPlugins = await client.plugins.list({ type: 'auth' });

// Enable/disable plugin
await client.plugins.update('plugin-001', { enabled: false });
```

### Configuration Management
```javascript
// Get complete configuration
const config = await client.config.get();

// Validate configuration
const validation = await client.config.validate(config);
if (!validation.valid) {
  console.error('Configuration errors:', validation.errors);
}

// Export configuration
const configJson = await client.config.export();
```

### Portal API Integration
```javascript
import { StargatePortalClient } from '@stargate/sdk';

const portalClient = new StargatePortalClient({
  baseURL: 'http://localhost:8080',
  username: 'developer',
  password: 'password'
});

// Authenticate
await portalClient.auth.login();

// List available APIs
const apis = await portalClient.apis.list();

// Get API documentation
const apiDoc = await portalClient.apis.getDocumentation('user-api');

// Test API endpoint
const testResult = await portalClient.test.request({
  method: 'GET',
  url: '/api/v1/users',
  headers: { 'Authorization': 'Bearer token' }
});
```

## Python SDK

### Installation
```bash
pip install stargate-sdk
```

### Basic Usage
```python
from stargate_sdk import StargateClient

client = StargateClient(
    base_url='http://localhost:9090',
    api_key='your-api-key'
)

# List routes
routes = client.routes.list()
print(routes)

# Create route
route = client.routes.create({
    'name': 'My API Route',
    'rules': {
        'hosts': ['api.example.com'],
        'paths': [{'type': 'prefix', 'value': '/api'}]
    },
    'upstream_id': 'upstream-001'
})
```

### Async Support
```python
import asyncio
from stargate_sdk import AsyncStargateClient

async def main():
    client = AsyncStargateClient(
        base_url='http://localhost:9090',
        api_key='your-api-key'
    )
    
    # Async operations
    routes = await client.routes.list()
    
    # Context manager for automatic cleanup
    async with client:
        route = await client.routes.create({
            'name': 'Async Route',
            'rules': {'hosts': ['async.example.com']},
            'upstream_id': 'upstream-001'
        })

asyncio.run(main())
```

### Error Handling
```python
from stargate_sdk import StargateClient, StargateError, ValidationError

client = StargateClient(base_url='http://localhost:9090', api_key='key')

try:
    route = client.routes.create({
        'name': '',  # Invalid: empty name
        'rules': {'hosts': []},  # Invalid: no hosts
        'upstream_id': 'upstream-001'
    })
except ValidationError as e:
    print(f"Validation error: {e.message}")
    print(f"Details: {e.details}")
except StargateError as e:
    print(f"API error: {e.status_code} - {e.message}")
```

### Configuration with Environment Variables
```python
import os
from stargate_sdk import StargateClient

# Automatically uses environment variables:
# STARGATE_BASE_URL, STARGATE_API_KEY
client = StargateClient.from_env()

# Or with custom environment variables
client = StargateClient(
    base_url=os.getenv('MY_STARGATE_URL'),
    api_key=os.getenv('MY_STARGATE_KEY')
)
```

## Go SDK

### Installation
```bash
go get github.com/stargate/go-sdk
```

### Basic Usage
```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/stargate/go-sdk/client"
)

func main() {
    cfg := client.Config{
        BaseURL: "http://localhost:9090",
        APIKey:  "your-api-key",
    }
    
    c, err := client.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    
    ctx := context.Background()
    
    // List routes
    routes, err := c.Routes.List(ctx, &client.ListOptions{
        Limit:  50,
        Offset: 0,
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Found %d routes\n", len(routes.Routes))
}
```

### Route Management
```go
// Create route
route := &client.Route{
    Name: "My API Route",
    Rules: client.RouteRules{
        Hosts: []string{"api.example.com"},
        Paths: []client.PathRule{
            {Type: "prefix", Value: "/api"},
        },
        Methods: []string{"GET", "POST"},
    },
    UpstreamID: "upstream-001",
    Priority:   100,
}

createdRoute, err := c.Routes.Create(ctx, route)
if err != nil {
    log.Fatal(err)
}

// Update route
route.Priority = 200
updatedRoute, err := c.Routes.Update(ctx, route.ID, route)
if err != nil {
    log.Fatal(err)
}
```

### Error Handling
```go
import "github.com/stargate/go-sdk/errors"

route, err := c.Routes.Get(ctx, "non-existent-route")
if err != nil {
    switch e := err.(type) {
    case *errors.NotFoundError:
        fmt.Println("Route not found")
    case *errors.ValidationError:
        fmt.Printf("Validation error: %s\n", e.Details)
    case *errors.APIError:
        fmt.Printf("API error %d: %s\n", e.StatusCode, e.Message)
    default:
        log.Fatal(err)
    }
}
```

### Concurrent Operations
```go
import "sync"

func createMultipleRoutes(c *client.Client, routes []*client.Route) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(routes))
    
    for _, route := range routes {
        wg.Add(1)
        go func(r *client.Route) {
            defer wg.Done()
            _, err := c.Routes.Create(context.Background(), r)
            if err != nil {
                errChan <- err
            }
        }(route)
    }
    
    wg.Wait()
    close(errChan)
    
    for err := range errChan {
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

## Java SDK

### Installation (Maven)
```xml
<dependency>
    <groupId>io.stargate</groupId>
    <artifactId>stargate-sdk</artifactId>
    <version>1.0.0</version>
</dependency>
```

### Installation (Gradle)
```gradle
implementation 'io.stargate:stargate-sdk:1.0.0'
```

### Basic Usage
```java
import io.stargate.sdk.StargateClient;
import io.stargate.sdk.model.Route;
import io.stargate.sdk.model.RouteRules;

public class StargateExample {
    public static void main(String[] args) {
        StargateClient client = StargateClient.builder()
            .baseUrl("http://localhost:9090")
            .apiKey("your-api-key")
            .build();
        
        // List routes
        List<Route> routes = client.routes().list();
        System.out.println("Found " + routes.size() + " routes");
        
        // Create route
        Route route = Route.builder()
            .name("My API Route")
            .rules(RouteRules.builder()
                .hosts(Arrays.asList("api.example.com"))
                .paths(Arrays.asList(PathRule.prefix("/api")))
                .build())
            .upstreamId("upstream-001")
            .build();
            
        Route created = client.routes().create(route);
        System.out.println("Created route: " + created.getId());
    }
}
```

### Async Operations
```java
import java.util.concurrent.CompletableFuture;

StargateAsyncClient asyncClient = StargateAsyncClient.builder()
    .baseUrl("http://localhost:9090")
    .apiKey("your-api-key")
    .build();

// Async route creation
CompletableFuture<Route> future = asyncClient.routes().createAsync(route);
future.thenAccept(createdRoute -> {
    System.out.println("Route created: " + createdRoute.getId());
}).exceptionally(throwable -> {
    System.err.println("Error creating route: " + throwable.getMessage());
    return null;
});
```

## Common SDK Features

### 1. Authentication
All SDKs support multiple authentication methods:
- API Key authentication
- JWT Bearer tokens
- Custom authentication headers

### 2. Error Handling
Consistent error handling across all SDKs:
- Network errors
- HTTP status code errors
- Validation errors
- Rate limiting errors

### 3. Retry Logic
Built-in retry mechanisms with:
- Exponential backoff
- Configurable retry attempts
- Retry on specific error conditions

### 4. Logging and Debugging
Debug capabilities:
- Request/response logging
- Performance metrics
- Error tracking

### 5. Configuration
Flexible configuration options:
- Environment variables
- Configuration files
- Programmatic configuration

## Best Practices

### 1. Connection Management
```javascript
// Reuse client instances
const client = new StargateClient(config);

// Use connection pooling for high-throughput applications
const client = new StargateClient({
  ...config,
  pool: {
    maxConnections: 100,
    keepAlive: true
  }
});
```

### 2. Error Handling
```python
# Always handle specific error types
try:
    route = client.routes.create(route_data)
except ValidationError as e:
    # Handle validation errors
    logger.error(f"Invalid route data: {e.details}")
except RateLimitError as e:
    # Handle rate limiting
    time.sleep(e.retry_after)
    route = client.routes.create(route_data)
except StargateError as e:
    # Handle other API errors
    logger.error(f"API error: {e.status_code} - {e.message}")
```

### 3. Performance Optimization
```go
// Use context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

routes, err := client.Routes.List(ctx, &client.ListOptions{
    Limit: 100,  // Batch requests
})
```

### 4. Security
```java
// Never hardcode credentials
StargateClient client = StargateClient.builder()
    .baseUrl(System.getenv("STARGATE_URL"))
    .apiKey(System.getenv("STARGATE_API_KEY"))
    .build();

// Use HTTPS in production
StargateClient client = StargateClient.builder()
    .baseUrl("https://api.stargate.io")
    .sslVerification(true)
    .build();
```

## Migration Guide

### From REST API to SDK
```javascript
// Before: Direct REST API calls
const response = await fetch('http://localhost:9090/api/v1/routes', {
  headers: {
    'X-Admin-Key': 'your-api-key',
    'Content-Type': 'application/json'
  }
});
const routes = await response.json();

// After: Using SDK
const client = new StargateClient({
  baseURL: 'http://localhost:9090',
  apiKey: 'your-api-key'
});
const routes = await client.routes.list();
```

### Version Compatibility
- SDK v1.x: Compatible with Stargate API v1.0+
- SDK v2.x: Compatible with Stargate API v2.0+
- Always check compatibility matrix in documentation

## Support and Resources

### Documentation
- **API Reference**: https://docs.stargate.io/api
- **SDK Examples**: https://github.com/stargate/sdk-examples
- **Tutorials**: https://docs.stargate.io/tutorials

### Community
- **GitHub**: https://github.com/stargate/stargate
- **Discord**: https://discord.gg/stargate
- **Stack Overflow**: Tag `stargate-api`

### Contributing
- **SDK Issues**: Report bugs and feature requests
- **Pull Requests**: Contribute improvements
- **Documentation**: Help improve SDK documentation
