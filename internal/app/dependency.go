package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Dependency represents a chart dependency
type Dependency struct {
	Name       string `yaml:"name"`
	Repository string `yaml:"repository,omitempty"` // Optional for local charts
	Version    string `yaml:"version,omitempty"`    // Optional for local charts
	Path       string `yaml:"path,omitempty"`       // Path to local chart
}

// Hook represents a pre or post hook configuration
type Hook struct {
	Name      string           `yaml:"name"`
	Type      string           `yaml:"type"` // "pre" or "post"
	Command   []string         `yaml:"command,omitempty"`
	Container *ContainerConfig `yaml:"container,omitempty"`
	WaitFor   []string         `yaml:"waitFor,omitempty"` // List of services to wait for
	Timeout   string           `yaml:"timeout,omitempty"` // e.g., "30s", "1m"
}

// ContainerConfig represents a container configuration for hooks
type ContainerConfig struct {
	Image   string            `yaml:"image"`
	Command []string          `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Network string            `yaml:"network,omitempty"`
}

// ChartYAML represents the structure of Chart.yaml
type ChartYAML struct {
	Name         string       `yaml:"name"`
	Version      string       `yaml:"version"`
	Dependencies []Dependency `yaml:"dependencies"`
	Hooks        []Hook       `yaml:"hooks,omitempty"`
	MaxReleases  int          `yaml:"maxReleases,omitempty"` // Maximum number of releases to keep
}

// loadChartYAML loads and parses Chart.yaml
func loadChartYAML(chartPath string) (*ChartYAML, error) {
	data, err := os.ReadFile(filepath.Join(chartPath, "Chart.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	var chart ChartYAML
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, fmt.Errorf("failed to parse Chart.yaml: %w", err)
	}

	return &chart, nil
}

// isGitRepo checks if the repository URL is a Git repository
func isGitRepo(repo string) bool {
	return strings.HasSuffix(repo, ".git") || strings.HasPrefix(repo, "git@")
}

// downloadGitDependency downloads a chart from a Git repository
func downloadGitDependency(dep Dependency, chartsDir string) error {
	depDir := filepath.Join(chartsDir, dep.Name)

	// Clone or update the repository
	if _, err := os.Stat(depDir); os.IsNotExist(err) {
		// Clone new repository
		cmd := exec.Command("git", "clone", dep.Repository, depDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		// Update existing repository
		cmd := exec.Command("git", "-C", depDir, "fetch", "origin")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to fetch repository: %w", err)
		}
	}

	// Checkout specific version
	cmd := exec.Command("git", "-C", depDir, "checkout", dep.Version)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to checkout version %s: %w", dep.Version, err)
	}

	return nil
}

// downloadHelmDependency downloads a chart from a Helm repository
func downloadHelmDependency(dep Dependency, chartsDir string) error {
	depDir := filepath.Join(chartsDir, dep.Name)

	// Create temporary directory for download
	tmpDir, err := os.MkdirTemp("", "helm-chart-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download chart using helm
	cmd := exec.Command("helm", "pull",
		"--repo", dep.Repository,
		"--version", dep.Version,
		"--destination", tmpDir,
		dep.Name)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download chart: %w", err)
	}

	// Extract chart
	cmd = exec.Command("tar", "-xf", filepath.Join(tmpDir, fmt.Sprintf("%s-%s.tgz", dep.Name, dep.Version)), "-C", tmpDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract chart: %w", err)
	}

	// Move to charts directory
	if err := os.RemoveAll(depDir); err != nil {
		return fmt.Errorf("failed to remove existing chart: %w", err)
	}
	if err := os.Rename(filepath.Join(tmpDir, dep.Name), depDir); err != nil {
		return fmt.Errorf("failed to move chart: %w", err)
	}

	return nil
}

// UpdateDependencies updates all chart dependencies
func UpdateDependencies(chartPath string) error {
	chart, err := loadChartYAML(chartPath)
	if err != nil {
		return err
	}

	chartsDir := filepath.Join(chartPath, "charts")
	if err := os.MkdirAll(chartsDir, 0755); err != nil {
		return fmt.Errorf("failed to create charts directory: %w", err)
	}

	for _, dep := range chart.Dependencies {
		fmt.Printf("Updating dependency: %s\n", dep.Name)

		// Handle local chart
		if dep.Path != "" {
			localPath := filepath.Join(chartPath, dep.Path)
			targetPath := filepath.Join(chartsDir, dep.Name)

			// Check if source exists
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				return fmt.Errorf("local chart path does not exist: %s", localPath)
			}

			// Remove existing chart if it exists
			if err := os.RemoveAll(targetPath); err != nil {
				return fmt.Errorf("failed to remove existing chart: %w", err)
			}

			// Copy local chart to charts directory
			if err := copyDir(localPath, targetPath); err != nil {
				return fmt.Errorf("failed to copy local chart: %w", err)
			}
			continue
		}

		// Handle remote chart
		if isGitRepo(dep.Repository) {
			if err := downloadGitDependency(dep, chartsDir); err != nil {
				return fmt.Errorf("failed to download git dependency %s: %w", dep.Name, err)
			}
		} else {
			if err := downloadHelmDependency(dep, chartsDir); err != nil {
				return fmt.Errorf("failed to download helm dependency %s: %w", dep.Name, err)
			}
		}
	}

	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// ListDependencies lists all chart dependencies
func ListDependencies(chartPath string) error {
	chart, err := loadChartYAML(chartPath)
	if err != nil {
		return err
	}

	fmt.Println("Chart dependencies:")
	for _, dep := range chart.Dependencies {
		fmt.Printf("- %s (%s) from %s\n", dep.Name, dep.Version, dep.Repository)
	}

	return nil
}
