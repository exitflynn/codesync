package diff

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateDiff(t *testing.T) {
	original := `func main() {
	fmt.Println("Hello")
	// A comment
	x := 10
	return
}`

	updated := `func main() {
	fmt.Println("Hello World")
	x := 10
	y := 20
	return
}`

	diff := GenerateDiff(original, updated)

	// Check that we have some hunks
	if len(diff.Hunks) == 0 {
		t.Error("Expected diff hunks, got none")
	}

	// Check stats
	if diff.Stats.Added == 0 && diff.Stats.Removed == 0 && diff.Stats.Changed == 0 {
		t.Error("Expected non-zero diff stats")
	}

	// Verify general structure
	if diff.Original != original || diff.Updated != updated {
		t.Error("Original or updated content not preserved in diff result")
	}
}

func TestGenerateUnifiedDiff(t *testing.T) {
	original := "line1\nline2\nline3\n"
	updated := "line1\nline2 modified\nline3\nline4\n"

	unifiedDiff := GenerateUnifiedDiff(original, updated, "original.txt", "updated.txt")

	// Check that we have a diff
	if unifiedDiff == "" {
		t.Error("Expected non-empty unified diff")
	}

	// Check that we have patch information
	if !strings.Contains(unifiedDiff, "@@ ") {
		t.Error("Expected patch header in unified diff")
	}
}

func TestApplyDiff(t *testing.T) {
	original := "line1\nline2\nline3\n"
	updated := "line1\nline2 modified\nline3\nline4\n"

	// Generate a diff
	diff := GenerateDiff(original, updated)

	// Apply the diff back to the original
	result, err := ApplyDiff(original, diff)
	if err != nil {
		t.Fatalf("Failed to apply diff: %v", err)
	}

	// Verify the result matches the updated
	if result != updated {
		t.Errorf("Applied diff does not match expected result.\nExpected:\n%s\nGot:\n%s", updated, result)
	}
}

func TestApplyPatch(t *testing.T) {
	original := "line1\nline2\nline3\n"
	updated := "line1\nline2 modified\nline3\nline4\n"

	// Create a temp file with original content
	tmpFile, err := os.CreateTemp("", "diff_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(original); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Generate a patch
	patch := GenerateUnifiedDiff(original, updated, "original.txt", "updated.txt")

	// Apply the patch to the file
	err = ApplyPatch(tmpFile.Name(), patch)
	if err != nil {
		t.Fatalf("Failed to apply patch: %v", err)
	}

	// Read the result
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read patched file: %v", err)
	}

	// Verify the result
	if string(content) != updated {
		t.Errorf("Applied patch does not match expected result.\nExpected:\n%s\nGot:\n%s", updated, string(content))
	}
}

func TestCompareFunctions(t *testing.T) {
	oldFunc := `func add(a, b int) int {
	return a + b
}`

	newFunc := `func add(a, b int) int {
	// Add two numbers
	result := a + b
	return result
}`

	diff := CompareFunctions(oldFunc, newFunc)

	// Verify we have changes
	if diff.Stats.Added == 0 && diff.Stats.Removed == 0 && diff.Stats.Changed == 0 {
		t.Error("Expected changes in function comparison")
	}
}

func TestFormatDiff(t *testing.T) {
	original := "line1\nline2\nline3\n"
	updated := "line1\nmodified\nline3\n"

	diff := GenerateDiff(original, updated)

	// Format without colors
	formatted := FormatDiff(diff, false)

	// Check that we have the expected markers
	if !strings.Contains(formatted, "+ modified") || !strings.Contains(formatted, "- line2") {
		t.Errorf("Expected formatted diff to contain added/removed markers, got:\n%s", formatted)
	}

	// Check that we have stats
	if !strings.Contains(formatted, "Changes:") {
		t.Error("Expected diff stats in formatted output")
	}
}

func TestWriteDiffToFile(t *testing.T) {
	original := "line1\nline2\nline3\n"
	updated := "line1\nmodified\nline3\n"

	diff := GenerateDiff(original, updated)

	// Create a temp file for the diff
	tmpFile, err := os.CreateTemp("", "diff_output_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Write diff to file
	err = WriteDiffToFile(diff, tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to write diff to file: %v", err)
	}

	// Read the file
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read diff file: %v", err)
	}

	// Verify content
	if !strings.Contains(string(content), "+ modified") || !strings.Contains(string(content), "- line2") {
		t.Errorf("Expected diff file to contain added/removed markers, got:\n%s", string(content))
	}
}
