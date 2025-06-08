---
layout: default
title: Code Structure
---

# Code Structure

The Docker Compose Wrapper is organized into several key components:

## Directory Structure

```
.
├── cmd/
│   └── compose-wrapper/    # Main application entry point
├── internal/
│   ├── app/               # Core application logic
│   ├── chart/             # Chart management
│   ├── template/          # Template processing
│   └── values/            # Values management
├── pkg/
│   └── utils/             # Shared utilities
└── docs/                  # Documentation
```

## Key Components

### Command Line Interface (`cmd/compose-wrapper/`)

The main entry point for the application, handling command-line arguments and routing to appropriate handlers.

```go
// main.go
func main() {
    // Parse command line arguments
    // Initialize application
    // Execute commands
}
```

### Core Application Logic (`internal/app/`)

Contains the main business logic for:
- Service management
- Rolling updates
- Configuration processing
- Command execution

Key files:
- `commands.go`: Command implementations
- `rolling.go`: Rolling update logic
- `config.go`: Configuration management

### Chart Management (`internal/chart/`)

Handles chart-related operations:
- Chart loading and validation
- Template processing
- Values merging
- Release management

```go
// chart.go
type Chart struct {
    Name       string
    Version    string
    Templates  []Template
    Values     map[string]interface{}
}
```

### Template Processing (`internal/template/`)

Manages template rendering and processing:
- Template loading
- Variable substitution
- Environment-specific overrides

```go
// template.go
type Template struct {
    Name     string
    Content  string
    Values   map[string]interface{}
}
```

### Values Management (`internal/values/`)

Handles configuration values:
- Default values
- Environment-specific values
- Value merging and precedence
- Validation

```go
// values.go
type Values struct {
    Global    map[string]interface{}
    Services  map[string]Service
}
```

## Key Interfaces

### Service Interface

```go
type Service interface {
    Start() error
    Stop() error
    Restart() error
    Scale(replicas int) error
    RollingUpdate(config RollingUpdateConfig) error
}
```

### Chart Interface

```go
type Chart interface {
    Load() error
    Validate() error
    Render() (string, error)
    GetValues() map[string]interface{}
}
```

## Error Handling

The application uses a consistent error handling approach:
- Custom error types for different scenarios
- Detailed error messages
- Proper error wrapping
- Context preservation

## Testing

The codebase includes:
- Unit tests for core functionality
- Integration tests for complex operations
- Mock implementations for testing
- Test utilities and helpers 