package github

import "time"

type FileContent struct {
	Content string
}

type Commit struct {
	SHA  string
	Date time.Time
}

type GitHubClient interface {
	GetFile(owner, repo, path, revision string) (*FileContent, error)
	GetCommitsSince(owner, repo, path string, since time.Time, until string) ([]Commit, error)
	ExtractFunction(content, language, functionName string) (string, error)
}
