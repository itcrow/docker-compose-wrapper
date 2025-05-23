package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// executeHook runs a hook either as a command or container
func executeHook(hook Hook, networkName string) error {
	logger.Info("executing hook", "type", hook.Type, "name", hook.Name)

	// If it's a command hook
	if len(hook.Command) > 0 {
		cmd := exec.Command(hook.Command[0], hook.Command[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// If it's a container hook
	if hook.Container != nil {
		ctx := context.Background()
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}
		defer cli.Close()

		// Prepare environment variables
		var env []string
		for k, v := range hook.Container.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}

		logger.Debug("creating container", "image", hook.Container.Image, "network", hook.Container.Network)

		// Create container
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:     hook.Container.Image,
			Cmd:       append(hook.Container.Command, hook.Container.Args...),
			Env:       env,
			Tty:       false,
			OpenStdin: false,
		}, &container.HostConfig{
			NetworkMode: container.NetworkMode(hook.Container.Network),
		}, nil, nil, fmt.Sprintf("%s-hook-%s", hook.Type, hook.Name))
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		logger.Debug("starting container", "id", resp.ID)

		// Start container
		if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}

		// Wait for container to finish
		statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("error waiting for container: %w", err)
			}
		case <-statusCh:
		}

		logger.Debug("container finished", "id", resp.ID)

		// Get container logs
		logs, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		})
		if err != nil {
			return fmt.Errorf("failed to get container logs: %w", err)
		}
		defer logs.Close()

		// Copy logs to stdout
		if _, err := os.Stdout.ReadFrom(logs); err != nil {
			return fmt.Errorf("failed to read container logs: %w", err)
		}

		// Get container exit code
		inspect, err := cli.ContainerInspect(ctx, resp.ID)
		if err != nil {
			return fmt.Errorf("failed to inspect container: %w", err)
		}

		if inspect.State.ExitCode != 0 {
			return fmt.Errorf("hook container exited with code %d", inspect.State.ExitCode)
		}

		logger.Debug("removing container", "id", resp.ID)

		// Remove container
		removeOpts := types.ContainerRemoveOptions{
			Force: true,
		}
		if err := cli.ContainerRemove(ctx, resp.ID, removeOpts); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}

		// Wait for the container to be removed
		waitCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionRemoved)
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("failed to wait for container removal: %w", err)
			}
		case <-waitCh:
			logger.Debug("container removed", "id", resp.ID)
		}
	}

	return nil
}

// waitForServices waits for specified services to be ready
func waitForServices(services []string, timeout time.Duration) error {
	if len(services) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	for _, service := range services {
		logger.Info("waiting for service", "service", service)
		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting for service %s", service)
			default:
				filterArgs := filters.NewArgs()
				filterArgs.Add("name", service)
				containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
					Filters: filterArgs,
				})
				if err != nil {
					return fmt.Errorf("failed to list containers: %w", err)
				}

				if len(containers) > 0 && containers[0].State == "running" {
					logger.Info("service is ready", "service", service)
					break
				}

				time.Sleep(time.Second)
			}
		}
	}

	return nil
}

// ExecuteHooks runs all hooks of the specified type
func ExecuteHooks(chart *ChartYAML, hookType string, networkName string) error {
	var hooks []Hook
	for _, hook := range chart.Hooks {
		if hook.Type == hookType {
			hooks = append(hooks, hook)
		}
	}

	for _, hook := range hooks {
		// Parse timeout
		timeout := 5 * time.Minute // default timeout
		if hook.Timeout != "" {
			var err error
			timeout, err = time.ParseDuration(hook.Timeout)
			if err != nil {
				return fmt.Errorf("invalid timeout format for hook %s: %w", hook.Name, err)
			}
		}

		logger.Debug("executing hook", "name", hook.Name, "type", hook.Type, "timeout", timeout)

		// Wait for required services
		if err := waitForServices(hook.WaitFor, timeout); err != nil {
			return fmt.Errorf("failed waiting for services for hook %s: %w", hook.Name, err)
		}

		// Execute the hook
		if err := executeHook(hook, networkName); err != nil {
			return fmt.Errorf("hook %s failed: %w", hook.Name, err)
		}
	}

	return nil
}
