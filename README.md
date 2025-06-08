# Docker Compose Wrapper

A wrapper for Docker Compose that adds support for:
- Rolling updates
- Environment-specific configurations
- Template-based configuration
- Values management

## Installation

```bash
go install github.com/your-server-support/docker-compose-wrapper/cmd/compose-wrapper@latest
```

## Usage

The wrapper provides a command-line interface similar to Docker Compose, with additional features:

```bash
# Start services
dcw up

# Start services with specific environment
dcw up -e prod

# Start specific service
dcw up web

# Start specific service with environment
dcw up -e prod web

# Rolling update for a service
dcw rolling-update web

# Rolling update with specific environment
dcw rolling-update -e prod web

# Rolling update with custom configuration
dcw rolling-update --replicas 3 --retry-count 10 --retry-interval 30 web
```

## Configuration

### Environment-specific Configuration

Create environment-specific configuration files in the `environments` directory:

```yaml
# environments/prod.yaml
global:
  projectName: myapp
  environment: production

services:
  web:
    image: myapp/web:latest
    replicas: 3
    rollingUpdate:
      replicas: 3
      retryCount: 10
      retryInterval: 30
```

### Template-based Configuration

Use Go templates in your configuration files:

```yaml
# docker-compose.yaml
services:
  web:
    image: {{ .Values.services.web.image }}
    ports:
      - "{{ .Values.services.web.port }}:80"
```

### Values Management

Values can be defined in multiple places with the following precedence (highest to lowest):

1. Command-line arguments
2. Environment-specific configuration
3. Default values

Example values file:

```yaml
# values.yaml
global:
  projectName: myapp
  environment: development

services:
  web:
    image: myapp/web:dev
    port: 8080
    replicas: 1
    rollingUpdate:
      replicas: 2
      retryCount: 5
      retryInterval: 10
```

## Rolling Updates

The rolling update feature ensures zero-downtime deployments by:

1. Scaling up the service to double the desired replicas
2. Waiting for new containers to start
3. Removing old containers
4. Scaling back down to the desired number of replicas

Configuration options:
- `replicas`: Number of replicas to maintain
- `retryCount`: Number of attempts to wait for new containers
- `retryInterval`: Time between retry attempts in seconds

## Development

### Building

```bash
go build -o dcw cmd/compose-wrapper/main.go
```

### Testing

```bash
go test ./...
```

## License

MIT 

## Features

- **Template-based Docker Compose configuration** using Go templates
- **Versioned releases**: Each configuration generation is saved as a new version in `dist/`
- **Automatic config hashing**: Output directory includes a hash of the config for traceability
- **Configurable release retention**: Control how many releases to keep (default: 20)
- **Rollback**: Instantly roll back to any previous release, or the previous one by default
- **Releases listing**: See all available releases and their timestamps
- **Lint**: Validate all generated Docker Compose files using `docker compose config`
- **Values file management** with override and priority support
- **Dependency management** between services (charts)
- **Automated Docker network management**
- **Transparent Docker Compose command passing**
- **Configuration validation (lint)**
- **Pre and post hooks** for running commands or containers
- **Rolling updates** with zero-downtime deployment support
- **Service-specific configuration** for replicas and update strategies

## Directory Structure

```
/chart-example
|-- Chart.yaml                # Main chart description and dependencies
|-- values.yaml               # Main values file
|-- /templates                # Main chart templates (Go templates)
|   |-- docker-compose.yml.tmpl
|-- /charts                   # Child charts directory
|   |-- /database
|   |   |-- Chart.yaml
|   |   |-- values.yaml
|   |   |-- templates/
|   |   |   |-- docker-compose.yml.tmpl
|   |-- /cache
|       |-- Chart.yaml
|       |-- values.yaml
|       |-- templates/
|           |-- docker-compose.yml.tmpl
|-- /dist                     # All generated releases
|   |-- v1-<hash>/
|   |   |-- values.yaml       # The merged config for this release
|   |   |-- docker/
|   |   |   |-- docker-compose.yml
|   |   |   |-- database/
|   |   |   |   |-- docker-compose.yml
|   |   |   |-- cache/
|   |   |   |   |-- docker-compose.yml
|   |-- v2-<hash>/
|   |   |-- ...
```

## Template Naming Convention

All template files follow the naming convention:
```
<filename>.<extension>.tmpl
```

For example:
- `docker-compose.yml.tmpl`
- `config.json.tmpl`
- `nginx.conf.tmpl`

This convention makes it clear which files are templates and what their final output format will be.

## Value Precedence

1. `--set` and `--set-file` (highest priority)
2. Additional values files (`-f`)
3. Main chart values (`values.yaml`)
4. Child chart values (lowest, used as base)

> **Note:** The flags `--set`, `--set-file`, `--set-string`, `-f`, and `--values` are only interpreted by the wrapper for value merging and are **not** passed to Docker Compose itself.

## Commands

### Generate/Up (default)
Generates a new release if the config changes, or reuses the latest if not. Runs Docker Compose with the generated files.

```
dcw up -d
```

> **Note:** Any value-related flags (`--set`, `--set-file`, `--set-string`, `-f`, `--values`) are handled by the wrapper and will not be forwarded to Docker Compose.

### Lint
Validates all generated Docker Compose files using `docker compose config`.

```
dcw lint
```

### Releases
Lists all available releases with their timestamps.

```
dcw releases
```

### Rollback
Creates a new release from a previous one and runs Docker Compose from it. Supports rolling updates if configured in the target release.

- Roll back to the previous release:
  ```
  dcw rollback up -d
  ```
- Roll back to a specific release:
  ```
  dcw rollback v3-abcdef12 up -d
  ```

When rolling back, the wrapper will:
1. Create a new release from the selected version
2. Preserve all configuration including rolling update settings
3. Apply rolling updates if enabled in the target release's configuration
4. Use the same zero-downtime update process as regular deployments

## Output Example

After each command, you will see a summary:

```
+++++++++++++++++++++++++++++++++++++++
Release:  v5-abcdef12
Status:   [32mSUCCESS[0m
+++++++++++++++++++++++++++++++++++++++
```

or, if there was an error:

```
+++++++++++++++++++++++++++++++++++++++
Release:  v5-abcdef12
Status:   [31mFAIL!!!![0m
+++++++++++++++++++++++++++++++++++++++
```

When rolling back, you will see:

```
[33mNew state version v6-12345678 created from release v5-abcdef12[0m
```

## Template Syntax

- Uses Go's `text/template` syntax.
- Supports all Go template features: `{{ .key }}`, `{{ if ... }}`, `{{ range ... }}`.

## Example: Main Compose Template

```
version: '3.9'

services:
  web:
    image: {{ .image.repository }}:{{ .image.tag }}
    ports:
      - "{{ .appPort }}:8080"
    environment:
      - ENVIRONMENT={{ .global.environment }}
      - DB_HOST=database
      - REDIS_HOST=cache
    {{- if .global.network.alias }}
    networks:
      - {{ .global.network.alias }}
    {{- end }}

{{- if .global.network.alias }}
networks:
  {{ .global.network.alias }}:
    driver: {{ .global.network.driver }}
    name: {{ .global.network.name }}
{{- end }}
```

## How it works

- The wrapper parses and merges values from all supported sources (`--set`, `--set-file`, `--set-string`, `-f`, `--values`, chart defaults).
- These flags are **not** passed to Docker Compose. Only arguments relevant to Docker Compose are forwarded.
- This ensures Docker Compose receives only valid arguments, while the wrapper manages all configuration logic.

## Docker Compose Integration

The wrapper uses Docker Compose's ability to work with multiple compose files through the `COMPOSE_FILE` environment variable. For example:

```
COMPOSE_FILE=docker-compose.yml:cache/docker-compose.yml:database/docker-compose.yml
```

This allows:
- Each service to have its own compose file
- Services to be organized in subdirectories
- Easy addition of new services without modifying existing files
- Clear separation of concerns between different services

## License

MIT 

## Chart Dependencies

Charts can depend on other charts from Git repositories or Helm-like repositories. Dependencies are declared in the `Chart.yaml` file:

```yaml
dependencies:
  - name: database
    repository: https://github.com/your-org/database-chart.git
    version: main
  - name: cache
    repository: https://charts.your-org.com
    version: 1.2.3
  - name: web2
    path: ./charts/web2
```

### Repository Types

1. **Git Repository**
   - URL format: `https://github.com/org/repo.git` or `git@github.com:org/repo.git`
   - Version can be a branch name, tag, or commit hash
   - The repository must contain a valid chart structure

2. **Helm-like Repository**
   - URL format: `https://charts.your-org.com`
   - Version must be a semantic version (e.g., `1.2.3`)
   - Repository must provide an `index.yaml` file

### Managing Dependencies

To update dependencies:

```bash
dcw dependency update
```

To list current dependencies:

```bash
dcw dependency list
```

Dependencies are stored in the `charts/` directory and are automatically downloaded when needed.

## Hooks

Hooks allow you to run commands or containers before or after Docker Compose operations. They are defined in `Chart.yaml`:

```yaml
hooks:
  - name: wait-for-db
    type: pre
    command: ["./scripts/wait-for-db.sh"]
  - name: backup
    type: post
    container:
      image: backup-tool:latest
      command: ["backup", "--target", "database"]
```

### Hook Types

1. **Pre-hooks**: Run before Docker Compose operations
2. **Post-hooks**: Run after Docker Compose operations

### Hook Formats

1. **Command Hooks**: Run shell commands
   ```yaml
   hooks:
     - name: setup
       type: pre
       command: ["./scripts/setup.sh"]
   ```

2. **Container Hooks**: Run containers
   ```yaml
   hooks:
     - name: backup
       type: post
       container:
         image: backup-tool:latest
         command: ["backup"]
         env:
           BACKUP_PATH: "/data"
   ```

### Hook Features

- **Wait for Services**: Hooks can wait for services to be ready
  ```yaml
  hooks:
    - name: wait-for-db
      type: pre
      waitFor: ["database"]
      timeout: "30s"
  ```

- **Environment Variables**: Pass environment variables to hooks
  ```yaml
   hooks:
     - name: setup
       type: pre
       command: ["./scripts/setup.sh"]
       env:
         DB_HOST: "database"
         DB_PORT: "5432"
   ```

- **Network Access**: Container hooks can access the same network as your services
  ```yaml
   hooks:
     - name: backup
       type: post
       container:
         image: backup-tool:latest
         network: "appnet"
   ```

## Chart Configuration

The `Chart.yaml` file supports the following configuration options:

```yaml
name: example
version: 1.0.0
maxReleases: 10  # Optional: Maximum number of releases to keep (default: 20)

dependencies:
  - name: database
    repository: https://github.com/your-org/database-chart.git
    version: main
  - name: local-service
    path: ./local-charts/service  # Path to local chart

hooks:
  - name: init-db
    type: pre
    container:
      image: postgres:14
      command: ["psql"]
      args: ["-h", "database", "-U", "postgres", "-f", "/docker-entrypoint-initdb.d/init.sql"]
      env:
        PGPASSWORD: "postgres"
      network: "my-network"
    waitFor:
      - database
    timeout: "30s"
```

### Configuration Options

- `name`: Chart name
- `version`: Chart version
- `maxReleases`: Maximum number of releases to keep (default: 20)
- `dependencies`: List of chart dependencies
  - `name`: Dependency name
  - `repository`: Git repository URL or Helm repository (optional for local charts)
  - `version`: Git branch/tag or Helm chart version (optional for local charts)
  - `path`: Path to local chart directory (relative to Chart.yaml)
- `hooks`: List of pre and post hooks 

## Logging

This project uses Go's `log/slog` for structured logging. You can control the verbosity of logs using the `LOG_LEVEL` environment variable:

- `debug` — show debug, info, warning, and error messages
- `info`  — show info, warning, and error messages (default)
- `warn`  — show warning and error messages
- `error` — show only error messages

Example usage:

```bash
LOG_LEVEL=debug ./compose-wrapper up
```

All debug and info messages (such as dependency updates, hook execution, and internal steps) are logged via slog. Only the release status and summary are printed to the console via `fmt` for clear user feedback.

## Rolling Updates

The wrapper supports zero-downtime rolling updates for services. This is configured in the `values.yaml` file:

```yaml
# For main service
appName: "web"  # Must match the service name in docker-compose.yml
rolling-update: true
replicas: 2

# For other services
web2:
  rolling-update: true
  replicas: 3
```

### How Rolling Updates Work

1. The service is scaled up to double the desired replicas
2. New containers are started with the updated configuration
3. Old containers are gracefully terminated (SIGTERM)
4. The service is scaled back to the desired number of replicas

### Container Name Matching

The wrapper uses exact container name matching to prevent unintended container updates. This means:
- Services with similar names (e.g., "web" and "web2") are updated independently
- Each service's containers are identified by their exact name
- No risk of accidentally updating containers from other services

### Rolling Update Configuration

You can configure rolling updates at two levels:

1. **Root Level** (applies to main service):
   ```yaml
   appName: "web"
   rolling-update: true
   replicas: 2
   ```

2. **Service Level** (applies to specific services):
   ```yaml
   web2:
     rolling-update: true
     replicas: 3
   ```

The `appName` field in the root configuration determines which service is considered the main service. This service will use the root-level rolling update configuration. The value of `appName` and service name in docker-compose.yml.tmpl from root chart must be same.

## Rolling Update Configuration

Rolling updates can be configured at both global and service levels:

```yaml
# Global configuration (applies to main service)
rolling-update: true
replicas: 2

# Service-specific configuration
web2:
  rolling-update: true
  replicas: 1
```

### Rolling Update Behavior

1. **Pre-update Check**:
   - Verifies current replica count
   - Scales to configured replica count if needed

2. **Update Process**:
   - Scales up to double the configured replicas
   - Waits for new containers to start (configurable retries)
   - Gracefully terminates old containers
   - Scales back to original replica count

3. **Configuration Options**:
   ```go
   RollingUpdateRetryCount    = 5    // Number of retries to wait for new containers
   RollingUpdateRetryInterval = 5    // Seconds to wait between retries
   ```
