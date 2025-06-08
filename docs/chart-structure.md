---
layout: default
title: Chart Structure
---

# Chart Structure

The Docker Compose Wrapper uses a chart-based structure for managing configurations and deployments.

## Chart Directory Structure

```
.
├── Chart.yaml              # Chart metadata
├── values.yaml            # Default values
├── environments/          # Environment-specific values
│   ├── dev.yaml
│   ├── staging.yaml
│   └── prod.yaml
├── templates/             # Template files
│   ├── docker-compose.yaml
│   ├── networks.yaml
│   └── _helpers.tpl
└── releases/             # Release history
    └── v1/
        ├── values.yaml
        └── docker-compose.yaml
```

## Chart.yaml

The main chart metadata file:

```yaml
name: myapp
version: 1.0.0
description: My Application Chart
type: application
```

## Values Structure

### Default Values (values.yaml)

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

  api:
    image: myapp/api:latest
    replicas: 1
    ports:
      - "3000:3000"
    environment:
      - DB_HOST=db
    networks:
      - frontend
      - backend
    rollingUpdate:
      replicas: 2
      retryCount: 5
      retryInterval: 10

  db:
    image: postgres:13
    environment:
      - POSTGRES_DB=myapp
      - POSTGRES_USER=myapp
    volumes:
      - db_data:/var/lib/postgresql/data
    networks:
      - backend

networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge

volumes:
  db_data:
```

### Environment-specific Values

Example for production (environments/prod.yaml):

```yaml
global:
  environment: production

services:
  web:
    image: myapp/web:prod
    replicas: 3
    rollingUpdate:
      replicas: 3
      retryCount: 10
      retryInterval: 30
    environment:
      - NODE_ENV=production

  api:
    image: myapp/api:prod
    replicas: 3
    rollingUpdate:
      replicas: 3
      retryCount: 10
      retryInterval: 30
    environment:
      - NODE_ENV=production
```

## Template Files

### docker-compose.yaml

```yaml
version: '3.8'

services:
  {{- range $name, $service := .Values.services }}
  {{ $name }}:
    image: {{ $service.image }}
    {{- if $service.replicas }}
    deploy:
      replicas: {{ $service.replicas }}
    {{- end }}
    {{- if $service.ports }}
    ports:
      {{- toYaml $service.ports | nindent 6 }}
    {{- end }}
    {{- if $service.environment }}
    environment:
      {{- toYaml $service.environment | nindent 6 }}
    {{- end }}
    {{- if $service.volumes }}
    volumes:
      {{- toYaml $service.volumes | nindent 6 }}
    {{- end }}
    {{- if $service.networks }}
    networks:
      {{- toYaml $service.networks | nindent 6 }}
    {{- end }}
  {{- end }}

networks:
  {{- toYaml .Values.networks | nindent 2 }}

volumes:
  {{- toYaml .Values.volumes | nindent 2 }}
```

### _helpers.tpl

```yaml
{{/*
Expand the name of the chart.
*/}}
{{- define "chart.name" -}}
{{- default .Chart.Name .Values.global.projectName | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "chart.fullname" -}}
{{- if .Values.global.fullnameOverride }}
{{- .Values.global.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.global.projectName }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}
```

## Value Precedence

Values are merged in the following order (highest to lowest precedence):

1. Command-line arguments
2. Environment-specific values
3. Default values from values.yaml
4. Chart defaults

## Release Management

Each release is stored in the `releases` directory with:
- Versioned values
- Rendered templates
- Release metadata

Example release structure:

```
releases/
└── v1/
    ├── values.yaml
    ├── docker-compose.yaml
    └── metadata.yaml
```

## Best Practices

1. **Value Organization**
   - Keep global values at the root level
   - Group service-specific values under `services`
   - Use environment-specific overrides for differences

2. **Template Usage**
   - Use helper templates for common patterns
   - Keep templates DRY (Don't Repeat Yourself)
   - Use conditional rendering for optional features

3. **Environment Management**
   - Keep environment-specific changes minimal
   - Use environment variables for sensitive data
   - Document environment-specific requirements

4. **Release Management**
   - Version all releases
   - Keep release history for rollbacks
   - Document release changes 