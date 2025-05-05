# CodeSync

A robust solution for managing "copy-pasted" code dependencies in open source projects.

## Overview

CodeSync helps you manage code that you've copied from other projects rather than imported as formal dependencies. This is common in open source projects when adding a full dependency would be overkill for just one function or file.

### Key Features

- **Track "indirect dependencies"** - Monitor specific files, functions, or directories copied from other repositories
- **Update notifications** - Get notified when upstream code changes
- **Smart diffing** - See exactly what changed in the upstream code
- **Auto-update PRs** - Automatically create pull requests to merge upstream changes
- **Flexible syncing** - Sync entire files, specific functions, or whole directories

## Why Use CodeSync?

Unlike package managers or tools like Dependabot that track formal dependencies, CodeSync focuses on code that has been directly copied into your project. This addresses several common needs:

- **Avoiding dependency bloat** - When you only need one small function from a large library
- **Stability** - Control exactly when and how you incorporate upstream changes
- **Cross-language support** - Works with any language, not just package-based ecosystems
- **Function-level tracking** - Track individual functions rather than entire files

## Getting Started

### Installation

```bash
# Install via Go
go install github.com/exitflynn/codesync/cmd/codesync@latest

# Or download binary releases from GitHub
curl -L https://github.com/exitflynn/codesync/releases/latest/download/codesync-$(uname -s)-$(uname -m) -o codesync
chmod +x codesync
```

### Basic Usage

1. Create a `codesync.yaml` file in your project root:

```yaml
version: "1.0"
projectName: "my-project"
githubToken: "" # Leave empty to use GITHUB_TOKEN env var
syncInterval: "0 0 * * *" # Daily at midnight
notifyOnly: false # Generate PRs automatically

items:
  - name: "logger"
    description: "Logging utility from main repository"
    source:
      owner: "your-org"
      repo: "common-utils"
      path: "pkg/logger/logger.go"
      branch: "main"
    target:
      path: "internal/logger/logger.go"
      type: "file"
```

2. Run CodeSync to check for updates:

```bash
# Check for updates once
codesync check

# Run as a daemon to check periodically
codesync daemon

# Check a specific item
codesync check --item logger
```

## Configuration Options

### Global Configuration

| Field | Description | Required | Default |
|-------|-------------|----------|---------|
| `version` | Config schema version | Yes | - |
| `projectName` | Name of your project | Yes | - |
| `githubToken` | GitHub API token | No | Uses `GITHUB_TOKEN` env var |
| `syncInterval` | How often to check (cron format) | No | `0 0 * * *` (daily) |
| `notifyOnly` | Only notify, don't create PRs | No | `false` |

### Sync Items

Each item in the `items` array describes a piece of code to sync:

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Human-readable identifier | Yes |
| `description` | Purpose of this sync | No |
| `disabled` | Whether to skip this item | No |
| `source` | Where to sync from | Yes |
| `target` | Where to sync to | Yes |

#### Source Configuration

| Field | Description | Required | Default |
|-------|-------------|----------|---------|
| `owner` | GitHub owner/org | Yes | - |
| `repo` | GitHub repository name | Yes | - |
| `path` | Path to file/directory | Yes | - |
| `branch` | Branch to track | No | `main` |
| `revision` | Specific commit to pin to | No | - |

#### Target Configuration

| Field | Description | Required | Default |
|-------|-------------|----------|---------|
| `path` | Local path | Yes | - |
| `type` | `file`, `directory`, or `function` | Yes | - |
| `language` | Language for function extraction | For `function` type | - |
| `function` | Function name to extract | For `function` type | - |
| `transform` | Script to transform code | No | - |

## Running as a GitHub Action

Create a workflow file `.github/workflows/codesync.yml`:

```yaml
name: CodeSync

on:
  schedule:
    - cron: '0 0 * * *'  # Run daily at midnight
  workflow_dispatch:      # Allow manual triggers

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
          
      - name: Install CodeSync
        run: go install github.com/yourusername/codesync/cmd/codesync@latest
          
      - name: Run CodeSync
        run: codesync check
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## VSCode Extension

The CodeSync VSCode extension (coming soon) provides:

- Visual indicators for synced code
- One-click updates
- Diff view for upstream changes
- Quick creation of new sync configurations

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.