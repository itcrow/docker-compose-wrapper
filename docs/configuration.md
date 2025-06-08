---
layout: default
title: Configuration
---

# Configuration Guide

The Docker Compose Wrapper uses a flexible configuration system based on YAML files and environment-specific overrides.

## Configuration Files

### 1. Chart.yaml

The main chart metadata file that defines basic information about your application:

```yaml
name: myapp
version: 1.0.0
description: My Application Chart
type: application
```

### 2. values.yaml

Default configuration values for your application:

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
    environment:
      - NODE_ENV=development
    volumes:
      - ./data:/app/data
    networks:
      - frontend
    rollingUpdate:
      replicas: 2
      retryCount: 5
      retryInterval: 10
```

### 3. Environment-specific Values

Create environment-specific configuration files in the `environments` directory:

```yaml
# environments/prod.yaml
global:
  environment: production

services:
  web:
    image: myapp/web:prod
    replicas: 3
    environment:
      - NODE_ENV=production
```

## Value Precedence

Values are merged in the following order (highest to lowest precedence):

1. Command-line arguments
2. Environment-specific values
3. Default values from values.yaml
4. Chart defaults

## Configuration Structure

### Global Values

Global values apply to all services:

```yaml
global:
  projectName: myapp
  environment: development
  domain: example.com
  registry: docker.io
```

### Service Configuration

Service-specific configuration:

```yaml
services:
  web:
    # Basic configuration
    image: myapp/web:latest
    replicas: 1
    
    # Networking
    ports:
      - "8080:80"
    networks:
      - frontend
    
    # Environment variables
    environment:
      - NODE_ENV=development
      - DB_HOST=db
    
    # Volumes
    volumes:
      - ./data:/app/data
      - config:/app/config
    
    # Rolling update configuration
    rollingUpdate:
      replicas: 2
      retryCount: 5
      retryInterval: 10
```

### Network Configuration

Define custom networks:

```yaml
networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true
```

### Volume Configuration

Define named volumes:

```yaml
volumes:
  db_data:
  config:
```

## Template Usage

Use Go templates in your configuration files:

```yaml
services:
  web:
    image: {{ .Values.global.registry }}/{{ .Values.services.web.image }}
    environment:
      - NODE_ENV={{ .Values.global.environment }}
```

## Best Practices

1. **Value Organization**
   - Keep global values at the root level
   - Group service-specific values under `services`
   - Use environment-specific overrides for differences

2. **Environment Management**
   - Keep environment-specific changes minimal
   - Use environment variables for sensitive data
   - Document environment-specific requirements

3. **Template Usage**
   - Use helper templates for common patterns
   - Keep templates DRY (Don't Repeat Yourself)
   - Use conditional rendering for optional features

4. **Security**
   - Never commit sensitive data to version control
   - Use environment variables for secrets
   - Consider using a secrets management solution

## Common Configuration Patterns

### Development Environment

```yaml
# environments/dev.yaml
global:
  environment: development

services:
  web:
    image: myapp/web:dev
    replicas: 1
    environment:
      - DEBUG=true
```

### Production Environment

```yaml
# environments/prod.yaml
global:
  environment: production

services:
  web:
    image: myapp/web:prod
    replicas: 3
    environment:
      - DEBUG=false
```

### Staging Environment

```yaml
# environments/staging.yaml
global:
  environment: staging

services:
  web:
    image: myapp/web:staging
    replicas: 2
    environment:
      - DEBUG=false
``` 