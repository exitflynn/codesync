package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary test config file
	content := `
version: "1.0"
projectName: "test-project"
githubToken: "test-token"
syncInterval: "0 */12 * * *"
notifyOnly: true
items:
  - name: "util-functions"
    description: "Common utility functions"
    source:
      owner: "acme"
      repo: "utils"
      path: "src/utils/strings.go"
      branch: "main"
    target:
      path: "pkg/utils/strings.go"
      type: "file"
  - name: "parser-function"
    source:
      owner: "acme"
      repo: "parsers"
      path: "src/json/parse.go"
    target:
      path: "internal/parser/json.go"
      type: "function"
      language: "go"
      function: "ParseJSON"
  - name: "disabled-sync"
    disabled: true
    source:
      owner: "acme"
      repo: "helpers"
      path: "helpers/"
    target:
      path: "pkg/helpers"
      type: "directory"
`
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Test loading the config
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify the parsed config values
	t.Run("Basic Config Fields", func(t *testing.T) {
		if cfg.Version != "1.0" {
			t.Errorf("Expected version 1.0, got %s", cfg.Version)
		}
		if cfg.ProjectName != "test-project" {
			t.Errorf("Expected project name 'test-project', got %s", cfg.ProjectName)
		}
		if cfg.GitHubToken != "test-token" {
			t.Errorf("Expected GitHub token 'test-token', got %s", cfg.GitHubToken)
		}
		if cfg.SyncInterval != "0 */12 * * *" {
			t.Errorf("Expected sync interval '0 */12 * * *', got %s", cfg.SyncInterval)
		}
		if !cfg.NotifyOnly {
			t.Errorf("Expected notifyOnly to be true")
		}
	})

	t.Run("Sync Items Count", func(t *testing.T) {
		if len(cfg.Items) != 3 {
			t.Errorf("Expected 3 sync items, got %d", len(cfg.Items))
		}
	})

	t.Run("First Item Details", func(t *testing.T) {
		item := cfg.Items[0]
		if item.Name != "util-functions" {
			t.Errorf("Expected name 'util-functions', got %s", item.Name)
		}
		if item.Source.Owner != "acme" {
			t.Errorf("Expected source owner 'acme', got %s", item.Source.Owner)
		}
		if item.Target.Type != "file" {
			t.Errorf("Expected target type 'file', got %s", item.Target.Type)
		}
	})

	t.Run("Function Sync Item", func(t *testing.T) {
		item := cfg.Items[1]
		if item.Target.Language != "go" {
			t.Errorf("Expected language 'go', got %s", item.Target.Language)
		}
		if item.Target.Function != "ParseJSON" {
			t.Errorf("Expected function 'ParseJSON', got %s", item.Target.Function)
		}
	})

	t.Run("Disabled Item", func(t *testing.T) {
		if !cfg.Items[2].Disabled {
			t.Errorf("Expected item to be disabled")
		}
	})
}

func TestConfigValidation(t *testing.T) {
	t.Run("Valid Config", func(t *testing.T) {
		cfg := &Config{
			Version: "1.0",
			Items: []SyncItem{
				{
					Name: "test-item",
					Source: SyncSource{
						Owner:  "owner",
						Repo:   "repo",
						Path:   "path/to/file.go",
						Branch: "main",
					},
					Target: SyncTarget{
						Path: "local/path/file.go",
						Type: "file",
					},
				},
			},
		}

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validation should pass, but got error: %v", err)
		}
	})

	t.Run("Missing Version", func(t *testing.T) {
		cfg := &Config{
			Items: []SyncItem{
				{
					Name: "test-item",
					Source: SyncSource{
						Owner:  "owner",
						Repo:   "repo",
						Path:   "path/to/file.go",
						Branch: "main",
					},
					Target: SyncTarget{
						Path: "local/path/file.go",
						Type: "file",
					},
				},
			},
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Validation should fail due to missing version")
		}
	})

	t.Run("No Items", func(t *testing.T) {
		cfg := &Config{
			Version: "1.0",
			Items:   []SyncItem{},
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Validation should fail due to no items")
		}
	})

	t.Run("Invalid Target Type", func(t *testing.T) {
		cfg := &Config{
			Version: "1.0",
			Items: []SyncItem{
				{
					Name: "test-item",
					Source: SyncSource{
						Owner:  "owner",
						Repo:   "repo",
						Path:   "path/to/file.go",
						Branch: "main",
					},
					Target: SyncTarget{
						Path: "local/path/file.go",
						Type: "invalid-type", // Invalid type
					},
				},
			},
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Validation should fail due to invalid target type")
		}
	})

	t.Run("Missing Function Details", func(t *testing.T) {
		cfg := &Config{
			Version: "1.0",
			Items: []SyncItem{
				{
					Name: "test-item",
					Source: SyncSource{
						Owner:  "owner",
						Repo:   "repo",
						Path:   "path/to/file.go",
						Branch: "main",
					},
					Target: SyncTarget{
						Path: "local/path/file.go",
						Type: "function",
						// Missing Language and Function
					},
				},
			},
		}

		if err := cfg.Validate(); err == nil {
			t.Error("Validation should fail due to missing function details")
		}
	})
}

func TestGetAbsolutePath(t *testing.T) {
	t.Run("Relative Path", func(t *testing.T) {
		target := SyncTarget{
			Path: "relative/path/file.go",
		}

		// Use current directory as base
		basePath, _ := os.Getwd()
		absPath, err := target.GetAbsolutePath(basePath)

		if err != nil {
			t.Fatalf("GetAbsolutePath failed: %v", err)
		}

		expected, _ := filepath.Abs(filepath.Join(basePath, "relative/path/file.go"))
		if absPath != expected {
			t.Errorf("Expected %s, got %s", expected, absPath)
		}
	})

	t.Run("Absolute Path", func(t *testing.T) {
		// Use absolute path based on OS
		var absolutePath string
		if os.PathSeparator == '/' {
			absolutePath = "/absolute/path/file.go"
		} else {
			absolutePath = "C:\\absolute\\path\\file.go"
		}

		target := SyncTarget{
			Path: absolutePath,
		}

		absPath, err := target.GetAbsolutePath("any/base/path")

		if err != nil {
			t.Fatalf("GetAbsolutePath failed: %v", err)
		}

		if absPath != absolutePath {
			t.Errorf("Expected %s, got %s", absolutePath, absPath)
		}
	})
}
