# Stargate Documentation

Welcome to the comprehensive documentation for Stargate, a high-performance API gateway with advanced routing, load balancing, plugin system, and developer portal functionality.

## Documentation Overview

### Getting Started
- **[Quick Start Guide](api-quickstart.md)** - Get up and running with Stargate in minutes
- **[Installation Guide](installation.md)** - Complete installation and setup instructions
- **[Configuration Guide](configuration.md)** - Detailed configuration options

### API Documentation
- **[API Documentation](api-documentation.md)** - Complete REST API reference
- **[API Examples](api-examples.md)** - Practical examples and use cases
- **[OpenAPI Specification](openapi.yaml)** - Machine-readable API specification
- **[Troubleshooting Guide](api-troubleshooting.md)** - Common issues and solutions

### SDK and Integration
- **[SDK Documentation](sdk-documentation.md)** - Official SDKs for multiple languages
- **[Integration Examples](integration-examples.md)** - Integration patterns and examples
- **[Webhook Guide](webhook-guide.md)** - Setting up and using webhooks

### Developer Portal
- **[Portal Architecture](developer-portal-architecture.md)** - Portal system design
- **[Portal API Guide](portal-api-guide.md)** - Portal-specific API endpoints
- **[Portal Customization](portal-customization.md)** - Customizing the developer portal

### Advanced Topics
- **[Plugin Development](plugin-development.md)** - Creating custom plugins
- **[Performance Tuning](performance-tuning.md)** - Optimization best practices
- **[Security Guide](security-guide.md)** - Security configuration and best practices
- **[Monitoring and Observability](monitoring.md)** - Metrics, logging, and monitoring

## Quick Navigation

### For Developers
If you're a developer looking to integrate with Stargate:

1. Start with the **[Quick Start Guide](api-quickstart.md)**
2. Explore **[API Examples](api-examples.md)** for common use cases
3. Use the **[SDK Documentation](sdk-documentation.md)** for your preferred language
4. Reference the **[API Documentation](api-documentation.md)** for detailed endpoint information

### For DevOps/Platform Engineers
If you're setting up and managing Stargate:

1. Follow the **[Installation Guide](installation.md)**
2. Review **[Configuration Guide](configuration.md)** for setup options
3. Implement **[Monitoring and Observability](monitoring.md)**
4. Study **[Performance Tuning](performance-tuning.md)** for optimization
5. Secure your setup with the **[Security Guide](security-guide.md)**

### For API Consumers
If you're using APIs through the Stargate portal:

1. Access the **[Developer Portal](http://localhost:8080)** (when running)
2. Read the **[Portal API Guide](portal-api-guide.md)**
3. Check **[Integration Examples](integration-examples.md)** for your use case

## 📖 API Reference Quick Links

### Core Endpoints
- **Health Check**: `GET /health`
- **Authentication**: `POST /auth/login`
- **Routes**: `GET|POST /api/v1/routes`
- **Upstreams**: `GET|POST /api/v1/upstreams`
- **Plugins**: `GET|POST /api/v1/plugins`

### Portal Endpoints
- **API Discovery**: `GET /api/v1/portal/apis`
- **API Testing**: `POST /api/v1/portal/test`
- **Dashboard**: `GET /api/v1/portal/dashboard`

### Authentication Methods
- **API Key**: `X-Admin-Key: your-api-key`
- **JWT Bearer**: `Authorization: Bearer your-jwt-token`

## Configuration Examples

### Basic Route Configuration
```json
{
  "name": "API Route",
  "rules": {
    "hosts": ["api.example.com"],
    "paths": [{"type": "prefix", "value": "/api"}],
    "methods": ["GET", "POST"]
  },
  "upstream_id": "backend-service"
}
```

### Load Balancer Setup
```json
{
  "name": "Backend Service",
  "targets": [
    {"url": "http://backend1:8080", "weight": 100},
    {"url": "http://backend2:8080", "weight": 100}
  ],
  "algorithm": "round_robin"
}
```

### Rate Limiting Plugin
```json
{
  "name": "Rate Limiter",
  "type": "rate_limit",
  "config": {
    "requests_per_minute": 1000,
    "burst": 50
  }
}
```

## 🛠️ SDK Quick Examples

### JavaScript/Node.js
```javascript
import { StargateClient } from '@stargate/sdk';

const client = new StargateClient({
  baseURL: 'http://localhost:9090',
  apiKey: 'your-api-key'
});

const routes = await client.routes.list();
```

### Python
```python
from stargate_sdk import StargateClient

client = StargateClient(
    base_url='http://localhost:9090',
    api_key='your-api-key'
)

routes = client.routes.list()
```

### Go
```go
import "github.com/stargate/go-sdk/client"

cfg := client.Config{
    BaseURL: "http://localhost:9090",
    APIKey:  "your-api-key",
}

c, err := client.New(cfg)
routes, err := c.Routes.List(ctx, nil)
```

### cURL
```bash
curl -H "X-Admin-Key: your-api-key" \
  http://localhost:9090/api/v1/routes
```

## 🔍 Common Use Cases

### 1. API Gateway Setup
- [Basic API Gateway](api-examples.md#1-setting-up-a-basic-api-gateway)
- [Multi-Service Routing](api-examples.md#2-advanced-route-configuration)
- [Load Balancing](api-examples.md#3-load-balancing-configurations)

### 2. Security and Authentication
- [JWT Authentication](api-examples.md#4-plugin-configurations)
- [API Key Management](api-examples.md#14-advanced-authentication)
- [Rate Limiting](api-examples.md#4-plugin-configurations)

### 3. Developer Portal
- [API Documentation](portal-api-guide.md)
- [Interactive Testing](portal-api-guide.md#api-testing)
- [Application Management](api-documentation.md#application-management)

### 4. Monitoring and Observability
- [Metrics Collection](monitoring.md#metrics)
- [Health Checks](api-documentation.md#health--status)
- [Logging Configuration](monitoring.md#logging)

## 🐛 Troubleshooting

### Common Issues
- **[Route Not Matching](api-troubleshooting.md#2-route-configuration-issues)**
- **[502 Bad Gateway](api-troubleshooting.md#3-upstream-connection-issues)**
- **[Authentication Failures](api-troubleshooting.md#1-authentication-issues)**
- **[Plugin Issues](api-troubleshooting.md#4-plugin-issues)**

### Debug Tools
- **Health Check**: `GET /health?detailed=true`
- **Configuration Dump**: `GET /api/v1/config/dump`
- **Request Tracing**: `POST /debug/trace`

## Performance and Scaling

### Key Metrics to Monitor
- Request throughput (requests/second)
- Response latency (p50, p95, p99)
- Error rates by status code
- Upstream health status
- Plugin execution time

### Optimization Tips
- Use connection pooling for high throughput
- Configure appropriate timeouts
- Enable caching where possible
- Monitor resource usage
- Scale horizontally when needed

## 🔐 Security Best Practices

### Authentication
- Use strong API keys (minimum 32 characters)
- Rotate API keys regularly
- Implement JWT with proper expiration
- Use HTTPS in production

### Network Security
- Configure proper CORS policies
- Implement rate limiting
- Use IP whitelisting when appropriate
- Enable request/response logging for audit

### Data Protection
- Sanitize sensitive data in logs
- Use secure headers
- Implement proper error handling
- Regular security audits

## Contributing

We welcome contributions to Stargate! Here's how you can help:

### Documentation
- Fix typos and improve clarity
- Add more examples and use cases
- Translate documentation
- Create video tutorials

### Code Contributions
- Bug fixes and improvements
- New plugin development
- SDK enhancements
- Performance optimizations

### Community
- Answer questions in forums
- Share your use cases
- Write blog posts
- Speak at conferences

## 📞 Support and Community

### Getting Help
- **Documentation**: https://docs.stargate.io
- **GitHub Issues**: https://github.com/stargate/stargate/issues
- **Community Forum**: https://community.stargate.io
- **Discord**: https://discord.gg/stargate
- **Stack Overflow**: Tag `stargate-api`

### Commercial Support
For enterprise support, training, and consulting:
- **Email**: enterprise@stargate.io
- **Website**: https://stargate.io/enterprise

## 📋 Changelog and Releases

### Latest Release: v1.0.0
- Complete API gateway functionality
- Developer portal with interactive documentation
- Plugin system with built-in plugins
- Multi-language SDK support
- Comprehensive monitoring and observability

### Upcoming Features
- GraphQL support
- Advanced caching mechanisms
- Service mesh integration
- Enhanced security features
- Performance improvements

## License

Stargate is released under the [MIT License](../LICENSE).

## Acknowledgments

Special thanks to all contributors, early adopters, and the open-source community for making Stargate possible.

---

**Need help?** Don't hesitate to reach out through any of our support channels. We're here to help you succeed with Stargate!

**Found an issue with the documentation?** Please [open an issue](https://github.com/stargate/stargate/issues/new) or submit a pull request.
