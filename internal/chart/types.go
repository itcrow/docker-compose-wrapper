package chart

// Chart represents a Docker Compose chart
type Chart struct {
	Name         string       `yaml:"name"`
	Version      string       `yaml:"version"`
	Dependencies []Dependency `yaml:"dependencies"`
}

// Dependency represents a chart dependency
type Dependency struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// Values represents the configuration values
type Values struct {
	Version string                 `yaml:"version"`
	Global  GlobalValues           `yaml:"Global"`
	Values  map[string]interface{} `yaml:",inline"`
}

// GlobalValues represents global configuration values
type GlobalValues struct {
	ProjectName            string        `yaml:"projectName"`
	Environment            string        `yaml:"environment"`
	DefaultImagePullPolicy string        `yaml:"defaultImagePullPolicy"`
	Network                NetworkValues `yaml:"network"`
}

// NetworkValues represents network configuration
type NetworkValues struct {
	Name   string `yaml:"name"`
	Alias  string `yaml:"alias"`
	Driver string `yaml:"driver"`
}
 