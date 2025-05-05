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

type State struct {
	LastSync          time.Time `json:"lastSync"`
	LastCommitID      string    `json:"lastCommitID"`
	CurrentLocalHash  string    `json:"currentLocalHash"`
	CurrentRemoteHash string    `json:"currentRemoteHash"`
	HasLocalChanges   bool      `json:"hasLocalChanges"`
	HasRemoteChanges  bool      `json:"hasRemoteChanges"`
}

type SyncReport struct {
	SyncItem     config.SyncItem
	State        State
	UpdatedFiles []string
	Diffs        map[string]*diff.DiffResult
	Errors       []string
}

type SyncManager struct {
	config       *config.Config
	githubClient *github.Client
	stateDir     string
}

func NewSyncManager(cfg *config.Config, stateDir string) (*SyncManager, error) {
	if cfg.GitHubToken == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	githubClient := github.NewClient(cfg.GitHubToken)

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

func (sm *SyncManager) SyncAll() ([]*SyncReport, error) {
	var reports []*SyncReport

	for _, item := range sm.config.Items {
		if item.Disabled {
			continue
		}

		report, err := sm.SyncItem(item)
		if err != nil {
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

func (sm *SyncManager) SyncItem(item config.SyncItem) (*SyncReport, error) {
	report := &SyncReport{
		SyncItem: item,
		Diffs:    make(map[string]*diff.DiffResult),
		Errors:   []string{},
	}

	state, err := sm.loadState(item.Name)
	if err != nil {
		state = State{
			LastSync: time.Time{},
		}
	}

	hasLocalChanges, localHash, err := sm.checkLocalChanges(item, state.CurrentLocalHash)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Error checking local changes: %v", err))
	} else {
		state.HasLocalChanges = hasLocalChanges
		state.CurrentLocalHash = localHash
	}

	hasRemoteChanges, remoteContent, remoteHash, commitID, err := sm.checkRemoteChanges(item, state.LastCommitID)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Error checking remote changes: %v", err))
	} else {
		state.HasRemoteChanges = hasRemoteChanges
		state.CurrentRemoteHash = remoteHash
	}

	if state.HasLocalChanges && state.HasRemoteChanges {
		report.Errors = append(report.Errors, "Both local and remote have changes. Manual resolution required.")

		state.LastSync = time.Now()
		if err := sm.saveState(item.Name, state); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("Failed to save state: %v", err))
		}

		return report, fmt.Errorf("conflict detected")
	}

	if state.HasRemoteChanges {
		switch item.Target.Type {
		case "file":
			if err := sm.updateLocalFile(item, remoteContent); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("Failed to update local file: %v", err))
				return report, err
			}
			report.UpdatedFiles = append(report.UpdatedFiles, item.Target.Path)

		case "directory":
			report.Errors = append(report.Errors, "Directory sync not fully implemented yet")

		case "function":
			if err := sm.updateLocalFunction(item, remoteContent); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("Failed to update local function: %v", err))
				return report, err
			}
			report.UpdatedFiles = append(report.UpdatedFiles, item.Target.Path)
		}

		state.LastCommitID = commitID
		state.HasRemoteChanges = false
		state.HasLocalChanges = false
	}

	state.LastSync = time.Now()
	report.State = state

	if err := sm.saveState(item.Name, state); err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Failed to save state: %v", err))
	}

	return report, nil
}

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

	currentHash := calculateHash(string(content))

	hasChanges := currentHash != lastHash
	return hasChanges, currentHash, nil
}

func (sm *SyncManager) checkRemoteChanges(item config.SyncItem, lastCommitID string) (bool, string, string, string, error) {
	commits, err := sm.githubClient.GetCommitsSince(
		item.Source.Owner,
		item.Source.Repo,
		item.Source.Path,
		time.Time{},
		lastCommitID,
	)
	if err != nil {
		return false, "", "", "", fmt.Errorf("failed to get commits: %w", err)
	}

	if len(commits) == 0 {
		return false, "", "", "", nil
	}

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

	remoteHash := calculateHash(content.Content)

	hasChanges := remoteHash != item.Source.Revision && latestCommit.SHA != lastCommitID
	return hasChanges, content.Content, remoteHash, latestCommit.SHA, nil
}

func (sm *SyncManager) updateLocalFile(item config.SyncItem, remoteContent string) error {
	absPath, err := item.Target.GetAbsolutePath("")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(remoteContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (sm *SyncManager) updateLocalFunction(item config.SyncItem, remoteContent string) error {
	functionContent, err := sm.githubClient.ExtractFunction(
		remoteContent,
		item.Target.Language,
		item.Target.Function,
	)
	if err != nil {
		return fmt.Errorf("failed to extract function: %w", err)
	}

	absPath, err := item.Target.GetAbsolutePath("")
	if err != nil {
		return err
	}

	localContent, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	updatedContent, err := replaceFunction(
		string(localContent),
		item.Target.Language,
		item.Target.Function,
		functionContent,
	)
	if err != nil {
		return fmt.Errorf("failed to replace function: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(updatedContent), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

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

func replaceGoFunction(content, functionName, newFunction string) (string, error) {
	start := strings.Index(content, "func "+functionName)
	if start == -1 {
		return "", fmt.Errorf("function %s not found", functionName)
	}

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

func replacePythonFunction(content, functionName, newFunction string) (string, error) {
	start := strings.Index(content, "def "+functionName)
	if start == -1 {
		return "", fmt.Errorf("function %s not found", functionName)
	}

	end := start
	indent := 0
	for i := start; i < len(content); i++ {
		if content[i] == '\n' {
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

func replaceJavaScriptFunction(content, functionName, newFunction string) (string, error) {
	start := strings.Index(content, "function "+functionName)
	if start == -1 {
		start = strings.Index(content, functionName+" = ")
		if start == -1 {
			return "", fmt.Errorf("function %s not found", functionName)
		}
	}

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

func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
}

func calculateHash(content string) string {
	return fmt.Sprintf("%x", len(content))
}
