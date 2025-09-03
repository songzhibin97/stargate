# Stargate API Troubleshooting Guide

## Common Issues and Solutions

### 1. Authentication Issues

#### Problem: 401 Unauthorized
```json
{
  "error": "Unauthorized",
  "status": 401,
  "details": "Invalid or missing authentication credentials"
}
```

**Solutions:**
1. **Check API Key Format**
   ```bash
   # Correct format
   curl -H "X-Admin-Key: sk-1234567890abcdef" http://localhost:9090/health
   
   # Incorrect - missing header
   curl http://localhost:9090/api/v1/routes
   ```

2. **Verify JWT Token**
   ```bash
   # Check token expiration
   echo "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." | base64 -d
   
   # Refresh expired token
   curl -X POST http://localhost:8080/api/v1/auth/refresh \
     -H "Authorization: Bearer $OLD_TOKEN"
   ```

3. **Validate Token Scope**
   ```bash
   # Check if token has required permissions
   curl -X GET http://localhost:9090/api/v1/routes \
     -H "Authorization: Bearer $TOKEN" \
     -v  # Verbose output for debugging
   ```

#### Problem: 403 Forbidden
```json
{
  "error": "Forbidden",
  "status": 403,
  "details": "Insufficient permissions for this operation"
}
```

**Solutions:**
1. **Check User Permissions**
   ```bash
   # Verify user role and permissions
   curl -X GET http://localhost:8080/api/v1/auth/profile \
     -H "Authorization: Bearer $TOKEN"
   ```

2. **Use Admin API Key**
   ```bash
   # For admin operations, use admin API key
   curl -X POST http://localhost:9090/api/v1/routes \
     -H "X-Admin-Key: $ADMIN_KEY" \
     -H "Content-Type: application/json" \
     -d '{"name": "test"}'
   ```

### 2. Route Configuration Issues

#### Problem: Route Not Matching Requests
**Symptoms:**
- 404 Not Found for valid URLs
- Requests going to wrong upstream

**Debugging Steps:**
1. **Check Route Rules**
   ```bash
   # List all routes and check matching rules
   curl -X GET http://localhost:9090/api/v1/routes \
     -H "X-Admin-Key: $API_KEY" | jq '.routes[] | {id, name, rules}'
   ```

2. **Test Route Matching**
   ```bash
   # Enable debug logging to see route matching
   curl -X GET http://localhost:9090/debug/routes/match \
     -H "X-Admin-Key: $API_KEY" \
     -H "X-Debug-Host: api.example.com" \
     -H "X-Debug-Path: /api/v1/users"
   ```

3. **Check Route Priority**
   ```bash
   # Routes with higher priority are matched first
   curl -X GET http://localhost:9090/api/v1/routes \
     -H "X-Admin-Key: $API_KEY" | jq '.routes | sort_by(.priority) | reverse'
   ```

**Common Fixes:**
```json
{
  "name": "Fixed Route",
  "rules": {
    "hosts": ["api.example.com", "*.api.example.com"],  // Add wildcard
    "paths": [
      {
        "type": "prefix",
        "value": "/api/v1"  // Remove trailing slash
      }
    ],
    "methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"]  // Add OPTIONS
  },
  "priority": 100  // Set explicit priority
}
```

#### Problem: Regex Path Not Working
```bash
# Test regex pattern
curl -X POST http://localhost:9090/debug/regex/test \
  -H "X-Admin-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "pattern": "^/api/v1/users/[0-9]+$",
    "test_paths": ["/api/v1/users/123", "/api/v1/users/abc"]
  }'
```

### 3. Upstream Connection Issues

#### Problem: 502 Bad Gateway
```json
{
  "error": "Bad Gateway",
  "status": 502,
  "details": "Failed to connect to upstream server"
}
```

**Debugging Steps:**
1. **Check Upstream Health**
   ```bash
   # Get upstream status
   curl -X GET http://localhost:9090/api/v1/upstreams/upstream-001/health \
     -H "X-Admin-Key: $API_KEY"
   ```

2. **Test Direct Connection**
   ```bash
   # Test upstream server directly
   curl -X GET http://backend-server:8080/health
   ```

3. **Check Network Connectivity**
   ```bash
   # From Stargate container/server
   telnet backend-server 8080
   nslookup backend-server
   ```

**Solutions:**
1. **Update Upstream Configuration**
   ```json
   {
     "id": "upstream-001",
     "name": "Backend Service",
     "targets": [
       {
         "url": "http://backend-server:8080",
         "weight": 100
       }
     ],
     "health_check": {
       "enabled": true,
       "path": "/health",
       "interval": 30,
       "timeout": 10,
       "healthy_threshold": 2,
       "unhealthy_threshold": 3
     },
     "timeout": {
       "connect": 5,
       "send": 30,
       "read": 30
     }
   }
   ```

#### Problem: 504 Gateway Timeout
**Solutions:**
1. **Increase Timeout Settings**
   ```json
   {
     "timeout": {
       "connect": 10,
       "send": 60,
       "read": 60
     }
   }
   ```

2. **Check Upstream Performance**
   ```bash
   # Monitor upstream response times
   curl -w "@curl-format.txt" -o /dev/null -s http://backend-server:8080/api
   ```

### 4. Plugin Issues

#### Problem: Plugin Not Working
**Debugging Steps:**
1. **Check Plugin Status**
   ```bash
   curl -X GET http://localhost:9090/api/v1/plugins \
     -H "X-Admin-Key: $API_KEY" | jq '.plugins[] | {id, name, enabled}'
   ```

2. **Verify Plugin Configuration**
   ```bash
   curl -X GET http://localhost:9090/api/v1/plugins/plugin-001 \
     -H "X-Admin-Key: $API_KEY" | jq '.config'
   ```

3. **Check Plugin Logs**
   ```bash
   # Enable plugin debug logging
   curl -X PUT http://localhost:9090/api/v1/config/logging \
     -H "X-Admin-Key: $API_KEY" \
     -H "Content-Type: application/json" \
     -d '{"level": "debug", "plugins": true}'
   ```

#### Problem: Rate Limiting Not Working
**Common Issues:**
1. **Incorrect Key Source**
   ```json
   {
     "config": {
       "key_source": "ip",  // Change to "header" or "user_id"
       "header_name": "X-User-ID"  // Required for header source
     }
   }
   ```

2. **Redis Connection Issues**
   ```bash
   # Test Redis connectivity
   redis-cli -h redis-server ping
   ```

### 5. Performance Issues

#### Problem: High Response Times
**Debugging Steps:**
1. **Enable Metrics**
   ```bash
   curl -X GET http://localhost:9090/metrics \
     -H "X-Admin-Key: $API_KEY"
   ```

2. **Check Resource Usage**
   ```bash
   # Monitor CPU and memory
   docker stats stargate-container
   
   # Check connection pools
   curl -X GET http://localhost:9090/debug/pools \
     -H "X-Admin-Key: $API_KEY"
   ```

3. **Analyze Request Patterns**
   ```bash
   # Get request statistics
   curl -X GET http://localhost:9090/api/v1/stats/requests \
     -H "X-Admin-Key: $API_KEY"
   ```

**Solutions:**
1. **Optimize Connection Pooling**
   ```json
   {
     "upstream": {
       "connection_pool": {
         "max_connections": 100,
         "max_idle_connections": 20,
         "idle_timeout": 300
       }
     }
   }
   ```

2. **Enable Caching**
   ```json
   {
     "name": "Response Cache",
     "type": "cache",
     "config": {
       "cache_ttl": 300,
       "cache_key": "uri",
       "cache_methods": ["GET"],
       "cache_status_codes": [200, 301, 302]
     }
   }
   ```

### 6. Configuration Issues

#### Problem: Configuration Not Applied
**Debugging Steps:**
1. **Validate Configuration**
   ```bash
   curl -X POST http://localhost:9090/api/v1/config/validate \
     -H "X-Admin-Key: $API_KEY" \
     -H "Content-Type: application/json" \
     -d @config.json
   ```

2. **Check Configuration Reload**
   ```bash
   # Force configuration reload
   curl -X POST http://localhost:9090/api/v1/config/reload \
     -H "X-Admin-Key: $API_KEY"
   ```

3. **Monitor Configuration Changes**
   ```bash
   # Watch configuration events
   curl -X GET http://localhost:9090/api/v1/events/config \
     -H "X-Admin-Key: $API_KEY"
   ```

### 7. SSL/TLS Issues

#### Problem: SSL Certificate Errors
**Solutions:**
1. **Check Certificate Configuration**
   ```json
   {
     "tls": {
       "cert_file": "/etc/ssl/certs/server.crt",
       "key_file": "/etc/ssl/private/server.key",
       "ca_file": "/etc/ssl/certs/ca.crt"
     }
   }
   ```

2. **Verify Certificate Validity**
   ```bash
   openssl x509 -in server.crt -text -noout
   openssl verify -CAfile ca.crt server.crt
   ```

### 8. Database/Storage Issues

#### Problem: etcd Connection Errors
**Solutions:**
1. **Check etcd Connectivity**
   ```bash
   etcdctl --endpoints=http://etcd:2379 endpoint health
   ```

2. **Verify etcd Configuration**
   ```yaml
   storage:
     type: etcd
     endpoints:
       - http://etcd-1:2379
       - http://etcd-2:2379
     timeout: 5s
     username: stargate
     password: password
   ```

### 9. Logging and Debugging

#### Enable Debug Logging
```bash
# Set log level to debug
curl -X PUT http://localhost:9090/api/v1/config/logging \
  -H "X-Admin-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "level": "debug",
    "format": "json",
    "output": "stdout",
    "components": {
      "router": "debug",
      "plugins": "debug",
      "upstream": "debug"
    }
  }'
```

#### Request Tracing
```bash
# Enable request tracing
curl -X GET http://localhost:9090/api/v1/routes \
  -H "X-Admin-Key: $API_KEY" \
  -H "X-Trace-Request: true"
```

### 10. Health Checks and Monitoring

#### Comprehensive Health Check
```bash
curl -X GET http://localhost:9090/health?detailed=true \
  -H "X-Admin-Key: $API_KEY"
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": 1640995200,
  "components": {
    "api": {"status": "healthy"},
    "storage": {"status": "healthy", "latency": "2ms"},
    "upstreams": {
      "upstream-001": {"status": "healthy", "active_targets": 2}
    },
    "plugins": {
      "rate_limit": {"status": "healthy"},
      "auth": {"status": "healthy"}
    }
  }
}
```

#### Monitor Key Metrics
```bash
# Get Prometheus metrics
curl -X GET http://localhost:9090/metrics | grep stargate_

# Key metrics to monitor:
# - stargate_requests_total
# - stargate_request_duration_seconds
# - stargate_upstream_requests_total
# - stargate_plugin_execution_duration_seconds
```

## Diagnostic Tools

### 1. Configuration Dump
```bash
# Export complete configuration
curl -X GET http://localhost:9090/api/v1/config/dump \
  -H "X-Admin-Key: $API_KEY" > stargate-config-dump.json
```

### 2. Request Flow Analysis
```bash
# Trace request flow through the gateway
curl -X POST http://localhost:9090/debug/trace \
  -H "X-Admin-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "GET",
    "host": "api.example.com",
    "path": "/api/v1/users",
    "headers": {"Authorization": "Bearer token"}
  }'
```

### 3. Performance Profiling
```bash
# Get CPU profile
curl -X GET http://localhost:9090/debug/pprof/profile \
  -H "X-Admin-Key: $API_KEY" > cpu.prof

# Get memory profile
curl -X GET http://localhost:9090/debug/pprof/heap \
  -H "X-Admin-Key: $API_KEY" > mem.prof
```

## Getting Help

### 1. Enable Support Mode
```bash
# Enable detailed logging and diagnostics
curl -X POST http://localhost:9090/api/v1/support/enable \
  -H "X-Admin-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "duration": "1h",
    "include_sensitive": false
  }'
```

### 2. Generate Support Bundle
```bash
# Create support bundle with logs and configuration
curl -X POST http://localhost:9090/api/v1/support/bundle \
  -H "X-Admin-Key: $API_KEY" \
  -o support-bundle.tar.gz
```

### 3. Community Resources
- **Documentation**: https://docs.stargate.io
- **GitHub Issues**: https://github.com/stargate/stargate/issues
- **Community Forum**: https://community.stargate.io
- **Discord**: https://discord.gg/stargate
