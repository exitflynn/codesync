// package sync

// import (
// 	"os"
// 	"path/filepath"
// 	"testing"
// 	"time"

// 	"github.com/exitflynn/codesync/internal/config"
// 	"github.com/exitflynn/codesync/internal/github"
// 	"github.com/exitflynn/codesync/mocks"
// 	"go.uber.org/mock/gomock"
// )

// func TestSyncManager(t *testing.T) {
// 	// Create a temporary directory for testing
// 	tempDir := t.TempDir()
// 	stateDir := filepath.Join(tempDir, ".codesync")

// 	// Create a test config
// 	cfg := &config.Config{
// 		Version:     "1.0",
// 		ProjectName: "test-project",
// 		GitHubToken: "test-token",
// 		Items: []config.SyncItem{
// 			{
// 				Name: "test-file",
// 				Source: config.SyncSource{
// 					Owner: "test-owner",
// 					Repo:  "test-repo",
// 					Path:  "test.go",
// 				},
// 				Target: config.SyncTarget{
// 					Path: filepath.Join(tempDir, "test.go"),
// 					Type: "file",
// 				},
// 			},
// 			{
// 				Name: "test-function",
// 				Source: config.SyncSource{
// 					Owner: "test-owner",
// 					Repo:  "test-repo",
// 					Path:  "utils.go",
// 				},
// 				Target: config.SyncTarget{
// 					Path:     filepath.Join(tempDir, "utils.go"),
// 					Type:     "function",
// 					Language: "go",
// 					Function: "TestFunc",
// 				},
// 			},
// 		},
// 	}

// 	// Create mock controller
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()

// 	// Create mock GitHub client
// 	mockClient := mocks.NewMockGitHubClient(ctrl)

// 	// Set up expectations
// 	mockClient.EXPECT().
// 		GetFile("test-owner", "test-repo", "test.go", gomock.Any()).
// 		Return(&github.FileContent{Content: "test content"}, nil).
// 		AnyTimes()

// 	mockClient.EXPECT().
// 		GetCommitsSince("test-owner", "test-repo", "test.go", gomock.Any(), gomock.Any()).
// 		Return([]github.Commit{
// 			{
// 				SHA:  "test-commit",
// 				Date: time.Now(),
// 			},
// 		}, nil).
// 		AnyTimes()

// 	mockClient.EXPECT().
// 		ExtractFunction(gomock.Any(), "go", "TestFunc").
// 		Return("func TestFunc() {}", nil).
// 		AnyTimes()

// 	// Create sync manager
// 	sm, err := NewSyncManager(cfg, stateDir)
// 	if err != nil {
// 		t.Fatalf("Failed to create sync manager: %v", err)
// 	}

// 	// Override the GitHub client with mock
// 	sm.githubClient = mockClient

// 	t.Run("SyncAll", func(t *testing.T) {
// 		reports, err := sm.SyncAll()
// 		if err != nil {
// 			t.Fatalf("SyncAll failed: %v", err)
// 		}

// 		if len(reports) != 2 {
// 			t.Errorf("Expected 2 reports, got %d", len(reports))
// 		}
// 	})

// 	t.Run("SyncItem", func(t *testing.T) {
// 		report, err := sm.SyncItem(cfg.Items[0])
// 		if err != nil {
// 			t.Fatalf("SyncItem failed: %v", err)
// 		}

// 		if report.SyncItem.Name != "test-file" {
// 			t.Errorf("Expected report for test-file, got %s", report.SyncItem.Name)
// 		}
// 	})

// 	t.Run("LoadState", func(t *testing.T) {
// 		// Create a test state
// 		testState := State{
// 			LastSync:          time.Now(),
// 			LastCommitID:      "test-commit",
// 			CurrentLocalHash:  "test-local-hash",
// 			CurrentRemoteHash: "test-remote-hash",
// 		}

// 		// Save the state
// 		if err := sm.saveState("test-file", testState); err != nil {
// 			t.Fatalf("Failed to save state: %v", err)
// 		}

// 		// Load the state
// 		loadedState, err := sm.loadState("test-file")
// 		if err != nil {
// 			t.Fatalf("Failed to load state: %v", err)
// 		}

// 		if loadedState.LastCommitID != testState.LastCommitID {
// 			t.Errorf("Expected LastCommitID %s, got %s", testState.LastCommitID, loadedState.LastCommitID)
// 		}
// 	})

// 	t.Run("CheckLocalChanges", func(t *testing.T) {
// 		// Create a test file
// 		testFile := filepath.Join(tempDir, "test.go")
// 		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
// 			t.Fatalf("Failed to create test file: %v", err)
// 		}

// 		hasChanges, hash, err := sm.checkLocalChanges(cfg.Items[0], "")
// 		if err != nil {
// 			t.Fatalf("checkLocalChanges failed: %v", err)
// 		}

// 		if !hasChanges {
// 			t.Error("Expected changes to be detected")
// 		}

// 		if hash == "" {
// 			t.Error("Expected non-empty hash")
// 		}
// 	})

// 	t.Run("UpdateLocalFile", func(t *testing.T) {
// 		testFile := filepath.Join(tempDir, "test.go")
// 		content := "new content"

// 		if err := sm.updateLocalFile(cfg.Items[0], content); err != nil {
// 			t.Fatalf("updateLocalFile failed: %v", err)
// 		}

// 		// Verify the file was updated
// 		data, err := os.ReadFile(testFile)
// 		if err != nil {
// 			t.Fatalf("Failed to read updated file: %v", err)
// 		}

// 		if string(data) != content {
// 			t.Errorf("Expected content %q, got %q", content, string(data))
// 		}
// 	})
// }
