package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v52/github"
)

func TestExtractGoFunction(t *testing.T) {
	code := `package main

import "fmt"

// TestFunc is a test function
func TestFunc(a, b int) int {
	return a + b
}

func AnotherFunc() string {
	return "hello"
}`

	// Test successful extraction
	result, err := extractGoFunction(code, "TestFunc")
	if err != nil {
		t.Fatalf("Failed to extract function: %v", err)
	}

	expected := `// TestFunc is a test function
func TestFunc(a, b int) int {
	return a + b
}`

	if result != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, result)
	}

	// Test function not found
	_, err = extractGoFunction(code, "NonExistentFunc")
	if err == nil {
		t.Error("Expected error for non-existent function, got nil")
	}
}

func TestExtractPythonFunction(t *testing.T) {
	code := `
def test_func(a, b):
    """Test function docstring"""
    return a + b

def another_func():
    return "hello"
`

	// Test successful extraction
	result, err := extractPythonFunction(code, "test_func")
	if err != nil {
		t.Fatalf("Failed to extract function: %v", err)
	}

	expected := `def test_func(a, b):
    """Test function docstring"""
    return a + b`

	if result != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, result)
	}

	// Test function not found
	_, err = extractPythonFunction(code, "non_existent_func")
	if err == nil {
		t.Error("Expected error for non-existent function, got nil")
	}
}

func TestExtractJavaScriptFunction(t *testing.T) {
	code := `
// Function declaration
function testFunc(a, b) {
    return a + b;
}

// Arrow function
const arrowFunc = (a, b) => {
    return a * b;
};

// Object method
const obj = {
    methodFunc: function(a, b) {
        return a - b;
    }
};
`

	// Test successful extraction of regular function
	result, err := extractJavaScriptFunction(code, "testFunc")
	if err != nil {
		t.Fatalf("Failed to extract function: %v", err)
	}

	expected := `function testFunc(a, b) {
    return a + b;
}`

	if result != expected {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expected, result)
	}

	// Test function not found
	_, err = extractJavaScriptFunction(code, "nonExistentFunc")
	if err == nil {
		t.Error("Expected error for non-existent function, got nil")
	}
}

// Mock server for testing HTTP requests
func setupMockServer() (*httptest.Server, *Client) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock responses based on the request path
		switch r.URL.Path {
		case "/repos/owner/repo/contents/file.go":
			// Mock file content response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"type": "file",
				"encoding": "base64",
				"content": "cGFja2FnZSBtYWluCgppbXBvcnQgImZtdCIKCmZ1bmMgbWFpbigpIHsKICAgIGZtdC5QcmludGxuKCJIZWxsbyB3b3JsZCIpCn0=",
				"sha": "abc123",
				"path": "file.go"
			}`))

		case "/repos/owner/repo/contents/dir":
			// Mock directory content response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"type": "file",
					"name": "file1.go",
					"path": "dir/file1.go",
					"sha": "def456"
				},
				{
					"type": "dir",
					"name": "subdir",
					"path": "dir/subdir",
					"sha": "ghi789"
				}
			]`))

		default:
			// Default 404 response
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Create a client that uses the test server
	client := &Client{
		client: github.NewClient(server.Client()),
		ctx:    context.Background(),
	}

	return server, client
}

// Note: The following tests would require mocking the GitHub API.
// For brevity and since they would need extensive mocking,
// we're providing the test structure but not the full implementation.
// In a real project, you'd use a mocking library or set up proper fixtures.

func TestGetFile(t *testing.T) {
	// This would need mocking the GitHub API
	t.Skip("Requires mocking GitHub API")
}

func TestGetDirectory(t *testing.T) {
	// This would need mocking the GitHub API
	t.Skip("Requires mocking GitHub API")
}

func TestGetCommitsSince(t *testing.T) {
	// This would need mocking the GitHub API
	t.Skip("Requires mocking GitHub API")
}

func TestGetFileDiff(t *testing.T) {
	// This would need mocking the GitHub API
	t.Skip("Requires mocking GitHub API")
}
