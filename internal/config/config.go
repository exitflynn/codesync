package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SyncSource represents a source location for synced code
type SyncSource struct {
	Owner    string `yaml:"owner"`    // GitHub owner
	Repo     string `yaml:"repo"`     // GitHub repository name
	Path     string `yaml:"path"`     // Path to file or directory in repository
	Branch   string `yaml:"branch"`   // Branch to track (default: main)
	Revision string `yaml:"revision"` // Optional specific revision to pin to
}

// SyncTarget represents a destination location for synced code
type SyncTarget struct {
	Path      string `yaml:"path"`                // Local path to sync the code to
	Type      string `yaml:"type"`                // "file", "directory", or "function"
	Language  string `yaml:"language,omitempty"`  // Language for function-level sync (python, go, etc.)
	Function  string `yaml:"function,omitempty"`  // Function name for function-level sync
	Transform string `yaml:"transform,omitempty"` // Optional transformation script path
}

// SyncItem represents a single sync operation
type SyncItem struct {
	Name        string     `yaml:"name"`        // Human-readable name for this sync
	Description string     `yaml:"description"` // Optional description
	Source      SyncSource `yaml:"source"`      // Where to sync from
	Target      SyncTarget `yaml:"target"`      // Where to sync to
	Disabled    bool       `yaml:"disabled"`    // Whether this sync is currently disabled
}

// Config is the main configuration structure
type Config struct {
	Version      string     `yaml:"version"`      // Config schema version
	ProjectName  string     `yaml:"projectName"`  // Name of this project
	GitHubToken  string     `yaml:"githubToken"`  // GitHub API token (or use env var)
	SyncInterval string     `yaml:"syncInterval"` // How often to check for updates (cron format)
	Items        []SyncItem `yaml:"items"`        // List of things to sync
	NotifyOnly   bool       `yaml:"notifyOnly"`   // If true, don't auto-generate PRs
}

// LoadConfig loads the configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	// Set default path if not provided
	if path == "" {
		path = "codesync.yaml"
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Use environment variable for GitHub token if not in config
	if config.GitHubToken == "" {
		config.GitHubToken = os.Getenv("GITHUB_TOKEN")
	}

	// Set default values
	for i := range config.Items {
		if config.Items[i].Source.Branch == "" {
			config.Items[i].Source.Branch = "main"
		}
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Version == "" {
		return fmt.Errorf("config version is required")
	}

	if len(c.Items) == 0 {
		return fmt.Errorf("no sync items defined")
	}

	for i, item := range c.Items {
		// Skip disabled items
		if item.Disabled {
			continue
		}

		// Validate source
		if item.Source.Owner == "" || item.Source.Repo == "" || item.Source.Path == "" {
			return fmt.Errorf("item %d (%s): incomplete source configuration", i, item.Name)
		}

		// Validate target
		if item.Target.Path == "" || item.Target.Type == "" {
			return fmt.Errorf("item %d (%s): incomplete target configuration", i, item.Name)
		}

		// Validate target type
		if item.Target.Type != "file" && item.Target.Type != "directory" && item.Target.Type != "function" {
			return fmt.Errorf("item %d (%s): invalid target type '%s'", i, item.Name, item.Target.Type)
		}

		// Validate function sync
		if item.Target.Type == "function" && (item.Target.Language == "" || item.Target.Function == "") {
			return fmt.Errorf("item %d (%s): function sync requires language and function name", i, item.Name)
		}
	}

	return nil
}

// GetAbsolutePath returns the absolute path for a target
func (t *SyncTarget) GetAbsolutePath(basePath string) (string, error) {
	if filepath.IsAbs(t.Path) {
		return t.Path, nil
	}

	return filepath.Abs(filepath.Join(basePath, t.Path))
}
