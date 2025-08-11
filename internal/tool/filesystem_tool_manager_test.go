package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fpt/go-gennai-cli/internal/repository"
	"github.com/fpt/go-gennai-cli/pkg/message"
)

func TestFileSystemToolManager_SecurityFeatures(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	allowedSubDir := filepath.Join(tempDir, "allowed")
	forbiddenDir := filepath.Join(tempDir, "forbidden")

	if err := os.MkdirAll(allowedSubDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}
	if err := os.MkdirAll(forbiddenDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create test files
	testFile := filepath.Join(allowedSubDir, "test.txt")
	secretFile := filepath.Join(allowedSubDir, "secret.env")
	forbiddenFile := filepath.Join(forbiddenDir, "forbidden.txt")

	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(secretFile, []byte("API_KEY=secret123"), 0644); err != nil {
		t.Fatalf("Failed to create secret file: %v", err)
	}
	if err := os.WriteFile(forbiddenFile, []byte("forbidden content"), 0644); err != nil {
		t.Fatalf("Failed to create forbidden file: %v", err)
	}

	// Create filesystem tool manager with restricted access
	config := repository.FileSystemConfig{
		AllowedDirectories: []string{allowedSubDir},
		BlacklistedFiles:   []string{"*.env", "*secret*"},
	}

	manager := NewFileSystemToolManager(config, tempDir)
	ctx := context.Background()

	t.Run("AllowedDirectoryAccess", func(t *testing.T) {
		// Should be able to read allowed file
		result, err := manager.handleReadFile(ctx, map[string]any{
			"path": testFile,
		})
		if err != nil {
			t.Errorf("Expected success reading allowed file, got error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("Expected success, got error: %s", result.Error)
		}
		if !strings.Contains(result.Text, "test content") {
			t.Errorf("Expected file content, got: %s", result.Text)
		}
	})

	t.Run("ForbiddenDirectoryAccess", func(t *testing.T) {
		// Should not be able to read file in forbidden directory
		result, err := manager.handleReadFile(ctx, map[string]any{
			"path": forbiddenFile,
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("Expected access denied error for forbidden directory")
		}
		if !strings.Contains(result.Error, "not within allowed directories") {
			t.Errorf("Expected directory access error, got: %s", result.Error)
		}
	})

	t.Run("BlacklistedFileAccess", func(t *testing.T) {
		// Should not be able to read blacklisted file
		result, err := manager.handleReadFile(ctx, map[string]any{
			"path": secretFile,
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("Expected access denied error for blacklisted file")
		}
		if !strings.Contains(result.Error, "blacklisted") {
			t.Errorf("Expected blacklist error, got: %s", result.Error)
		}
	})

	t.Run("ReadWriteSemantics", func(t *testing.T) {
		writeFile := filepath.Join(allowedSubDir, "write_test.txt")

		// Attempt to write without reading first - should fail
		result, err := manager.handleWriteFile(ctx, map[string]any{
			"path":    writeFile,
			"content": "new content",
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("Expected read-write semantics violation")
		}
		if !strings.Contains(result.Error, "was not read before write") {
			t.Errorf("Expected read-write semantics error, got: %s", result.Error)
		}

		// Read the file first
		_, err = manager.handleReadFile(ctx, map[string]any{
			"path": writeFile,
		})
		if err != nil {
			t.Errorf("Unexpected error reading for write semantics: %v", err)
		}

		// Now write should succeed
		result, err = manager.handleWriteFile(ctx, map[string]any{
			"path":    writeFile,
			"content": "new content",
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result.Error != "" {
			t.Errorf("Expected write success after read, got error: %s", result.Error)
		}
	})

	t.Run("TimestampValidation", func(t *testing.T) {
		timestampFile := filepath.Join(allowedSubDir, "timestamp_test.txt")

		// Create and read a file
		if err := os.WriteFile(timestampFile, []byte("original"), 0644); err != nil {
			t.Fatalf("Failed to create timestamp test file: %v", err)
		}

		// Read the file to establish timestamp
		_, err := manager.handleReadFile(ctx, map[string]any{
			"path": timestampFile,
		})
		if err != nil {
			t.Fatalf("Failed to read file for timestamp test: %v", err)
		}

		// Simulate external modification
		time.Sleep(10 * time.Millisecond) // Ensure different timestamp
		if err := os.WriteFile(timestampFile, []byte("externally modified"), 0644); err != nil {
			t.Fatalf("Failed to modify file externally: %v", err)
		}

		// Attempt to write - should fail due to external modification
		result, err := manager.handleWriteFile(ctx, map[string]any{
			"path":    timestampFile,
			"content": "my content",
		})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result.Error == "" {
			t.Error("Expected timestamp validation failure")
		}
		if !strings.Contains(result.Error, "was modified after last read") {
			t.Errorf("Expected timestamp error, got: %s", result.Error)
		}
	})
}

func TestFileSystemToolManager_ToolRegistration(t *testing.T) {
	// Create a temporary directory for this test
	tempDir := t.TempDir()

	config := repository.FileSystemConfig{
		AllowedDirectories: []string{"."},
		BlacklistedFiles:   []string{},
	}

	manager := NewFileSystemToolManager(config, tempDir)

	// Verify all expected tools are registered
	expectedTools := []string{
		"read_file",
		"write_file",
		"edit_file",
		"list_directory",
		"find_file",
	}

	toolsMap := manager.GetTools()
	if len(toolsMap) != len(expectedTools) {
		t.Errorf("Expected %d tools, got %d", len(expectedTools), len(toolsMap))
	}

	for _, expectedName := range expectedTools {
		tool, exists := manager.GetTool(message.ToolName(expectedName))
		if !exists {
			t.Errorf("Expected tool %s not found", expectedName)
		}
		if tool.Name() != message.ToolName(expectedName) {
			t.Errorf("Tool name mismatch: expected %s, got %s", expectedName, tool.Name())
		}
	}
}

func TestFileSystemToolManager_ResolvePath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create filesystem tool manager with working directory set to tempDir
	config := repository.FileSystemConfig{
		AllowedDirectories: []string{tempDir},
	}
	manager := NewFileSystemToolManager(config, tempDir)

	tests := []struct {
		name        string
		inputPath   string
		expectError bool
		checkSuffix string
	}{
		{
			name:        "RelativePath",
			inputPath:   "test.txt",
			expectError: false,
			checkSuffix: filepath.Join(tempDir, "test.txt"),
		},
		{
			name:        "AbsolutePath",
			inputPath:   filepath.Join(tempDir, "absolute.txt"),
			expectError: false,
			checkSuffix: filepath.Join(tempDir, "absolute.txt"),
		},
		{
			name:        "DotPath",
			inputPath:   ".",
			expectError: false,
			checkSuffix: tempDir,
		},
		{
			name:        "SubdirectoryPath",
			inputPath:   "subdir/file.txt",
			expectError: false,
			checkSuffix: filepath.Join(tempDir, "subdir", "file.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := manager.resolvePath(tt.inputPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none for path %s", tt.inputPath)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for path %s: %v", tt.inputPath, err)
				return
			}

			if !strings.HasSuffix(resolved, tt.checkSuffix) {
				t.Errorf("Expected resolved path to end with %s, but got %s", tt.checkSuffix, resolved)
			}

			// Verify that resolved path is absolute
			if !filepath.IsAbs(resolved) {
				t.Errorf("Expected absolute path but got relative path: %s", resolved)
			}
		})
	}
}
