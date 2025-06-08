---
layout: default
title: Rolling Updates
---

# Rolling Updates

The Docker Compose Wrapper provides a powerful rolling update mechanism that ensures zero-downtime deployments.

## How Rolling Updates Work

The rolling update process follows these steps:

1. **Scale Up**: Double the number of replicas
2. **Wait for New Containers**: Ensure new containers are running
3. **Remove Old Containers**: Stop and remove old containers
4. **Scale Down**: Return to the desired number of replicas

## Configuration

Configure rolling updates in your values file:

```yaml
services:
  web:
    replicas: 3
    rollingUpdate:
      replicas: 3        # Number of replicas to maintain
      retryCount: 10     # Number of attempts to wait for new containers
      retryInterval: 30  # Time between retry attempts in seconds
```

## Usage

### Basic Rolling Update

```bash
dcw rolling-update web
```

### Rolling Update with Environment

```bash
dcw rolling-update -e prod web
```

### Custom Configuration

```bash
dcw rolling-update --replicas 3 --retry-count 10 --retry-interval 30 web
```

## Process Details

### 1. Scale Up

The service is scaled to double the desired replicas:

```bash
docker compose up -d --scale web=6
```

### 2. Wait for New Containers

The system waits for new containers to start:

```go
// Wait for new containers
for i := 0; i < retryCount; i++ {
    time.Sleep(time.Duration(retryInterval) * time.Second)
    // Check if new containers are running
}
```

### 3. Remove Old Containers

Old containers are stopped and removed:

```bash
docker stop <old-container-id>
docker rm <old-container-id>
```

### 4. Scale Down

The service is scaled back to the desired number of replicas:

```bash
docker compose up -d --scale web=3
```

## Best Practices

1. **Replica Count**
   - Set appropriate replica counts for your workload
   - Consider resource constraints
   - Plan for peak loads

2. **Retry Configuration**
   - Set reasonable retry counts and intervals
   - Consider container startup time
   - Account for network latency

3. **Monitoring**
   - Monitor container health during updates
   - Watch for resource usage spikes
   - Check application logs

4. **Rollback Plan**
   - Have a rollback strategy ready
   - Test rollback procedures
   - Keep previous versions available

## Common Scenarios

### High Availability Setup

```yaml
services:
  web:
    replicas: 3
    rollingUpdate:
      replicas: 3
      retryCount: 15
      retryInterval: 20
```

### Development Environment

```yaml
services:
  web:
    replicas: 1
    rollingUpdate:
      replicas: 2
      retryCount: 5
      retryInterval: 10
```

### Production Environment

```yaml
services:
  web:
    replicas: 5
    rollingUpdate:
      replicas: 5
      retryCount: 20
      retryInterval: 30
```

## Troubleshooting

### Common Issues

1. **Container Startup Failures**
   - Check container logs
   - Verify resource constraints
   - Review environment variables

2. **Update Timeouts**
   - Increase retry count
   - Adjust retry interval
   - Check network connectivity

3. **Resource Exhaustion**
   - Monitor system resources
   - Adjust replica counts
   - Consider resource limits

### Debug Commands

```bash
# Check container status
docker ps -a

# View container logs
docker logs <container-id>

# Check service status
dcw ps

# View service logs
dcw logs web
```

## Advanced Topics

### Custom Health Checks

Add health checks to your service configuration:

```yaml
services:
  web:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Graceful Shutdown

Configure graceful shutdown periods:

```yaml
services:
  web:
    stop_grace_period: 30s
```

### Resource Limits

Set resource constraints:

```yaml
services:
  web:
    deploy:
      resources:
        limits:
          cpus: '0.50'
          memory: 512M
        reservations:
          cpus: '0.25'
          memory: 256M
``` 