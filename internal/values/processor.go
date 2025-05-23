package values

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Processor handles value processing and merging
type Processor struct {
	rootPath string
}

// NewProcessor creates a new values processor
func NewProcessor(rootPath string) *Processor {
	return &Processor{
		rootPath: rootPath,
	}
}

// LoadValuesFile loads values from a YAML file
func (p *Processor) LoadValuesFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read values file: %w", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("failed to parse values file: %w", err)
	}

	return values, nil
}

// MergeValues merges multiple value maps
func (p *Processor) MergeValues(values ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for _, v := range values {
		for key, value := range v {
			result[key] = value
		}
	}

	return result
}

// ProcessSetValues processes --set values
func (p *Processor) ProcessSetValues(setValues []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, setValue := range setValues {
		parts := strings.SplitN(setValue, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid set value format: %s", setValue)
		}

		key := parts[0]
		value := parts[1]

		// Handle nested keys
		keys := strings.Split(key, ".")
		current := result
		for i, k := range keys {
			if i == len(keys)-1 {
				current[k] = value
			} else {
				if _, ok := current[k]; !ok {
					current[k] = make(map[string]interface{})
				}
				current = current[k].(map[string]interface{})
			}
		}
	}

	return result, nil
}

// ProcessSetFileValues processes --set-file values
func (p *Processor) ProcessSetFileValues(setFileValues []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, setFileValue := range setFileValues {
		parts := strings.SplitN(setFileValue, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid set-file value format: %s", setFileValue)
		}

		key := parts[0]
		filePath := parts[1]

		// Read file content
		content, err := os.ReadFile(filepath.Join(p.rootPath, filePath))
		if err != nil {
			return nil, fmt.Errorf("failed to read file for set-file: %w", err)
		}

		// Handle nested keys
		keys := strings.Split(key, ".")
		current := result
		for i, k := range keys {
			if i == len(keys)-1 {
				current[k] = string(content)
			} else {
				if _, ok := current[k]; !ok {
					current[k] = make(map[string]interface{})
				}
				current = current[k].(map[string]interface{})
			}
		}
	}

	return result, nil
}
