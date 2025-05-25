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
func GetServiceContainers(serviceName string) ([]string, error) {
	// Use exact name match with ^ and $ to prevent partial matches
	cmd := exec.Command("docker", "ps", "-q", "-f", fmt.Sprintf("name=^%s$", serviceName))
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
func PerformRollingUpdate(serviceName string, config RollingUpdateConfig) error {
	// Get current containers
	currentContainers, err := GetServiceContainers(serviceName)
	if err != nil {
		return fmt.Errorf("failed to get current containers: %w", err)
	}

	// Scale up to double replicas
	scaleUpCmd := exec.Command("docker", "compose", "up", "-d", "--no-deps",
		"--scale", fmt.Sprintf("%s=%d", serviceName, config.Replicas*2),
		"--no-recreate", serviceName)
	scaleUpCmd.Stdout = os.Stdout
	scaleUpCmd.Stderr = os.Stderr
	if err := scaleUpCmd.Run(); err != nil {
		return fmt.Errorf("failed to scale up: %w", err)
	}

	// Wait for new containers to start
	time.Sleep(10 * time.Second)

	// Verify that new containers are running
	_, err = GetServiceContainers(serviceName)
	if err != nil {
		return fmt.Errorf("failed to verify new containers: %w", err)
	}

	// Remove old containers
	for _, container := range currentContainers {
		// Send SIGTERM
		killCmd := exec.Command("docker", "kill", "-s", "SIGTERM", container)
		killCmd.Stdout = os.Stdout
		killCmd.Stderr = os.Stderr
		if err := killCmd.Run(); err != nil {
			fmt.Printf("Warning: failed to kill container %s: %v\n", container, err)
		}

		// Wait a bit
		time.Sleep(time.Second)

		// Remove container
		rmCmd := exec.Command("docker", "rm", "-f", container)
		rmCmd.Stdout = os.Stdout
		rmCmd.Stderr = os.Stderr
		if err := rmCmd.Run(); err != nil {
			fmt.Printf("Warning: failed to remove container %s: %v\n", container, err)
		}
	}

	// Scale back to original replicas
	scaleDownCmd := exec.Command("docker", "compose", "up", "-d", "--no-deps",
		"--scale", fmt.Sprintf("%s=%d", serviceName, config.Replicas),
		"--no-recreate", serviceName)
	scaleDownCmd.Stdout = os.Stdout
	scaleDownCmd.Stderr = os.Stderr
	if err := scaleDownCmd.Run(); err != nil {
		return fmt.Errorf("failed to scale down: %w", err)
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
		return PerformRollingUpdate(serviceName, config)
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
