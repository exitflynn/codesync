package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/google/go-github/v52/github"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	ctx    context.Context
}

// FileInfo represents information about a file in a GitHub repository
type FileInfo struct {
	Content  string
	Path     string
	SHA      string
	Updated  time.Time
	CommitID string
}

// CommitInfo represents information about a commit
type CommitInfo struct {
	SHA       string
	Message   string
	Author    string
	Timestamp time.Time
}

// NewClient creates a new GitHub API client
func NewClient(token string) *Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &Client{
		client: client,
		ctx:    ctx,
	}
}

// GetFile retrieves a file from a GitHub repository
func (c *Client) GetFile(owner, repo, path, ref string) (*FileInfo, error) {
	fileContent, directoryContent, _, err := c.client.Repositories.GetContents(
		c.ctx,
		owner,
		repo,
		path,
		&github.RepositoryContentGetOptions{Ref: ref},
	)

	// Handle directory case
	if directoryContent != nil {
		return nil, errors.New("path points to a directory, not a file")
	}

	// Handle file case
	if err != nil {
		return nil, fmt.Errorf("error getting file content: %w", err)
	}

	if fileContent == nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("error decoding content: %w", err)
	}

	// Get commit information for the file
	commits, _, err := c.client.Repositories.ListCommits(
		c.ctx,
		owner,
		repo,
		&github.CommitsListOptions{
			Path: path,
			ListOptions: github.ListOptions{
				PerPage: 1,
			},
		},
	)

	var commitID string
	var updated time.Time
	if err == nil && len(commits) > 0 {
		commitID = commits[0].GetSHA()
		if commits[0].Commit != nil && commits[0].Commit.Author != nil {
			updated = commits[0].Commit.Author.GetDate().Time
		}
	}

	return &FileInfo{
		Content:  content,
		Path:     fileContent.GetPath(),
		SHA:      fileContent.GetSHA(),
		Updated:  updated,
		CommitID: commitID,
	}, nil
}

// GetDirectory retrieves all files from a directory in a GitHub repository
func (c *Client) GetDirectory(owner, repo, path, ref string) (map[string]*FileInfo, error) {
	result := make(map[string]*FileInfo)

	_, directoryContent, _, err := c.client.Repositories.GetContents(
		c.ctx,
		owner,
		repo,
		path,
		&github.RepositoryContentGetOptions{Ref: ref},
	)

	if err != nil {
		return nil, fmt.Errorf("error getting directory content: %w", err)
	}

	// If it's not a directory
	if directoryContent == nil {
		return nil, errors.New("path does not point to a directory")
	}

	// Process each item in the directory
	for _, item := range directoryContent {
		switch item.GetType() {
		case "file":
			// Get the file content
			fileInfo, err := c.GetFile(owner, repo, item.GetPath(), ref)
			if err != nil {
				continue // Skip files that can't be retrieved
			}
			result[item.GetPath()] = fileInfo

		case "dir":
			// Recursively get the directory content
			subdir, err := c.GetDirectory(owner, repo, item.GetPath(), ref)
			if err != nil {
				continue // Skip directories that can't be retrieved
			}
			// Add all files from subdirectory
			for k, v := range subdir {
				result[k] = v
			}
		}
	}

	return result, nil
}

// GetCommitsSince gets all commits for a file since a specific date or commit
func (c *Client) GetCommitsSince(owner, repo, path string, since time.Time, sinceCommit string) ([]CommitInfo, error) {
	var result []CommitInfo

	options := &github.CommitsListOptions{
		Path: path,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	if !since.IsZero() {
		options.Since = since
	}

	foundSinceCommit := sinceCommit == ""
	page := 1

	for {
		options.Page = page
		commits, resp, err := c.client.Repositories.ListCommits(c.ctx, owner, repo, options)
		if err != nil {
			return nil, fmt.Errorf("error listing commits: %w", err)
		}

		for _, commit := range commits {
			// If tracking since a specific commit, skip until we find it
			if !foundSinceCommit {
				if commit.GetSHA() == sinceCommit {
					foundSinceCommit = true
					break
				}
				continue
			}

			// Skip the sinceCommit itself
			if commit.GetSHA() == sinceCommit {
				continue
			}

			// Add commit info
			commitInfo := CommitInfo{
				SHA:     commit.GetSHA(),
				Message: commit.Commit.GetMessage(),
				Author:  commit.Commit.Author.GetName(),
			}

			if commit.Commit.Author != nil {
				commitInfo.Timestamp = commit.Commit.Author.GetDate().Time
			}

			result = append(result, commitInfo)
		}

		// Stop if we've found our commit or reached the end
		if foundSinceCommit || resp.NextPage == 0 {
			break
		}

		page = resp.NextPage
	}

	return result, nil
}

// GetFileDiff gets the diff between two versions of a file
func (c *Client) GetFileDiff(owner, repo, path, baseRef, headRef string) (string, error) {
	// Get the comparison between the two refs
	comparison, _, err := c.client.Repositories.CompareCommits(
		c.ctx,
		owner,
		repo,
		baseRef,
		headRef,
		&github.ListOptions{},
	)

	if err != nil {
		return "", fmt.Errorf("error comparing commits: %w", err)
	}

	// Find the file in the comparison files
	targetPath := path
	for _, file := range comparison.Files {
		if file.GetFilename() == targetPath || file.GetPreviousFilename() == targetPath {
			// Return the patch if available
			if file.GetPatch() != "" {
				return file.GetPatch(), nil
			}

			// If no patch is available (e.g., for binary files), just indicate change
			return fmt.Sprintf("File %s was changed (no text diff available)", targetPath), nil
		}
	}

	// If we didn't find the file in the comparison
	return "", fmt.Errorf("file %s was not changed between %s and %s", path, baseRef, headRef)
}

// GetRawFile gets the raw content of a file without processing
func (c *Client) GetRawFile(owner, repo, path, ref string) ([]byte, error) {
	// Construct the raw URL
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		owner, repo, ref, path)

	// Create a new request
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received status code %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

// ExtractFunction attempts to extract a function from a file
func (c *Client) ExtractFunction(content, language, functionName string) (string, error) {
	switch language {
	case "go":
		return extractGoFunction(content, functionName)
	case "python":
		return extractPythonFunction(content, functionName)
	case "javascript", "js":
		return extractJavaScriptFunction(content, functionName)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
}

// isNamedFunction checks if a node is a function declaration with the given name.
func isNamedFunction(node *sitter.Node, name string) bool {
	if node.Type() == "function_declaration" {
		identifier := node.ChildByFieldName("name")
		return identifier != nil && identifier.Content([]byte(name)) == name
	}

	return false
}

func walk(n *sitter.Node, result **sitter.Node, functionName string) {
	if *result != nil {
		return // already found
	}

	if isNamedFunction(n, functionName) {
		*result = n
		return
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child != nil {
			walk(child, result, functionName)
		}
	}
}

// Helper functions to extract code by language
func extractGoFunction(content, functionName string) (string, error) {
	// Create a new file set
	fset := token.NewFileSet()

	// Parse the file content
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("error parsing Go file: %w", err)
	}

	// Find the function declaration
	var funcDecl *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			if fd.Name.Name == functionName {
				funcDecl = fd
				return false
			}
		}
		return true
	})

	if funcDecl == nil {
		return "", fmt.Errorf("function %s not found", functionName)
	}

	// Get the function's position in the source
	start := fset.Position(funcDecl.Pos())
	end := fset.Position(funcDecl.End())

	// Extract the function code
	lines := strings.Split(content, "\n")
	if start.Line < 1 || end.Line > len(lines) {
		return "", fmt.Errorf("invalid function position")
	}

	// Include comments before the function
	startLine := start.Line - 1
	for startLine > 0 && strings.HasPrefix(strings.TrimSpace(lines[startLine-1]), "//") {
		startLine--
	}

	return strings.Join(lines[startLine:end.Line], "\n"), nil
}

func extractPythonFunction(content, functionName string) (string, error) {
	// For Python, we'll use a simpler approach with string manipulation
	// since the ANTLR Python parser requires a grammar file
	lines := strings.Split(content, "\n")

	start := -1
	inFunction := false
	indent := -1
	docstring := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		// Count leading spaces for indentation
		lineIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		// Look for function definition
		if !inFunction && strings.HasPrefix(trimmedLine, "def "+functionName+"(") {
			start = i
			inFunction = true
			indent = lineIndent
			continue
		}

		// Handle docstrings
		if inFunction && (strings.HasPrefix(trimmedLine, `"""`) || strings.HasPrefix(trimmedLine, `'''`)) {
			docstring = !docstring
			continue
		}

		// If we're in a function and hit a line with same or less indentation, we're done
		if inFunction && !docstring && lineIndent <= indent && !strings.HasPrefix(trimmedLine, "#") {
			return strings.Join(lines[start:i], "\n"), nil
		}
	}

	if start == -1 {
		return "", fmt.Errorf("function %s not found", functionName)
	}

	// If we reached the end of the file while still in the function
	if inFunction {
		return strings.Join(lines[start:], "\n"), nil
	}

	return "", fmt.Errorf("function %s seems incomplete", functionName)
}

func extractJavaScriptFunction(content, functionName string) (string, error) {
	// Create a new Tree-sitter parser
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	// Parse the content
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(content))
	if err != nil {
		return "", fmt.Errorf("error parsing JavaScript content: %w", err)
	}
	defer tree.Close()

	// Find the function node
	var functionNode *sitter.Node
	walk(tree.RootNode(), &functionNode, functionName)

	if functionNode == nil {
		return "", fmt.Errorf("function %s not found", functionName)
	}

	// Extract the function content
	start := functionNode.StartByte()
	end := functionNode.EndByte()
	return content[start:end], nil
}
