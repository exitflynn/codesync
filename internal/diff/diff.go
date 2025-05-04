package diff

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// DiffResult represents the difference between two files
type DiffResult struct {
	Original string
	Updated  string
	Hunks    []DiffHunk
	Stats    DiffStats
}

// DiffHunk represents a chunk of changes
type DiffHunk struct {
	LineStart int
	Content   string
	Added     bool
	Removed   bool
}

// DiffStats contains statistics about the differences
type DiffStats struct {
	Added   int
	Removed int
	Changed int
}

// GenerateDiff creates a diff between two strings
func GenerateDiff(original, updated string) *DiffResult {
	dmp := diffmatchpatch.New()

	// Generate line-mode diff
	a, b, c := dmp.DiffLinesToChars(original, updated)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, c)

	// Process the diff into our structure
	result := &DiffResult{
		Original: original,
		Updated:  updated,
		Hunks:    make([]DiffHunk, 0),
		Stats:    DiffStats{},
	}

	lineNumber := 1

	for _, d := range diffs {
		if d.Type == diffmatchpatch.DiffEqual {
			// For equal parts, just update the line count
			lineNumber += strings.Count(d.Text, "\n")
			continue
		}

		hunk := DiffHunk{
			LineStart: lineNumber,
			Content:   d.Text,
		}

		// Update stats and hunk properties based on diff type
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			hunk.Added = true
			result.Stats.Added += strings.Count(d.Text, "\n") + 1
		case diffmatchpatch.DiffDelete:
			hunk.Removed = true
			result.Stats.Removed += strings.Count(d.Text, "\n") + 1
			lineNumber += strings.Count(d.Text, "\n")
		}

		result.Hunks = append(result.Hunks, hunk)
	}

	// Calculate changed lines (estimate)
	result.Stats.Changed = min(result.Stats.Added, result.Stats.Removed)
	result.Stats.Added -= result.Stats.Changed
	result.Stats.Removed -= result.Stats.Changed

	return result
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GenerateUnifiedDiff creates a unified diff format string
func GenerateUnifiedDiff(original, updated, originalName, updatedName string) string {
	dmp := diffmatchpatch.New()
	patches := dmp.PatchMake(original, updated)
	return dmp.PatchToText(patches)
}

// ApplyDiff applies the changes from a DiffResult to a string
func ApplyDiff(original string, result *DiffResult) (string, error) {
	dmp := diffmatchpatch.New()
	patches := dmp.PatchMake(original, result.Updated)
	newText, successes := dmp.PatchApply(patches, original)

	// Check if all patches were applied
	for _, success := range successes {
		if !success {
			return "", fmt.Errorf("failed to apply some patches")
		}
	}

	return newText, nil
}

// ApplyPatch applies a patch in unified diff format to a file
func ApplyPatch(filePath, patch string) error {
	// Read the original file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Apply the patch
	dmp := diffmatchpatch.New()
	patches, err := dmp.PatchFromText(patch)
	if err != nil {
		return fmt.Errorf("error parsing patch: %w", err)
	}

	newText, successes := dmp.PatchApply(patches, string(content))

	// Check if all patches were applied
	for _, success := range successes {
		if !success {
			return fmt.Errorf("failed to apply some patches")
		}
	}

	// Write the patched content back to the file
	return os.WriteFile(filePath, []byte(newText), 0644)
}

// CompareFunctions compares two versions of a function and returns a diff
func CompareFunctions(oldFunc, newFunc string) *DiffResult {
	return GenerateDiff(oldFunc, newFunc)
}

// FormatDiff formats a DiffResult for display
func FormatDiff(diff *DiffResult, colorize bool) string {
	var sb strings.Builder

	// Output stats
	sb.WriteString(fmt.Sprintf("Changes: +%d -%d ~%d\n\n",
		diff.Stats.Added, diff.Stats.Removed, diff.Stats.Changed))

	// Output hunks
	for _, hunk := range diff.Hunks {
		// Add header for each hunk
		sb.WriteString(fmt.Sprintf("@@ Line %d @@\n", hunk.LineStart))

		// Add content with prefixes
		lines := strings.Split(hunk.Content, "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}

			if hunk.Added {
				if colorize {
					sb.WriteString("\033[32m+ " + line + "\033[0m\n")
				} else {
					sb.WriteString("+ " + line + "\n")
				}
			} else if hunk.Removed {
				if colorize {
					sb.WriteString("\033[31m- " + line + "\033[0m\n")
				} else {
					sb.WriteString("- " + line + "\n")
				}
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// WriteDiffToFile writes a diff to a file
func WriteDiffToFile(diff *DiffResult, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating diff file: %w", err)
	}
	defer f.Close()

	// Write plain text diff
	_, err = io.WriteString(f, FormatDiff(diff, false))
	if err != nil {
		return fmt.Errorf("error writing diff: %w", err)
	}

	return nil
}
