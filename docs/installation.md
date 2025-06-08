---
layout: default
title: Installation
---

# Installation Guide

This guide will help you install and set up the Docker Compose Wrapper.

## Prerequisites

- Docker Engine (version 20.10.0 or later)
- Docker Compose (version 2.0.0 or later)
- Go (version 1.21 or later)

## Installation Methods

### 1. Using Go Install

The simplest way to install the wrapper:

```bash
go install github.com/your-server-support/docker-compose-wrapper/cmd/compose-wrapper@latest
```

This will install the `dcw` command in your `$GOPATH/bin` directory.

### 2. Building from Source

Clone the repository and build:

```bash
# Clone the repository
git clone https://github.com/your-server-support/docker-compose-wrapper.git
cd docker-compose-wrapper

# Build the binary
go build -o dcw cmd/compose-wrapper/main.go

# Move to a directory in your PATH
sudo mv dcw /usr/local/bin/
```

### 3. Using Package Managers

#### Fedora/RHEL

```bash
# Add the repository
sudo dnf config-manager --add-repo https://your-server-support.github.io/docker-compose-wrapper/fedora/docker-compose-wrapper.repo

# Install the package
sudo dnf install docker-compose-wrapper
```

#### Ubuntu/Debian

```bash
# Add the repository
curl -fsSL https://your-server-support.github.io/docker-compose-wrapper/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-compose-wrapper-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/docker-compose-wrapper-archive-keyring.gpg] https://your-server-support.github.io/docker-compose-wrapper/ubuntu stable main" | sudo tee /etc/apt/sources.list.d/docker-compose-wrapper.list

# Install the package
sudo apt-get update
sudo apt-get install docker-compose-wrapper
```

## Verification

Verify the installation:

```bash
# Check version
dcw version

# Check help
dcw --help
```

## Configuration

### 1. Create Project Structure

```bash
mkdir myapp
cd myapp

# Create basic structure
mkdir -p environments templates
```

### 2. Create Configuration Files

Create `Chart.yaml`:

```yaml
name: myapp
version: 1.0.0
description: My Application Chart
type: application
```

Create `values.yaml`:

```yaml
global:
  projectName: myapp
  environment: development

services:
  web:
    image: myapp/web:latest
    replicas: 1
```

### 3. Environment-specific Configuration

Create `environments/dev.yaml`:

```yaml
global:
  environment: development

services:
  web:
    image: myapp/web:dev
```

## Post-Installation

### 1. Set Up Shell Completion

#### Bash

Add to `~/.bashrc`:

```bash
source <(dcw completion bash)
```

#### Zsh

Add to `~/.zshrc`:

```bash
source <(dcw completion zsh)
```

### 2. Configure Docker

Ensure Docker is running and your user has the necessary permissions:

```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Verify Docker access
docker ps
```

### 3. Test Installation

Create a test service:

```bash
# Create a simple service
cat > docker-compose.yaml << EOF
version: '3.8'
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
EOF

# Start the service
dcw up -d
```

## Troubleshooting

### Common Issues

1. **Command Not Found**
   - Ensure `$GOPATH/bin` is in your PATH
   - Verify the binary was installed correctly
   - Check file permissions

2. **Docker Permission Issues**
   - Verify Docker daemon is running
   - Check user group membership
   - Review Docker socket permissions

3. **Configuration Errors**
   - Validate YAML syntax
   - Check file permissions
   - Verify environment variables

### Debug Commands

```bash
# Check installation
which dcw
dcw version

# Check Docker
docker version
docker compose version

# Check configuration
dcw config
```

## Updating

### Using Go Install

```bash
go install github.com/your-server-support/docker-compose-wrapper/cmd/compose-wrapper@latest
```

### Using Package Managers

#### Fedora/RHEL

```bash
sudo dnf update docker-compose-wrapper
```

#### Ubuntu/Debian

```bash
sudo apt-get update
sudo apt-get upgrade docker-compose-wrapper
```

## Uninstallation

### Using Go Install

```bash
rm $(which dcw)
```

### Using Package Managers

#### Fedora/RHEL

```bash
sudo dnf remove docker-compose-wrapper
```

#### Ubuntu/Debian

```bash
sudo apt-get remove docker-compose-wrapper
```

## Next Steps

1. [Configuration Guide](configuration)
2. [Rolling Updates](rolling-updates)
3. [Code Structure](code-structure) 