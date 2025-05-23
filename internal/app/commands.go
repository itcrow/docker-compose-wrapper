package app

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tplt "github.com/batishchev/docker-manager/internal/template"

	"github.com/batishchev/docker-manager/internal/chart"
	"github.com/batishchev/docker-manager/internal/values"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// For color output
const (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func NewRootCommand() *cobra.Command {
	var valuesFiles []string
	var setValues []string
	var setStringValues []string
	var setFileValues []string
	var force bool

	cmd := &cobra.Command{
		Use:   "compose-wrapper",
		Short: "Docker Compose wrapper with template support",
		Long:  `A wrapper for Docker Compose that supports templating and values files`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				switch args[0] {
				case "releases":
					return newReleasesCommand().RunE(cmd, args[1:])
				case "rollback":
					return newRollbackCommand().RunE(cmd, args[1:])
				case "lint":
					return newLintCommand().RunE(cmd, args[1:])
				case "dependency":
					return RunCommand(args[1:])
				}
			}
			if len(args) == 0 {
				return cmd.Help()
			}

			// Get current directory
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Initialize processors
			chartLoader := chart.NewLoader(workDir)
			valuesProcessor := values.NewProcessor(workDir)

			// Load main values
			mainValues, err := chartLoader.LoadValues(".")
			if err != nil {
				return fmt.Errorf("failed to load main values: %w", err)
			}

			// Load additional values files
			var additionalValues []map[string]interface{}
			for _, valuesFile := range valuesFiles {
				vals, err := valuesProcessor.LoadValuesFile(valuesFile)
				if err != nil {
					return fmt.Errorf("failed to load values file %s: %w", valuesFile, err)
				}
				additionalValues = append(additionalValues, vals)
			}

			// Process set values
			setVals, err := valuesProcessor.ProcessSetValues(setValues)
			if err != nil {
				return fmt.Errorf("failed to process set values: %w", err)
			}

			// Process set-file values
			setFileVals, err := valuesProcessor.ProcessSetFileValues(setFileValues)
			if err != nil {
				return fmt.Errorf("failed to process set-file values: %w", err)
			}

			// Merge all values
			allValues := append([]map[string]interface{}{mainValues.Values}, additionalValues...)
			allValues = append(allValues, setVals, setFileVals)
			mergedValues := valuesProcessor.MergeValues(allValues...)

			// Add global values
			globalMap := map[string]interface{}{
				"projectName":            mainValues.Global.ProjectName,
				"environment":            mainValues.Global.Environment,
				"defaultImagePullPolicy": mainValues.Global.DefaultImagePullPolicy,
				"network": map[string]interface{}{
					"name":   mainValues.Global.Network.Name,
					"alias":  mainValues.Global.Network.Alias,
					"driver": mainValues.Global.Network.Driver,
				},
			}
			mergedValues["global"] = globalMap

			// Debug print Global struct
			fmt.Printf("Global values: %#v\n", mainValues.Global)

			// Debug print merged values
			fmt.Printf("Merged values: %#v\n", mergedValues)

			// Discover all child charts
			childChartsDir := filepath.Join(workDir, "charts")
			childChartEntries, err := os.ReadDir(childChartsDir)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to read child charts directory: %w", err)
			}
			var childCharts []string
			for _, entry := range childChartEntries {
				if entry.IsDir() {
					childCharts = append(childCharts, entry.Name())
				}
			}

			// Render main chart templates
			_, err = tplt.NewRenderer(workDir).RenderTemplates("templates", mergedValues)
			if err != nil {
				return fmt.Errorf("failed to render templates: %w", err)
			}

			// Create output directory
			distDir := filepath.Join(workDir, "dist")
			var versionDirs []struct {
				name    string
				version int
			}
			entries, err := os.ReadDir(distDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						name := entry.Name()
						if len(name) > 2 && name[0] == 'v' {
							var n int
							if _, err := fmt.Sscanf(name, "v%d-", &n); err == nil {
								versionDirs = append(versionDirs, struct {
									name    string
									version int
								}{name, n})
							}
						}
					}
				}
			}
			sort.Slice(versionDirs, func(i, j int) bool {
				return versionDirs[i].version > versionDirs[j].version
			})

			// Get max releases from Chart.yaml or use default
			chart, err := loadChartYAML(workDir)
			if err != nil {
				return fmt.Errorf("failed to load Chart.yaml: %w", err)
			}
			maxReleases := 20 // default value
			if chart.MaxReleases > 0 {
				maxReleases = chart.MaxReleases
			}

			// Cleanup old releases
			if len(versionDirs) >= maxReleases {
				for _, v := range versionDirs[maxReleases:] {
					oldReleasePath := filepath.Join(distDir, v.name)
					logger.Debug("removing old release", "version", v.name)
					if err := os.RemoveAll(oldReleasePath); err != nil {
						logger.Warn("failed to remove old release", "version", v.name, "error", err)
					}
				}
			}

			maxVersion := 0
			if len(versionDirs) > 0 {
				maxVersion = versionDirs[0].version
			}

			// Calculate config hash
			configBytes, err := json.Marshal(mergedValues)
			if err != nil {
				return fmt.Errorf("failed to marshal merged values: %w", err)
			}
			configHash := fmt.Sprintf("%x", sha1.Sum(configBytes))[:8]

			// Check if we have a previous version with the same hash
			var latestVersion string
			var versionDir string
			if len(versionDirs) > 0 {
				latestVersion = versionDirs[0].name
				latestHash := strings.Split(latestVersion, "-")[1]
				if latestHash == configHash && !force {
					logger.Debug("no changes detected, reusing latest version", "version", latestVersion)
					// Use the latest version directory
					versionDir = filepath.Join(distDir, latestVersion)
					fmt.Printf("\n%sNo changes detected in configuration%s\n", colorYellow, colorReset)
					fmt.Printf("Reusing existing version: %s\n", latestVersion)
				} else {
					// Generate new version
					newVersion := maxVersion + 1
					versionDir = filepath.Join(distDir, fmt.Sprintf("v%d-%s", newVersion, configHash))
					if force {
						logger.Debug("force creating new release", "version", newVersion, "hash", configHash)
						fmt.Printf("\n%sForce creating new version%s\n", colorYellow, colorReset)
					} else {
						logger.Debug("creating new release", "version", newVersion, "hash", configHash)
					}
				}
			} else {
				// First version
				versionDir = filepath.Join(distDir, fmt.Sprintf("v1-%s", configHash))
				logger.Debug("creating first release", "hash", configHash)
			}

			// Create version directory if it doesn't exist or if force is true
			if force {
				// Якщо force=true, видаляємо стару директорію якщо вона існує
				if err := os.RemoveAll(versionDir); err != nil {
					return fmt.Errorf("failed to remove old version directory: %w", err)
				}
			}
			if _, err := os.Stat(versionDir); os.IsNotExist(err) {
				if err := os.MkdirAll(versionDir, 0755); err != nil {
					return fmt.Errorf("failed to create version directory: %w", err)
				}

				// Save merged values
				mergedValuesFile := filepath.Join(versionDir, "values.yaml")
				valuesYamlBytes, err := yaml.Marshal(mergedValues)
				if err != nil {
					return fmt.Errorf("failed to marshal merged values to YAML: %w", err)
				}
				if err := os.WriteFile(mergedValuesFile, valuesYamlBytes, 0644); err != nil {
					return fmt.Errorf("failed to write values.yaml: %w", err)
				}

				// Create docker directory
				dockerDir := filepath.Join(versionDir, "docker")
				if err := os.MkdirAll(dockerDir, 0755); err != nil {
					return fmt.Errorf("failed to create docker directory: %w", err)
				}

				// Generate main compose file
				mainComposeFile := filepath.Join(dockerDir, "docker-compose.yml")
				mainTemplate := filepath.Join(workDir, "templates/docker-compose.yml.tmpl")
				mainContent, err := renderTemplate(mainTemplate, mergedValues)
				if err != nil {
					return fmt.Errorf("failed to render main template: %w", err)
				}
				if err := os.WriteFile(mainComposeFile, []byte(mainContent), 0644); err != nil {
					return fmt.Errorf("failed to write main compose file: %w", err)
				}

				// Generate compose files for dependencies
				chartsDir := filepath.Join(workDir, "charts")
				entries, err = os.ReadDir(chartsDir)
				if err != nil {
					return fmt.Errorf("failed to read charts directory: %w", err)
				}

				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}

					chartName := entry.Name()
					chartTemplate := filepath.Join(chartsDir, chartName, "templates/docker-compose.yml.tmpl")
					if _, err := os.Stat(chartTemplate); os.IsNotExist(err) {
						continue
					}

					// Створюємо контекст для шаблону
					chartValues, ok := mergedValues[chartName].(map[string]interface{})
					if !ok {
						return fmt.Errorf("invalid values type for child chart %s", chartName)
					}

					// Рекурсивно об'єднуємо значення
					mergedChartValues := make(map[string]interface{})

					// Додаємо глобальні значення
					if global, ok := mergedValues["global"].(map[string]interface{}); ok {
						mergedChartValues["global"] = global
					}

					// Додаємо значення з кореневого чарту
					if rootValues, ok := mergedValues["app"].(map[string]interface{}); ok {
						mergedChartValues["root"] = rootValues
					}

					// Додаємо значення з інших чартів
					for name, values := range mergedValues {
						if name != chartName && name != "global" && name != "app" {
							if vals, ok := values.(map[string]interface{}); ok {
								mergedChartValues[name] = vals
							}
						}
					}

					// Рекурсивно об'єднуємо значення з поточного чарту
					mergeValuesRecursively(mergedChartValues, chartValues)

					// Додаємо значення з поточного чарту в корінь
					if currentValues, ok := mergedValues[chartName].(map[string]interface{}); ok {
						mergeValuesRecursively(mergedChartValues, currentValues)
					}

					// Дебаг вивід значень для дочірнього чарту
					fmt.Printf("\nValues for chart %s:\n", chartName)
					valuesYaml, err := yaml.Marshal(mergedChartValues)
					if err != nil {
						return fmt.Errorf("failed to marshal values for chart %s: %w", chartName, err)
					}
					fmt.Printf("%s\n", string(valuesYaml))

					// Створюємо контекст для шаблону з правильним шляхом до значень
					templateContext := map[string]interface{}{
						"Values": mergedChartValues,
					}

					chartContent, err := renderTemplate(chartTemplate, templateContext)
					if err != nil {
						return fmt.Errorf("failed to render chart template %s: %w", chartName, err)
					}

					chartDir := filepath.Join(dockerDir, chartName)
					if err := os.MkdirAll(chartDir, 0755); err != nil {
						return fmt.Errorf("failed to create chart directory: %w", err)
					}

					chartFile := filepath.Join(chartDir, "docker-compose.yml")
					if err := os.WriteFile(chartFile, []byte(chartContent), 0644); err != nil {
						return fmt.Errorf("failed to write chart compose file: %w", err)
					}
				}
			} else {
				logger.Debug("reusing existing version", "version", filepath.Base(versionDir))
			}

			// Get network name from global values
			networkName := "default"
			if global, ok := mergedValues["global"].(map[string]interface{}); ok {
				if network, ok := global["network"].(map[string]interface{}); ok {
					if name, ok := network["name"].(string); ok {
						networkName = strings.ToLower(name)
					}
				}
			}

			logger.Debug("running pre-hooks")
			// Run pre-hooks
			if err := ExecuteHooks(chart, "pre", networkName); err != nil {
				return fmt.Errorf("pre-hooks failed: %w", err)
			}

			logger.Debug("running docker compose")
			// Run docker compose

			// Збираємо всі docker-compose файли
			var composeFiles []string
			composeFiles = append(composeFiles, "docker-compose.yml")

			// Додаємо файли з піддиректорій
			chartsDir := filepath.Join(versionDir, "docker")
			entries, err = os.ReadDir(chartsDir)
			if err != nil {
				return fmt.Errorf("failed to read charts directory: %w", err)
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				chartName := entry.Name()
				chartComposeFile := filepath.Join(chartName, "docker-compose.yml")
				if _, err := os.Stat(filepath.Join(chartsDir, chartComposeFile)); os.IsNotExist(err) {
					continue
				}

				composeFiles = append(composeFiles, chartComposeFile)
			}

			// Встановлюємо змінну середовища
			os.Setenv("COMPOSE_FILE", strings.Join(composeFiles, ":"))

			// Змінюємо поточну директорію на директорію з docker-compose файлами
			if err := os.Chdir(filepath.Join(versionDir, "docker")); err != nil {
				return fmt.Errorf("failed to change to docker directory: %w", err)
			}

			// Запускаємо docker compose
			composeArgs := []string{"compose"}
			// Фільтруємо аргументи, видаляючи --force
			filteredArgs := make([]string, 0, len(args))
			for _, arg := range args {
				if arg != "--force" {
					filteredArgs = append(filteredArgs, arg)
				}
			}
			composeArgs = append(composeArgs, filteredArgs...)
			if force {
				// Додаємо --force-recreate як аргумент для команди up
				for i, arg := range composeArgs {
					if arg == "up" {
						composeArgs = append(composeArgs[:i+1], append([]string{"--force-recreate"}, composeArgs[i+1:]...)...)
						break
					}
				}
			}
			composeCmd := exec.Command("docker", composeArgs...)
			composeCmd.Stdout = os.Stdout
			composeCmd.Stderr = os.Stderr
			if err := composeCmd.Run(); err != nil {
				return fmt.Errorf("docker compose failed: %w", err)
			}

			logger.Debug("running post-hooks")
			// Run post-hooks
			if err := ExecuteHooks(chart, "post", networkName); err != nil {
				return fmt.Errorf("post-hooks failed: %w", err)
			}

			// Print release info using fmt
			fmt.Printf("+++++++++++++++++++++++++++++++++++++++\n")
			fmt.Printf("Release:  %s\n", filepath.Base(versionDir))
			fmt.Printf("Status:   \033[32mSUCCESS\033[0m\n")
			fmt.Printf("+++++++++++++++++++++++++++++++++++++++\n")

			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&valuesFiles, "values", "f", []string{}, "Specify values files")
	cmd.Flags().StringArrayVar(&setValues, "set", []string{}, "Set values on the command line")
	cmd.Flags().StringArrayVar(&setStringValues, "set-string", []string{}, "Set STRING values on the command line")
	cmd.Flags().StringArrayVar(&setFileValues, "set-file", []string{}, "Set values from respective files")
	cmd.Flags().BoolVar(&force, "force", false, "Force recreation of containers")
	cmd.DisableFlagParsing = true

	return cmd
}

func newLintCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate chart configuration",
		Long:  `Validate the chart configuration, including templates and values files`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Render all templates to a temp directory
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			chartLoader := chart.NewLoader(workDir)
			valuesProcessor := values.NewProcessor(workDir)

			mainValues, err := chartLoader.LoadValues(".")
			if err != nil {
				return fmt.Errorf("failed to load main values: %w", err)
			}

			allValues := []map[string]interface{}{mainValues.Values}
			mergedValues := valuesProcessor.MergeValues(allValues...)
			globalMap := map[string]interface{}{
				"projectName":            mainValues.Global.ProjectName,
				"environment":            mainValues.Global.Environment,
				"defaultImagePullPolicy": mainValues.Global.DefaultImagePullPolicy,
				"network": map[string]interface{}{
					"name":   mainValues.Global.Network.Name,
					"alias":  mainValues.Global.Network.Alias,
					"driver": mainValues.Global.Network.Driver,
				},
			}
			mergedValues["global"] = globalMap

			tempDir, err := os.MkdirTemp("", "compose-lint-*")
			if err != nil {
				return fmt.Errorf("failed to create temp dir: %w", err)
			}
			defer os.RemoveAll(tempDir)

			_, err = tplt.NewRenderer(workDir).RenderTemplates("templates", mergedValues)
			if err != nil {
				return fmt.Errorf("failed to render templates: %w", err)
			}

			childChartsDir := filepath.Join(workDir, "charts")
			childChartEntries, _ := os.ReadDir(childChartsDir)
			var childCharts []string
			for _, entry := range childChartEntries {
				if entry.IsDir() {
					childCharts = append(childCharts, entry.Name())
				}
			}
			for _, child := range childCharts {
				childTemplatesDir := filepath.Join("charts", child, "templates")
				childValues, err := chartLoader.LoadValues(filepath.Join("charts", child))
				if err != nil {
					return fmt.Errorf("failed to load child chart values for %s: %w", child, err)
				}
				childMergedValues := make(map[string]interface{})
				for k, v := range childValues.Values {
					childMergedValues[k] = v
				}
				for k, v := range mergedValues {
					if k == "global" || k == child {
						continue
					}
					if _, exists := childMergedValues[k]; !exists {
						childMergedValues[k] = v
					}
				}
				if global, ok := mergedValues["global"]; ok {
					childMergedValues["global"] = global
				}
				valuesContext := map[string]interface{}{
					"Values": childMergedValues,
				}
				childRenderedFiles, err := tplt.NewRenderer(workDir).RenderTemplates(childTemplatesDir, valuesContext)
				if err != nil {
					return fmt.Errorf("failed to render child chart templates for %s: %w", child, err)
				}
				childOutputDir := filepath.Join(tempDir, child)
				for path, content := range childRenderedFiles {
					outputPath := filepath.Join(childOutputDir, path)
					if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
						return fmt.Errorf("failed to create output directory for %s: %w", path, err)
					}
					if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
						return fmt.Errorf("failed to write rendered file %s: %w", path, err)
					}
				}
			}

			// Find all compose files and lint them
			err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && (filepath.Base(path) == "docker-compose.yml" || filepath.Base(path) == "compose.yml") {
					fmt.Printf("Linting %s...\n", path)
					cmd := exec.Command("docker", "compose", "-f", path, "config")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					if err := cmd.Run(); err != nil {
						return fmt.Errorf("docker compose config failed for %s: %w", path, err)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			fmt.Println("All compose files linted successfully.")
			return nil
		},
	}

	return cmd
}

func newReleasesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "releases",
		Short: "List all generated configuration versions",
		Long:  "Show a list of all generated configuration versions in the dist/ directory.",
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			distDir := filepath.Join(workDir, "dist")
			entries, err := os.ReadDir(distDir)
			if err != nil {
				return fmt.Errorf("failed to read dist directory: %w", err)
			}
			var versionDirs []struct {
				name    string
				version int
				time    string
			}
			for _, entry := range entries {
				if entry.IsDir() {
					name := entry.Name()
					if len(name) > 2 && name[0] == 'v' {
						var n int
						if _, err := fmt.Sscanf(name, "v%d-", &n); err == nil {
							valuesPath := filepath.Join(distDir, name, "values.yaml")
							var ts string
							if info, err := os.Stat(valuesPath); err == nil {
								ts = info.ModTime().Format("2006-01-02 15:04:05")
							}
							versionDirs = append(versionDirs, struct {
								name    string
								version int
								time    string
							}{name, n, ts})
						}
					}
				}
			}
			sort.Slice(versionDirs, func(i, j int) bool {
				return versionDirs[i].version > versionDirs[j].version
			})
			fmt.Println("Available releases:")
			for _, v := range versionDirs {
				fmt.Printf("  %s  %s\n", v.name, v.time)
			}
			return nil
		},
	}
	return cmd
}

func newRollbackCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollback [release] [compose args]",
		Short: "Run a specific or previous generated configuration",
		Long:  "Run Docker Compose using a specific or previous generated configuration in dist/.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			distDir := filepath.Join(workDir, "dist")
			entries, err := os.ReadDir(distDir)
			if err != nil {
				return fmt.Errorf("failed to read dist directory: %w", err)
			}
			var versionDirs []struct {
				name    string
				version int
			}
			for _, entry := range entries {
				if entry.IsDir() {
					name := entry.Name()
					if len(name) > 2 && name[0] == 'v' {
						var n int
						if _, err := fmt.Sscanf(name, "v%d-", &n); err == nil {
							versionDirs = append(versionDirs, struct {
								name    string
								version int
							}{name, n})
						}
					}
				}
			}
			sort.Slice(versionDirs, func(i, j int) bool {
				return versionDirs[i].version > versionDirs[j].version
			})

			var targetDir string
			if len(args) > 0 && strings.HasPrefix(args[0], "v") {
				// User specified a release
				release := args[0]
				found := false
				for _, v := range versionDirs {
					if v.name == release {
						targetDir = v.name
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("release %s not found", release)
				}
				args = args[1:] // Remove release from args
			} else {
				// Use previous release
				if len(versionDirs) < 2 {
					return fmt.Errorf("no previous version to rollback to")
				}
				targetDir = versionDirs[1].name
			}

			// 1. Find the next version number
			maxVersion := 0
			for _, v := range versionDirs {
				if v.version > maxVersion {
					maxVersion = v.version
				}
			}
			nextVersion := maxVersion + 1
			versionStr := fmt.Sprintf("v%d", nextVersion)

			// 2. Read values.yaml from the selected release to get the hash
			valuesYamlPath := filepath.Join(distDir, targetDir, "values.yaml")
			valuesYamlBytes, err := os.ReadFile(valuesYamlPath)
			if err != nil {
				return fmt.Errorf("failed to read values.yaml from selected release: %w", err)
			}
			hash := fmt.Sprintf("%x", sha1.Sum(valuesYamlBytes))[:8]

			// 3. Create new versioned directory
			newReleaseName := fmt.Sprintf("%s-%s", versionStr, hash)
			newReleaseDir := filepath.Join(distDir, newReleaseName)
			if err := os.MkdirAll(newReleaseDir, 0755); err != nil {
				return fmt.Errorf("failed to create new release directory: %w", err)
			}

			// 4. Copy all contents from the selected release to the new release directory
			err = filepath.Walk(filepath.Join(distDir, targetDir), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				relPath, err := filepath.Rel(filepath.Join(distDir, targetDir), path)
				if err != nil {
					return err
				}
				destPath := filepath.Join(newReleaseDir, relPath)
				if info.IsDir() {
					return os.MkdirAll(destPath, info.Mode())
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(destPath, data, info.Mode())
			})
			if err != nil {
				return fmt.Errorf("failed to copy release contents: %w", err)
			}

			// 5. Use newReleaseDir/docker for compose operation
			prevDockerDir := filepath.Join(newReleaseDir, "docker")
			// Find all compose files
			var composeFiles []string
			err = filepath.Walk(prevDockerDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() && (filepath.Base(path) == "docker-compose.yml" || filepath.Base(path) == "compose.yml") {
					relPath, err := filepath.Rel(prevDockerDir, path)
					if err != nil {
						return fmt.Errorf("failed to get relative path: %w", err)
					}
					composeFiles = append(composeFiles, relPath)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to find compose files: %w", err)
			}
			os.Setenv("COMPOSE_FILE", strings.Join(composeFiles, ":"))
			if err := os.Chdir(prevDockerDir); err != nil {
				return fmt.Errorf("failed to change to previous docker directory: %w", err)
			}
			// Pass through any additional args to docker compose
			dockerComposeArgs := append([]string{"compose"}, args...)
			dockerCompose := exec.Command("docker", dockerComposeArgs...)
			dockerCompose.Stdout = os.Stdout
			dockerCompose.Stderr = os.Stderr
			status := "SUCCESS"
			color := colorGreen
			if err := dockerCompose.Run(); err != nil {
				status = "FAIL!!!!"
				color = colorRed
			}
			fmt.Printf("\n+++++++++++++++++++++++++++++++++++++++\nRelease:  %s\nStatus:   %s%s%s\n+++++++++++++++++++++++++++++++++++++++\n", targetDir, color, status, colorReset)
			if status == "FAIL!!!!" {
				return fmt.Errorf("compose failed")
			}
			fmt.Printf("%sNew state version %s created from release %s%s\n", colorYellow, newReleaseName, targetDir, colorReset)
			return nil
		},
	}
	return cmd
}

func RunCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Handle dependency commands
	if args[0] == "dependency" {
		if len(args) < 2 {
			return fmt.Errorf("dependency command requires a subcommand (update/list)")
		}

		// Find the chart directory
		chartDir := "."
		if len(args) > 2 {
			chartDir = args[2]
		}

		switch args[1] {
		case "update":
			return UpdateDependencies(chartDir)
		case "list":
			return ListDependencies(chartDir)
		default:
			return fmt.Errorf("unknown dependency command: %s", args[1])
		}
	}

	// Handle other commands
	// ... existing code ...

	return nil
}

// renderTemplate renders a template file with the given values
func renderTemplate(templatePath string, values map[string]interface{}) (string, error) {
	renderer := tplt.NewRenderer(filepath.Dir(templatePath))
	return renderer.RenderTemplate(filepath.Base(templatePath), values)
}

// NewUpCommand creates a new up command
func NewUpCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Generate a new release and run docker compose",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get working directory
			workDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}

			// Load values
			chartLoader := chart.NewLoader(workDir)

			// Load main values
			mainValues, err := chartLoader.LoadValues(".")
			if err != nil {
				return fmt.Errorf("failed to load values: %w", err)
			}

			// Get additional values files
			valuesFiles, err := cmd.Flags().GetStringArray("values")
			if err != nil {
				return fmt.Errorf("failed to get values files: %w", err)
			}

			// Get values file
			valuesFile, err := cmd.Flags().GetString("values-file")
			if err != nil {
				return fmt.Errorf("failed to get values file: %w", err)
			}
			if valuesFile != "" {
				valuesFiles = append(valuesFiles, valuesFile)
			}

			// Load additional values
			additionalValues := make([]map[string]interface{}, 0)
			for _, file := range valuesFiles {
				vals, err := chartLoader.LoadValues(file)
				if err != nil {
					return fmt.Errorf("failed to load values from %s: %w", file, err)
				}
				additionalValues = append(additionalValues, vals.Values)
			}

			// Get set values
			setVals, err := cmd.Flags().GetStringArray("set")
			if err != nil {
				return fmt.Errorf("failed to get set values: %w", err)
			}

			// Get set-file values
			setFileVals, err := cmd.Flags().GetStringArray("set-file")
			if err != nil {
				return fmt.Errorf("failed to get set-file values: %w", err)
			}

			// Get set-string values
			setStringVals, err := cmd.Flags().GetStringArray("set-string")
			if err != nil {
				return fmt.Errorf("failed to get set-string values: %w", err)
			}

			// Merge values
			mergedValues := make(map[string]interface{})
			for k, v := range mainValues.Values {
				mergedValues[k] = v
			}

			// Merge additional values
			for _, vals := range additionalValues {
				for k, v := range vals {
					mergedValues[k] = v
				}
			}

			// Merge set values
			for _, val := range setVals {
				parts := strings.SplitN(val, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid set value: %s", val)
				}
				mergedValues[parts[0]] = parts[1]
			}

			// Merge set-file values
			for _, val := range setFileVals {
				parts := strings.SplitN(val, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid set-file value: %s", val)
				}
				content, err := os.ReadFile(parts[1])
				if err != nil {
					return fmt.Errorf("failed to read set-file %s: %w", parts[1], err)
				}
				mergedValues[parts[0]] = string(content)
			}

			// Merge set-string values
			for _, val := range setStringVals {
				parts := strings.SplitN(val, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid set-string value: %s", val)
				}
				mergedValues[parts[0]] = parts[1]
			}

			// Create output directory
			distDir := filepath.Join(workDir, "dist")
			var versionDirs []struct {
				name    string
				version int
			}
			entries, err := os.ReadDir(distDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						name := entry.Name()
						if len(name) > 2 && name[0] == 'v' {
							var n int
							if _, err := fmt.Sscanf(name, "v%d-", &n); err == nil {
								versionDirs = append(versionDirs, struct {
									name    string
									version int
								}{name, n})
							}
						}
					}
				}
			}
			sort.Slice(versionDirs, func(i, j int) bool {
				return versionDirs[i].version > versionDirs[j].version
			})

			// Get max releases from Chart.yaml or use default
			chart, err := loadChartYAML(workDir)
			if err != nil {
				return fmt.Errorf("failed to load Chart.yaml: %w", err)
			}
			maxReleases := 20 // default value
			if chart.MaxReleases > 0 {
				maxReleases = chart.MaxReleases
			}

			// Cleanup old releases
			if len(versionDirs) >= maxReleases {
				for _, v := range versionDirs[maxReleases:] {
					oldReleasePath := filepath.Join(distDir, v.name)
					logger.Debug("removing old release", "version", v.name)
					if err := os.RemoveAll(oldReleasePath); err != nil {
						logger.Warn("failed to remove old release", "version", v.name, "error", err)
					}
				}
			}

			maxVersion := 0
			if len(versionDirs) > 0 {
				maxVersion = versionDirs[0].version
			}

			// Generate new version
			newVersion := maxVersion + 1

			// Calculate config hash
			configBytes, err := json.Marshal(mergedValues)
			if err != nil {
				return fmt.Errorf("failed to marshal merged values: %w", err)
			}
			configHash := fmt.Sprintf("%x", sha1.Sum(configBytes))[:8]

			versionDir := filepath.Join(distDir, fmt.Sprintf("v%d-%s", newVersion, configHash))
			logger.Debug("creating new release", "version", newVersion, "hash", configHash)

			// Create version directory if it doesn't exist or if force is true
			if force {
				// Якщо force=true, видаляємо стару директорію якщо вона існує
				if err := os.RemoveAll(versionDir); err != nil {
					return fmt.Errorf("failed to remove old version directory: %w", err)
				}
			}
			if _, err := os.Stat(versionDir); os.IsNotExist(err) {
				if err := os.MkdirAll(versionDir, 0755); err != nil {
					return fmt.Errorf("failed to create version directory: %w", err)
				}

				// Save merged values
				mergedValuesFile := filepath.Join(versionDir, "values.yaml")
				valuesYamlBytes, err := yaml.Marshal(mergedValues)
				if err != nil {
					return fmt.Errorf("failed to marshal merged values to YAML: %w", err)
				}
				if err := os.WriteFile(mergedValuesFile, valuesYamlBytes, 0644); err != nil {
					return fmt.Errorf("failed to write values.yaml: %w", err)
				}

				// Create docker directory
				dockerDir := filepath.Join(versionDir, "docker")
				if err := os.MkdirAll(dockerDir, 0755); err != nil {
					return fmt.Errorf("failed to create docker directory: %w", err)
				}

				// Generate main compose file
				mainComposeFile := filepath.Join(dockerDir, "docker-compose.yml")
				mainTemplate := filepath.Join(workDir, "templates/docker-compose.yml.tmpl")
				mainContent, err := renderTemplate(mainTemplate, mergedValues)
				if err != nil {
					return fmt.Errorf("failed to render main template: %w", err)
				}
				if err := os.WriteFile(mainComposeFile, []byte(mainContent), 0644); err != nil {
					return fmt.Errorf("failed to write main compose file: %w", err)
				}

				// Generate compose files for dependencies
				chartsDir := filepath.Join(workDir, "charts")
				entries, err = os.ReadDir(chartsDir)
				if err != nil {
					return fmt.Errorf("failed to read charts directory: %w", err)
				}

				for _, entry := range entries {
					if !entry.IsDir() {
						continue
					}

					chartName := entry.Name()
					chartTemplate := filepath.Join(chartsDir, chartName, "templates/docker-compose.yml.tmpl")
					if _, err := os.Stat(chartTemplate); os.IsNotExist(err) {
						continue
					}

					// Створюємо контекст для шаблону
					chartValues, ok := mergedValues[chartName].(map[string]interface{})
					if !ok {
						return fmt.Errorf("invalid values type for child chart %s", chartName)
					}

					// Рекурсивно об'єднуємо значення
					mergedChartValues := make(map[string]interface{})

					// Додаємо глобальні значення
					if global, ok := mergedValues["global"].(map[string]interface{}); ok {
						mergedChartValues["global"] = global
					}

					// Додаємо значення з кореневого чарту
					if rootValues, ok := mergedValues["app"].(map[string]interface{}); ok {
						mergedChartValues["root"] = rootValues
					}

					// Додаємо значення з інших чартів
					for name, values := range mergedValues {
						if name != chartName && name != "global" && name != "app" {
							if vals, ok := values.(map[string]interface{}); ok {
								mergedChartValues[name] = vals
							}
						}
					}

					// Рекурсивно об'єднуємо значення з поточного чарту
					mergeValuesRecursively(mergedChartValues, chartValues)

					// Додаємо значення з поточного чарту в корінь
					if currentValues, ok := mergedValues[chartName].(map[string]interface{}); ok {
						mergeValuesRecursively(mergedChartValues, currentValues)
					}

					// Дебаг вивід значень для дочірнього чарту
					fmt.Printf("\nValues for chart %s:\n", chartName)
					valuesYaml, err := yaml.Marshal(mergedChartValues)
					if err != nil {
						return fmt.Errorf("failed to marshal values for chart %s: %w", chartName, err)
					}
					fmt.Printf("%s\n", string(valuesYaml))

					// Створюємо контекст для шаблону з правильним шляхом до значень
					templateContext := map[string]interface{}{
						"Values": mergedChartValues,
					}

					chartContent, err := renderTemplate(chartTemplate, templateContext)
					if err != nil {
						return fmt.Errorf("failed to render chart template %s: %w", chartName, err)
					}

					chartDir := filepath.Join(dockerDir, chartName)
					if err := os.MkdirAll(chartDir, 0755); err != nil {
						return fmt.Errorf("failed to create chart directory: %w", err)
					}

					chartFile := filepath.Join(chartDir, "docker-compose.yml")
					if err := os.WriteFile(chartFile, []byte(chartContent), 0644); err != nil {
						return fmt.Errorf("failed to write chart compose file: %w", err)
					}
				}
			} else {
				logger.Debug("reusing existing version", "version", filepath.Base(versionDir))
			}

			// Get network name from global values
			networkName := "default"
			if global, ok := mergedValues["global"].(map[string]interface{}); ok {
				if network, ok := global["network"].(map[string]interface{}); ok {
					if name, ok := network["name"].(string); ok {
						networkName = strings.ToLower(name)
					}
				}
			}

			logger.Debug("running pre-hooks")
			// Run pre-hooks
			if err := ExecuteHooks(chart, "pre", networkName); err != nil {
				return fmt.Errorf("pre-hooks failed: %w", err)
			}

			logger.Debug("running docker compose")
			// Run docker compose

			// Збираємо всі docker-compose файли
			var composeFiles []string
			composeFiles = append(composeFiles, "docker-compose.yml")

			// Додаємо файли з піддиректорій
			chartsDir := filepath.Join(versionDir, "docker")
			entries, err = os.ReadDir(chartsDir)
			if err != nil {
				return fmt.Errorf("failed to read charts directory: %w", err)
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				chartName := entry.Name()
				chartComposeFile := filepath.Join(chartName, "docker-compose.yml")
				if _, err := os.Stat(filepath.Join(chartsDir, chartComposeFile)); os.IsNotExist(err) {
					continue
				}

				composeFiles = append(composeFiles, chartComposeFile)
			}

			// Встановлюємо змінну середовища
			os.Setenv("COMPOSE_FILE", strings.Join(composeFiles, ":"))

			// Змінюємо поточну директорію на директорію з docker-compose файлами
			if err := os.Chdir(filepath.Join(versionDir, "docker")); err != nil {
				return fmt.Errorf("failed to change to docker directory: %w", err)
			}

			// Запускаємо docker compose
			composeArgs := []string{"compose"}
			// Фільтруємо аргументи, видаляючи --force
			filteredArgs := make([]string, 0, len(args))
			for _, arg := range args {
				if arg != "--force" {
					filteredArgs = append(filteredArgs, arg)
				}
			}
			composeArgs = append(composeArgs, filteredArgs...)
			if force {
				// Додаємо --force-recreate як аргумент для команди up
				for i, arg := range composeArgs {
					if arg == "up" {
						composeArgs = append(composeArgs[:i+1], append([]string{"--force-recreate"}, composeArgs[i+1:]...)...)
						break
					}
				}
			}
			composeCmd := exec.Command("docker", composeArgs...)
			composeCmd.Stdout = os.Stdout
			composeCmd.Stderr = os.Stderr
			if err := composeCmd.Run(); err != nil {
				return fmt.Errorf("docker compose failed: %w", err)
			}

			logger.Debug("running post-hooks")
			// Run post-hooks
			if err := ExecuteHooks(chart, "post", networkName); err != nil {
				return fmt.Errorf("post-hooks failed: %w", err)
			}

			// Print release info using fmt
			fmt.Printf("+++++++++++++++++++++++++++++++++++++++\n")
			fmt.Printf("Release:  %s\n", filepath.Base(versionDir))
			fmt.Printf("Status:   \033[32mSUCCESS\033[0m\n")
			fmt.Printf("+++++++++++++++++++++++++++++++++++++++\n")

			return nil
		},
	}

	// Add flags
	cmd.Flags().StringArrayP("set", "s", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().StringArray("set-file", []string{}, "Set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	cmd.Flags().StringArray("set-string", []string{}, "Set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().StringArrayP("values", "f", []string{}, "Specify values in a YAML file (can specify multiple)")
	cmd.Flags().String("values-file", "", "Specify values in a YAML file")
	cmd.Flags().BoolVar(&force, "force", false, "Force recreation of containers")

	return cmd
}

// mergeValuesRecursively рекурсивно об'єднує значення з source в target
func mergeValuesRecursively(target, source map[string]interface{}) {
	for key, value := range source {
		// Якщо значення є мапою, рекурсивно об'єднуємо
		if sourceMap, ok := value.(map[string]interface{}); ok {
			if targetMap, ok := target[key].(map[string]interface{}); ok {
				mergeValuesRecursively(targetMap, sourceMap)
			} else {
				target[key] = sourceMap
			}
		} else {
			// Інакше просто копіюємо значення
			target[key] = value
		}
	}
}
