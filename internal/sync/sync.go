package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/exitflynn/codesync/internal/config"
	"github.com/exitflynn/codesync/internal/diff"
	"github.com/exitflynn/codesync/internal/github"
)

// State represents the current state of a sync item
type State struct {
	LastSync          time.Time `json:"lastSync"`
	LastCommitID      string    `json:"lastCommitID"`
	CurrentLocalHash  string    `json:"currentLocalHash"`
	CurrentRemoteHash string    `json:"currentRemoteHash"`
	HasLocalChanges   bool      `json:"hasLocalChanges"`
	HasRemoteChanges  bool      `json:"hasRemoteChanges"`
}

// SyncReport represents the result of a sync operation
type SyncReport struct {
	SyncItem     config.SyncItem
	State        State
	UpdatedFiles []string
	Diffs        map[string]*diff.DiffResult
	Errors       []string
}

// SyncManager handles the syncing process
type SyncManager struct {
	config       *config.Config
	githubClient *github.Client
	stateDir     string
}

// NewSyncManager creates a new sync manager
func NewSyncManager(cfg *config.Config, stateDir string) (*SyncManager, error) {
	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	// Initialize GitHub client
	githubClient := github.NewClient(cfg.GitHubToken)

	// Ensure state directory exists
	if stateDir == "" {
		stateDir = ".codesync"
	}

	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &SyncManager{
		config:       cfg,
		githubClient: githubClient,
		stateDir:     stateDir,
	}, nil
}

// SyncAll runs sync for all items in the config
func (sm *SyncManager) SyncAll() ([]*SyncReport, error) {
	var reports []*SyncReport

	for _, item := range sm.config.Items {
		// Skip disabled items
		if item.Disabled {
			continue
		}

		report, err := sm.SyncItem(item)
		if err != nil {
			// Add error but continue with other items
			if report == nil {
				report = &SyncReport{
					SyncItem: item,
					Errors:   []string{err.Error()},
				}
			} else {
				report.Errors = append(report.Errors, err.Error())
			}
		}

		reports = append(reports, report)
	}

	return reports, nil
}

// SyncItem synchronizes a single item
func (sm *SyncManager) SyncItem(item config.SyncItem) (*SyncReport, error) {
	report := &SyncReport{
		SyncItem: item,
		Diffs:    make(map[string]*diff.DiffResult),
		Errors:   []string{},
	}

	// Load previous state if it exists
	state, err := sm.loadState(item.Name)
	if err != nil {
		// Not returning error, just initializing a new state
		state = State{
			LastSync: time.Time{}, // Zero time
		}
	}

	// Check if we have local changes
	hasLocalChanges, localHash, err := sm.checkLocalChanges(item, state.CurrentLocalHash)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Error checking local changes: %v", err))
	} else {
		state.HasLocalChanges = hasLocalChanges
		state.CurrentLocalHash = localHash
	}

	// Check for remote changes
	hasRemoteChanges, remoteContent, remoteHash, commitID, err := sm.checkRemoteChanges(item, state.LastCommitID)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Error checking remote changes: %v", err))
	} else {
		state.HasRemoteChanges = hasRemoteChanges
		state.CurrentRemoteHash = remoteHash
	}

	// Handle conflicts
	if state.HasLocalChanges && state.HasRemoteChanges {
		report.Errors = append(report.Errors, "Both local and remote have changes. Manual resolution required.")

		// Save the state regardless
		state.LastSync = time.Now()
		if err := sm.saveState(item.Name, state); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Failed to save state: %v", err))
		}

		return report, fmt.Errorf("conflict detected")
	}

	// Apply remote changes to local if there are any
	if state.HasRemoteChanges {
		switch item.Target.Type {
		case "file":
			if err := sm.updateLocalFile(item, remoteContent); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("Failed to update local file: %v", err))
				return report, err
			}
			report.UpdatedFiles = append(report.UpdatedFiles, item.Target.Path)

		case "directory":
			// For directories, we would need to handle multiple files
			// This is more complex and would require additional implementation
			report.Errors = append(report.Errors, "Directory sync not fully implemented yet")

		case "function":
			if err := sm.updateLocalFunction(item, remoteContent); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("Failed to update local function: %v", err))
				return report, err
			}
			report.UpdatedFiles = append(report.UpdatedFiles, item.Target.Path)
		}

		// Update the state
		state.LastCommitID = commitID
		state.HasRemoteChanges = false
		state.HasLocalChanges = false
	}

	// Update sync timestamp
	state.LastSync = time.Now()
	report.State = state

	// Save the updated state
	if err := sm.saveState(item.Name, state); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to save state: %v", err))
	}

	return report, nil
}

// loadState loads the state for a sync item
func (sm *SyncManager) loadState(itemName string) (State, error) {
	statePath := filepath.Join(sm.stateDir, sanitizeFilename(itemName)+".json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		return State{}, err
	}

	var state State
	err = json.Unmarshal(data, &state)
	return state, err
}

// saveState saves the state for a sync item
func (sm *SyncManager) saveState(itemName string, state State) error {
	statePath := filepath.Join(sm.stateDir, sanitizeFilename(itemName)+".json")

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(statePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// checkLocalChanges checks if there are local changes since last sync
func (sm *SyncManager) checkLocalChanges(item config.SyncItem, lastHash string) (bool, string, error) {
	// Get absolute path
	absPath, err := item.Target.GetAbsolutePath("")
	if err != nil {
		return false, "", err
	}

	// Read local file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read local file: %w", err)
	}

	// Calculate hash of current content
	currentHash := calculateHash(string(content))

	// Compare with last known hash
	hasChanges := currentHash != lastHash
	return hasChanges, currentHash, nil
}

// checkRemoteChanges checks if there are remote changes since last sync
func (sm *SyncManager) checkRemoteChanges(item config.SyncItem, lastCommitID string) (bool, string, string, string, error) {
	// Get the latest commit for the file
	commits, err := sm.githubClient.GetCommitsSince(
		item.Source.Owner,
		item.Source.Repo,
		item.Source.Path,
		time.Time{}, // No time limit
		lastCommitID,
	)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return false, "", "", "", nil
	}

	// Get the latest content
	latestCommit := commits[0]
	content, err := sm.githubClient.GetFile(
		item.Source.Owner,
		item.Source.Repo,
		item.Source.Path,
		latestCommit.SHA,
	)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to get file content: %w", err)
	}

	// Calculate hash of remote content
	remoteHash := calculateHash(content.Content)

	// Compare with last known hash
	hasChanges := remoteHash != item.Source.Revision && latestCommit.SHA != lastCommitID
	return hasChanges, content.Content, remoteHash, latestCommit.SHA, nil
}

// updateLocalFile updates a local file with remote content
func (sm *SyncManager) updateLocalFile(item config.SyncItem, remoteContent string) error {
	// Get absolute path
	absPath, err := item.Target.GetAbsolutePath("")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write the content
	if err := os.WriteFile(absPath, []byte(remoteContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// updateLocalFunction updates a local function with remote content
func (sm *SyncManager) updateLocalFunction(item config.SyncItem, remoteContent string) error {
	// Extract the function from remote content
	functionContent, err := sm.githubClient.ExtractFunction(
		remoteContent,
		item.Target.Language,
		item.Target.Function,
	)
	if err != nil {
		return fmt.Errorf("failed to extract function: %w", err)
	}

	// Get absolute path
	absPath, err := item.Target.GetAbsolutePath("")
	if err != nil {
		return err
	}

	// Read local file
	localContent, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Replace the function in local content
	updatedContent, err := replaceFunction(
		string(localContent),
		item.Target.Language,
		item.Target.Function,
		functionContent,
	)
	if err != nil {
		return fmt.Errorf("failed to replace function: %w", err)
	}

	// Write the updated content
	if err := os.WriteFile(absPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// replaceFunction replaces a function in the local content with the updated version
func replaceFunction(localContent, language, functionName, newFunctionContent string) (string, error) {
	switch language {
	case "go":
		return replaceGoFunction(localContent, functionName, newFunctionContent)
	case "python":
		return replacePythonFunction(localContent, functionName, newFunctionContent)
	case "javascript":
		return replaceJavaScriptFunction(localContent, functionName, newFunctionContent)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

// replaceGoFunction replaces a Go function in the content
func replaceGoFunction(content, functionName, newFunction string) (string, error) {
	// TODO: Implement proper Go function replacement using go/parser
	// For now, use simple string replacement
	start := strings.Index(content, "func "+functionName)
	if start == -1 {
		return "", fmt.Errorf("function %s not found", functionName)
	}

	// Find the end of the function
	end := start
	braceCount := 0
	for i := start; i < len(content); i++ {
		if content[i] == '{' {
			braceCount++
		} else if content[i] == '}' {
			braceCount--
			if braceCount == 0 {
				end = i + 1
				break
			}
		}
	}

	return content[:start] + newFunction + content[end:], nil
}

// replacePythonFunction replaces a Python function in the content
func replacePythonFunction(content, functionName, newFunction string) (string, error) {
	// TODO: Implement proper Python function replacement
	// For now, use simple string replacement
	start := strings.Index(content, "def "+functionName)
	if start == -1 {
		return "", fmt.Errorf("function %s not found", functionName)
	}

	// Find the end of the function
	end := start
	indent := 0
	for i := start; i < len(content); i++ {
		if content[i] == '\n' {
			// Check if next line has less indentation
			j := i + 1
			for j < len(content) && (content[j] == ' ' || content[j] == '\t') {
				j++
			}
			if j < len(content) && content[j] != ' ' && content[j] != '\t' && content[j] != '\n' {
				if j-i-1 <= indent {
					end = i
					break
				}
			}
		}
	}

	return content[:start] + newFunction + content[end:], nil
}

// replaceJavaScriptFunction replaces a JavaScript function in the content
func replaceJavaScriptFunction(content, functionName, newFunction string) (string, error) {
	// TODO: Implement proper JavaScript function replacement
	// For now, use simple string replacement
	start := strings.Index(content, "function "+functionName)
	if start == -1 {
		// Try arrow function
		start = strings.Index(content, functionName+" = ")
		if start == -1 {
			return "", fmt.Errorf("function %s not found", functionName)
		}
	}

	// Find the end of the function
	end := start
	braceCount := 0
	for i := start; i < len(content); i++ {
		if content[i] == '{' {
			braceCount++
		} else if content[i] == '}' {
			braceCount--
			if braceCount == 0 {
				end = i + 1
				break
			}
		}
	}

	return content[:start] + newFunction + content[end:], nil
}

// Helper functions
func sanitizeFilename(name string) string {
	// Replace invalid characters with underscores
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
}

func calculateHash(content string) string {
	// Simple hash function for now
	// In production, use a proper hash function like SHA-256
	return fmt.Sprintf("%x", len(content))
}
