# Developer Portal Architecture

## Overview

The Stargate Developer Portal is a web-based interface that provides developers with comprehensive API documentation, interactive testing capabilities, and access management for all APIs registered through the gateway.

## Architecture Principles

### 1. Separation of Concerns
- **Frontend**: Modern React-based SPA for user interface
- **Backend**: RESTful API service for portal-specific functionality
- **Integration**: Seamless integration with existing Stargate Admin API
- **Storage**: Dedicated storage for portal-specific data

### 2. Scalability and Performance
- **Caching**: Multi-level caching for API documentation
- **CDN**: Static asset delivery via CDN
- **Lazy Loading**: Progressive loading of API documentation
- **Real-time Updates**: WebSocket connections for live updates

## System Components

### Frontend Components

#### 1. React Application (`/web/developer-portal/`)
```
src/
├── components/
│   ├── common/           # Reusable UI components
│   ├── layout/           # Layout components
│   ├── api-docs/         # API documentation components
│   ├── testing/          # API testing interface
│   └── auth/             # Authentication components
├── pages/
│   ├── Dashboard.tsx     # Main dashboard
│   ├── ApiExplorer.tsx   # API exploration interface
│   ├── ApiTester.tsx     # Interactive API testing
│   └── Profile.tsx       # User profile management
├── services/
│   ├── api.ts            # API client
│   ├── auth.ts           # Authentication service
│   └── websocket.ts      # Real-time updates
├── hooks/
│   ├── useApi.ts         # API data fetching
│   ├── useAuth.ts        # Authentication state
│   └── useWebSocket.ts   # WebSocket connection
└── utils/
    ├── openapi.ts        # OpenAPI spec parsing
    └── formatter.ts      # Data formatting utilities
```

#### 2. Key Frontend Features
- **Responsive Design**: Mobile-first responsive interface
- **Dark/Light Theme**: User preference-based theming
- **Search & Filter**: Advanced API discovery capabilities
- **Interactive Testing**: In-browser API testing with request/response visualization
- **Code Generation**: Auto-generated code samples in multiple languages
- **Real-time Updates**: Live updates when APIs are modified

### Backend Components

#### 1. Portal API Service (`/internal/portal/`)
```
internal/portal/
├── server.go            # HTTP server setup
├── handlers/
│   ├── api_docs.go      # API documentation endpoints
│   ├── testing.go       # API testing proxy endpoints
│   ├── auth.go          # Portal authentication
│   └── websocket.go     # Real-time update handlers
├── services/
│   ├── doc_fetcher.go   # OpenAPI spec fetching service
│   ├── doc_parser.go    # OpenAPI spec parsing and validation
│   ├── cache.go         # Documentation caching service
│   └── proxy.go         # API testing proxy service
├── models/
│   ├── api_spec.go      # API specification models
│   ├── user.go          # Portal user models
│   └── test_session.go  # API testing session models
└── storage/
    ├── spec_store.go    # API specification storage
    └── user_store.go    # User data storage
```

#### 2. Key Backend Features
- **OpenAPI Spec Management**: Automatic fetching and parsing of OpenAPI specifications
- **Caching Layer**: Redis-based caching for performance
- **API Testing Proxy**: Secure proxy for API testing with authentication
- **User Management**: Portal-specific user authentication and authorization
- **Real-time Notifications**: WebSocket-based live updates

### Data Storage

#### 1. Primary Storage (etcd)
- **Route Configurations**: Extended with OpenAPI spec URLs
- **API Specifications**: Cached parsed OpenAPI specs
- **Portal Settings**: Global portal configuration

#### 2. Cache Layer (Redis)
- **Parsed API Docs**: Processed OpenAPI specifications
- **User Sessions**: Portal user session data
- **Search Indices**: Pre-built search indices for fast API discovery

#### 3. File Storage (Optional)
- **Static Assets**: Portal static files and resources
- **Generated Code**: Cached code generation results

## Data Flow

### 1. API Documentation Discovery Flow
```
Route Registration → OpenAPI URL Discovery → Spec Fetching → 
Parsing & Validation → Cache Storage → Frontend Display
```

### 2. API Testing Flow
```
User Request → Authentication → Proxy Service → 
Target API → Response Processing → Frontend Display
```

### 3. Real-time Update Flow
```
Configuration Change → Event Notification → WebSocket Broadcast → 
Frontend Update → Cache Invalidation → Re-fetch if needed
```

## Integration Points

### 1. Stargate Admin API Integration
- **Route Management**: Read route configurations with OpenAPI URLs
- **Authentication**: Leverage existing JWT/API Key authentication
- **Configuration Updates**: Listen for configuration changes via WebSocket

### 2. External API Integration
- **OpenAPI Spec Fetching**: HTTP client for fetching OpenAPI specifications
- **API Testing**: Proxy requests to registered APIs
- **Authentication Forwarding**: Forward user credentials to target APIs

## Security Considerations

### 1. Authentication & Authorization
- **Portal Access**: Separate authentication for portal users
- **API Access**: Secure proxy with credential forwarding
- **RBAC**: Role-based access control for different API groups

### 2. Data Protection
- **Sensitive Data Filtering**: Remove sensitive information from displayed specs
- **Request Sanitization**: Sanitize API testing requests
- **CORS Configuration**: Proper CORS setup for cross-origin requests

## Technology Stack

### Frontend
- **Framework**: React 18 with TypeScript
- **State Management**: Zustand for global state
- **UI Library**: Ant Design or Material-UI
- **HTTP Client**: Axios with interceptors
- **WebSocket**: Socket.io-client
- **Build Tool**: Vite for fast development and building

### Backend
- **Language**: Go 1.21+
- **HTTP Framework**: Standard net/http with custom routing
- **WebSocket**: Gorilla WebSocket
- **Caching**: Redis client
- **OpenAPI**: go-openapi libraries for spec parsing
- **Testing**: Built-in testing framework

### Infrastructure
- **Reverse Proxy**: Nginx for static file serving
- **Caching**: Redis for application caching
- **Monitoring**: Prometheus metrics integration
- **Logging**: Structured logging with logrus

## Deployment Architecture

### 1. Development Environment
```
Developer Portal (React Dev Server) → Portal API (Go) → 
Stargate Controller → etcd → Redis
```

### 2. Production Environment
```
Load Balancer → Nginx → Portal API → Stargate Controller
                  ↓
              Static Files (React Build)
                  ↓
              CDN (Optional)
```

## Performance Considerations

### 1. Caching Strategy
- **L1 Cache**: In-memory caching for frequently accessed data
- **L2 Cache**: Redis for shared caching across instances
- **L3 Cache**: CDN for static assets

### 2. Optimization Techniques
- **Code Splitting**: Lazy loading of route-based components
- **API Pagination**: Paginated API listing for large numbers of APIs
- **Search Indexing**: Pre-built search indices for fast API discovery
- **Compression**: Gzip compression for API responses

## Scalability Plan

### 1. Horizontal Scaling
- **Stateless Design**: Portal API designed to be stateless
- **Load Balancing**: Multiple portal API instances behind load balancer
- **Shared Cache**: Redis cluster for shared caching

### 2. Vertical Scaling
- **Resource Optimization**: Efficient memory and CPU usage
- **Connection Pooling**: Database and cache connection pooling
- **Async Processing**: Background processing for heavy operations

This architecture provides a solid foundation for building a comprehensive developer portal that integrates seamlessly with the existing Stargate ecosystem while providing excellent user experience and performance.
