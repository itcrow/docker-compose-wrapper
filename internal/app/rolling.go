package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RollingUpdateConfig represents configuration for rolling update
type RollingUpdateConfig struct {
	Enabled  bool
	Replicas int
}

// Global rolling update configuration
var (
	RollingUpdateRetryCount    = 5 // Number of retries to wait for new containers
	RollingUpdateRetryInterval = 5 // Seconds to wait between retries
)

// GetMainServiceName returns the name of the main service from docker-compose.yml
func GetMainServiceName() (string, error) {
	services, err := GetServiceList()
	if err != nil {
		return "", err
	}
	if len(services) == 0 {
		return "", fmt.Errorf("no services found in docker-compose.yml")
	}
	return services[0], nil
}

// GetRollingUpdateConfig extracts rolling update configuration from values
func GetRollingUpdateConfig(values map[string]interface{}, serviceName string) RollingUpdateConfig {
	config := RollingUpdateConfig{
		Enabled:  false,
		Replicas: 1,
	}

	// Get main service name from appName
	mainService := ""
	if appName, ok := values["appName"].(string); ok {
		mainService = strings.ToLower(appName)
	}

	// Check if this is the main service
	if serviceName == mainService {
		// For main service, check root level configuration first
		if rolling, ok := values["rolling-update"].(bool); ok {
			config.Enabled = rolling
		}
		if replicas, ok := values["replicas"].(int); ok {
			config.Replicas = replicas
		}
		return config // Return immediately for main service
	}

	// For other services, check their specific configuration
	if service, ok := values[serviceName].(map[string]interface{}); ok {
		if rolling, ok := service["rolling-update"].(bool); ok {
			config.Enabled = rolling
		}
		if replicas, ok := service["replicas"].(int); ok {
			config.Replicas = replicas
		}
	}

	return config
}

// GetServiceContainers returns list of container IDs for a specific service
func GetServiceContainers(serviceName string, values map[string]interface{}) ([]string, error) {
	// Get project name from global.projectName
	projectName := "docker" // default fallback
	if global, ok := values["global"].(map[string]interface{}); ok {
		if name, ok := global["projectName"].(string); ok {
			projectName = strings.ToLower(name)
		}
	}

	// Use project name prefix for Docker Compose containers
	cmd := exec.Command("docker", "ps", "-q", "-f", fmt.Sprintf("name=^%s-%s-[0-9]+$", projectName, serviceName))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get containers: %w", err)
	}

	containers := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(containers) == 1 && containers[0] == "" {
		return []string{}, nil
	}

	return containers, nil
}

// PerformRollingUpdate performs rolling update for a service
func PerformRollingUpdate(serviceName string, config RollingUpdateConfig, mergedValues map[string]interface{}) error {
	// First, ensure the service is started
	startCmd := exec.Command("docker", "compose", "up", "-d", "--no-deps", serviceName)
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to start service %s: %w", serviceName, err)
	}

	// Get current containers
	currentContainers, err := GetServiceContainers(serviceName, mergedValues)
	if err != nil {
		return fmt.Errorf("failed to get current containers: %w", err)
	}

	// Scale up to double the replicas
	doubleReplicas := config.Replicas * 2
	scaleUpCmd := exec.Command("docker", "compose", "up", "-d", "--no-deps", "--scale", fmt.Sprintf("%s=%d", serviceName, doubleReplicas), "--no-recreate", serviceName)
	scaleUpCmd.Stdout = os.Stdout
	scaleUpCmd.Stderr = os.Stderr
	if err := scaleUpCmd.Run(); err != nil {
		return fmt.Errorf("failed to scale up service: %w", err)
	}

	// Wait for new containers to start
	var newContainers []string
	for i := 0; i < RollingUpdateRetryCount; i++ {
		fmt.Printf("Waiting for new containers (attempt %d/%d)...\n", i+1, RollingUpdateRetryCount)
		time.Sleep(time.Duration(RollingUpdateRetryInterval) * time.Second)

		// Get all containers after scaling
		allContainers, err := GetServiceContainers(serviceName, mergedValues)
		if err != nil {
			return fmt.Errorf("failed to get containers after scaling: %w", err)
		}

		// Find new containers by comparing with current containers
		newContainers = make([]string, 0)
		for _, container := range allContainers {
			isNew := true
			for _, current := range currentContainers {
				if container == current {
					isNew = false
					break
				}
			}
			if isNew {
				newContainers = append(newContainers, container)
			}
		}

		if len(newContainers) >= config.Replicas {
			break
		}
	}

	if len(newContainers) < config.Replicas {
		return fmt.Errorf("not enough new containers started after %d attempts: got %d, want %d", RollingUpdateRetryCount, len(newContainers), config.Replicas)
	}

	// Remove old containers
	for _, container := range currentContainers {
		stopCmd := exec.Command("docker", "stop", container)
		stopCmd.Stdout = os.Stdout
		stopCmd.Stderr = os.Stderr
		if err := stopCmd.Run(); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", container, err)
		}

		rmCmd := exec.Command("docker", "rm", container)
		rmCmd.Stdout = os.Stdout
		rmCmd.Stderr = os.Stderr
		if err := rmCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", container, err)
		}
	}

	// Scale back down to original replicas
	scaleDownCmd := exec.Command("docker", "compose", "up", "-d", "--no-deps", "--scale", fmt.Sprintf("%s=%d", serviceName, config.Replicas), "--no-recreate", serviceName)
	scaleDownCmd.Stdout = os.Stdout
	scaleDownCmd.Stderr = os.Stderr
	if err := scaleDownCmd.Run(); err != nil {
		return fmt.Errorf("failed to scale down service: %w", err)
	}

	return nil
}

// UpdateService updates a single service with rolling update if configured
func UpdateService(serviceName string, values map[string]interface{}) error {
	// Get main service name from appName
	mainService := ""
	if appName, ok := values["appName"].(string); ok {
		mainService = strings.ToLower(appName)
	}

	config := RollingUpdateConfig{
		Enabled:  false,
		Replicas: 1,
	}

	// Check if this is the main service
	if serviceName == mainService {
		// For main service, check root level configuration first
		if rolling, ok := values["rolling-update"].(bool); ok {
			config.Enabled = rolling
		}
		if replicas, ok := values["replicas"].(int); ok {
			config.Replicas = replicas
		}
	} else {
		// For other services, check their specific configuration
		if service, ok := values[serviceName].(map[string]interface{}); ok {
			if rolling, ok := service["rolling-update"].(bool); ok {
				config.Enabled = rolling
			}
			if replicas, ok := service["replicas"].(int); ok {
				config.Replicas = replicas
			}
		}
	}

	if config.Enabled {
		fmt.Printf("Performing rolling update for service %s\n", serviceName)
		return PerformRollingUpdate(serviceName, config, values)
	}

	// Regular update without rolling update
	fmt.Printf("Updating service %s without rolling update\n", serviceName)
	cmd := exec.Command("docker", "compose", "up", "-d", "--no-deps", serviceName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// HasRollingUpdateEnabled checks if any service has rolling update enabled
func HasRollingUpdateEnabled(values map[string]interface{}) bool {
	// Get main service name from appName
	mainService := ""
	if appName, ok := values["appName"].(string); ok {
		mainService = strings.ToLower(appName)
	}

	// Check root level configuration
	if rolling, ok := values["rolling-update"].(bool); ok && rolling {
		// If root level rolling-update is true, it only applies to main service
		return true
	}

	// Check other services
	for serviceName, serviceConfig := range values {
		// Skip root level configuration and main service
		if serviceName == "rolling-update" || serviceName == "replicas" || serviceName == mainService {
			continue
		}
		if service, ok := serviceConfig.(map[string]interface{}); ok {
			if rolling, ok := service["rolling-update"].(bool); ok && rolling {
				return true
			}
		}
	}
	return false
}

// GetServiceList returns list of all services from docker-compose.yml
func GetServiceList() ([]string, error) {
	cmd := exec.Command("docker", "compose", "config", "--services")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get services list: %w", err)
	}

	services := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(services) == 1 && services[0] == "" {
		return []string{}, nil
	}

	return services, nil
}
