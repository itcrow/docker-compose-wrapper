package chart

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Loader handles loading and processing of charts
type Loader struct {
	rootPath string
}

// NewLoader creates a new chart loader
func NewLoader(rootPath string) *Loader {
	return &Loader{
		rootPath: rootPath,
	}
}

// LoadChart loads a chart from the specified path
func (l *Loader) LoadChart(path string) (*Chart, error) {
	chartPath := filepath.Join(l.rootPath, path, "Chart.yaml")
	data, err := os.ReadFile(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Chart.yaml: %w", err)
	}

	var chart Chart
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, fmt.Errorf("failed to parse Chart.yaml: %w", err)
	}

	return &chart, nil
}

// LoadValues loads values from the specified path
func (l *Loader) LoadValues(path string) (*Values, error) {
	valuesPath := filepath.Join(l.rootPath, path, "values.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read values.yaml: %w", err)
	}

	var values Values
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse values.yaml: %w", err)
	}

	return &values, nil
}

// LoadDependencies loads all dependencies for a chart
func (l *Loader) LoadDependencies(chart *Chart) (map[string]*Chart, error) {
	deps := make(map[string]*Chart)
	for _, dep := range chart.Dependencies {
		depChart, err := l.LoadChart(dep.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to load dependency %s: %w", dep.Name, err)
		}
		deps[dep.Name] = depChart
	}
	return deps, nil
}
