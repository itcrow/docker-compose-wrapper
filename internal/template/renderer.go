package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Renderer handles template rendering
type Renderer struct {
	basePath string
}

// NewRenderer creates a new template renderer
func NewRenderer(basePath string) *Renderer {
	return &Renderer{
		basePath: basePath,
	}
}

// RenderTemplate renders a template with the given values
func (r *Renderer) RenderTemplate(templatePath string, values map[string]interface{}) (string, error) {
	tmplData, err := os.ReadFile(filepath.Join(r.basePath, templatePath))
	if err != nil {
		return "", fmt.Errorf("failed to read template: %w", err)
	}

	tmpl, err := template.New(filepath.Base(templatePath)).Option("missingkey=zero").Parse(string(tmplData))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Check if values already has a Values key
	if _, ok := values["Values"]; !ok {
		// Wrap values in a struct with a Values field
		templateData := struct {
			Values map[string]interface{}
		}{
			Values: values,
		}
		values = map[string]interface{}{
			"Values": templateData.Values,
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderTemplates renders all templates in a directory
func (r *Renderer) RenderTemplates(templateDir string, values map[string]interface{}) (map[string]string, error) {
	results := make(map[string]string)

	templatesAbsDir := filepath.Join(r.basePath, templateDir)
	entries, err := os.ReadDir(templatesAbsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if ext := filepath.Ext(entry.Name()); ext != ".tmpl" {
			continue
		}
		output, err := r.RenderTemplate(filepath.Join(templateDir, entry.Name()), values)
		if err != nil {
			return nil, fmt.Errorf("failed to render template %s: %w", entry.Name(), err)
		}
		base := entry.Name()[:len(entry.Name())-len(filepath.Ext(entry.Name()))]
		if strings.HasSuffix(base, ".yml") {
			results[base] = output
		} else {
			results[base+".yml"] = output
		}
	}

	return results, nil
}
