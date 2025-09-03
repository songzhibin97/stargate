# Stargate - 高性能API网关

<div align="center">

![Stargate Logo](https://img.shields.io/badge/Stargate-API%20Gateway-blue?style=for-the-badge)

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen?style=flat-square)](https://github.com/songzhibin97/stargate)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-yellow?style=flat-square)](https://github.com/songzhibin97/stargate)

[English](README.md) | [中文](README_zh.md)

</div>

---

## 项目简介

Stargate 是一个现代化的高性能 API 网关，专为云原生环境设计。它提供了完整的 API 管理解决方案，包括路由管理、负载均衡、流量治理、认证授权、监控观测等核心功能。

## 核心特性

- **高性能**：基于 Go 语言开发，支持高并发请求处理
- **智能路由**：支持多维度路由匹配（主机名、路径、方法、请求头）
- **负载均衡**：支持多种负载均衡算法（轮询、加权、IP哈希、最少连接）
- **安全认证**：支持 API Key、JWT、OAuth2 等多种认证方式
- **流量治理**：限流、熔断、流量镜像等流量管控功能
- **健康检查**：主动和被动健康检查，自动故障转移
- **可观测性**：完整的监控指标、日志记录、链路追踪
- **开发者门户**：友好的 Web 界面，API 文档和测试工具
- **易于配置**：支持 YAML/JSON 配置，动态配置更新
- **云原生**：支持 Docker、Kubernetes 部署

## 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client Apps   │    │  Developer      │    │   Admin Panel   │
│                 │    │  Portal         │    │                 │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          │              ┌───────▼───────┐              │
          │              │               │              │
          └──────────────►   Stargate    ◄──────────────┘
                         │   Gateway     │
                         └───────┬───────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
            ┌───────▼───────┐ ┌──▼──┐ ┌──────▼──────┐
            │   Service A   │ │ ... │ │  Service N  │
            │               │ │     │ │             │
            └───────────────┘ └─────┘ └─────────────┘
```

## 快速开始

### 环境要求

- Go 1.21 或更高版本
- Node.js 18+ (用于前端开发)
- Redis (可选，用于分布式限流)
- PostgreSQL/MySQL (可选，用于配置存储)

### 安装部署

**1. 克隆项目**
```bash
git clone https://github.com/songzhibin97/stargate.git
cd stargate
```

**2. 构建后端**
```bash
# 安装依赖
go mod tidy

# 构建二进制文件
make build

# 或者直接运行
go run cmd/stargate/main.go
```

**3. 构建前端**
```bash
cd web/developer-portal
npm install
npm run build
```

**4. 启动服务**
```bash
# 使用默认配置启动
./bin/stargate

# 或指定配置文件
./bin/stargate -config config/stargate.yaml
```

**5. 验证安装**
```bash
# 检查服务状态
curl http://localhost:8080/health

# 访问开发者门户
open http://localhost:8080
```

### Docker 部署

```bash
# 构建镜像
docker build -t stargate:latest .

# 运行容器
docker run -d \
  --name stargate \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/config:/app/config \
  stargate:latest
```

### Kubernetes 部署

```bash
# 应用 Kubernetes 配置
kubectl apply -f deployments/kubernetes/

# 检查部署状态
kubectl get pods -l app=stargate
```

## 基本使用

### 1. 配置上游服务

```yaml
# config/stargate.yaml
upstreams:
  - id: "user-service"
    name: "用户服务"
    algorithm: "round_robin"
    targets:
      - url: "http://user-service-1:8080"
        weight: 100
      - url: "http://user-service-2:8080"
        weight: 100
    health_check:
      enabled: true
      path: "/health"
      interval: 30
```

### 2. 配置路由规则

```yaml
routes:
  - id: "user-api"
    name: "用户API"
    rules:
      hosts:
        - "api.example.com"
      paths:
        - type: "prefix"
          value: "/api/v1/users"
      methods:
        - "GET"
        - "POST"
    upstream_id: "user-service"
    priority: 100
```

### 3. 启用认证

```yaml
auth:
  enabled: true
  methods:
    jwt:
      enabled: true
      secret: "your-jwt-secret"
      algorithm: "HS256"
```

### 4. 配置限流

```yaml
rate_limit:
  enabled: true
  per_ip:
    requests_per_minute: 60
    burst: 10
```

## 配置说明

详细的配置说明请参考：
- [功能文档](docs/功能文档.md) - 各功能模块详细说明
- [API 文档](docs/api-documentation.md) - REST API 接口文档
- [配置示例](config/) - 完整配置示例

## 监控和观测

Stargate 提供完整的可观测性支持：

- **指标监控**：Prometheus 格式指标，支持 Grafana 可视化
- **日志记录**：结构化日志，支持多种输出格式
- **链路追踪**：支持 Jaeger、Zipkin 等追踪系统
- **健康检查**：内置健康检查端点

```bash
# 查看指标
curl http://localhost:9090/metrics

# 查看健康状态
curl http://localhost:8080/health
```

## 测试

```bash
# 运行单元测试
make test

# 运行集成测试
make test-integration

# 运行性能测试
make test-performance

# 查看测试覆盖率
make test-coverage
```

## 文档

- [快速入门](docs/api-quickstart.md) - 5分钟快速上手
- [架构设计](docs/architecture.md) - 系统架构说明
- [功能文档](docs/功能文档.md) - 功能模块详解
- [API 文档](docs/api-documentation.md) - REST API 参考
- [故障排除](docs/api-troubleshooting.md) - 常见问题解决
- [SDK 文档](docs/sdk-documentation.md) - 多语言 SDK

## 贡献指南

我们欢迎社区贡献！请参考 [贡献指南](CONTRIBUTING.md) 了解如何参与项目开发。

### 开发环境设置

```bash
# 克隆项目
git clone https://github.com/songzhibin97/stargate.git
cd stargate

# 安装开发依赖
make dev-setup

# 运行开发服务器
make dev

# 运行测试
make test
```

## 许可证

本项目采用 [MIT 许可证](LICENSE)。

## 致谢

感谢所有贡献者和开源社区的支持！

---

<div align="center">

**如果这个项目对你有帮助，请给我们一个 Star！**

</div>
