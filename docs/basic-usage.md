---
layout: default
title: Basic Usage
---

# Basic Usage Guide

This guide covers the basic usage of the Docker Compose Wrapper.

## Common Commands

### Starting Services

```bash
# Start all services
dcw up

# Start services in detached mode
dcw up -d

# Start specific service
dcw up web

# Start with specific environment
dcw up -e prod

# Start with environment and service
dcw up -e prod web
```

### Stopping Services

```bash
# Stop all services
dcw down

# Stop specific service
dcw down web

# Stop and remove volumes
dcw down -v
```

### Service Management

```bash
# List running services
dcw ps

# View service logs
dcw logs web

# Follow service logs
dcw logs -f web

# Restart service
dcw restart web

# Scale service
dcw scale web=3
```

### Rolling Updates

```bash
# Perform rolling update
dcw rolling-update web

# Rolling update with environment
dcw rolling-update -e prod web

# Custom rolling update
dcw rolling-update --replicas 3 --retry-count 10 web
```

## Project Structure

A typical project structure looks like this:

```
myapp/
├── Chart.yaml              # Chart metadata
├── values.yaml            # Default values
├── environments/          # Environment-specific values
│   ├── dev.yaml
│   ├── staging.yaml
│   └── prod.yaml
├── templates/             # Template files
│   ├── docker-compose.yaml
│   └── _helpers.tpl
└── releases/             # Release history
    └── v1/
        ├── values.yaml
        └── docker-compose.yaml
```

## Basic Configuration

### 1. Chart.yaml

```yaml
name: myapp
version: 1.0.0
description: My Application Chart
type: application
```

### 2. values.yaml

```yaml
global:
  projectName: myapp
  environment: development

services:
  web:
    image: myapp/web:latest
    replicas: 1
    ports:
      - "8080:80"
```

### 3. docker-compose.yaml

```yaml
version: '3.8'
services:
  web:
    image: {{ .Values.services.web.image }}
    ports:
      - "{{ .Values.services.web.ports[0] }}"
```

## Environment Management

### Switching Environments

```bash
# Development
dcw up -e dev

# Staging
dcw up -e staging

# Production
dcw up -e prod
```

### Environment-specific Values

```yaml
# environments/prod.yaml
global:
  environment: production

services:
  web:
    image: myapp/web:prod
    replicas: 3
```

## Common Workflows

### 1. Development Workflow

```bash
# Start development environment
dcw up -e dev

# Make changes to configuration
vim values.yaml

# Apply changes
dcw up -d

# View logs
dcw logs -f web
```

### 2. Testing Workflow

```bash
# Start test environment
dcw up -e test

# Run tests
dcw exec web npm test

# Check test results
dcw logs web
```

### 3. Deployment Workflow

```bash
# Deploy to staging
dcw up -e staging

# Verify deployment
dcw ps

# Deploy to production
dcw up -e prod

# Perform rolling update
dcw rolling-update web
```

## Best Practices

1. **Service Management**
   - Use meaningful service names
   - Set appropriate resource limits
   - Configure health checks

2. **Environment Management**
   - Keep environment-specific changes minimal
   - Use environment variables for sensitive data
   - Document environment requirements

3. **Configuration**
   - Use templates for common patterns
   - Keep configuration DRY
   - Validate configuration before deployment

4. **Deployment**
   - Always use rolling updates in production
   - Monitor deployment progress
   - Have a rollback plan ready

## Troubleshooting

### Common Issues

1. **Service Won't Start**
   ```bash
   # Check service status
   dcw ps
   
   # View service logs
   dcw logs web
   
   # Check configuration
   dcw config
   ```

2. **Configuration Errors**
   ```bash
   # Validate configuration
   dcw config
   
   # Check environment variables
   dcw config --environment prod
   ```

3. **Network Issues**
   ```bash
   # Check network configuration
   docker network ls
   
   # Inspect network
   docker network inspect myapp_default
   ```

## Next Steps

1. [Configuration Guide](configuration)
2. [Rolling Updates](rolling-updates)
3. [Advanced Features](advanced-features) 