package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fpt/go-gennai-cli/internal/repository"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/pkg/errors"
)

var errNotInAllowedDirectory = errors.New("file access denied: path is not within allowed directories")

// FileSystemToolManager provides secure file system operations with safety controls
type FileSystemToolManager struct {
	// Access control
	allowedDirectories []string // Directories where file operations are allowed
	blacklistedFiles   []string // Files that cannot be read (to prevent secret leaks)

	// Working directory context
	workingDir string // Working directory for resolving relative paths

	// Read-write semantics tracking
	fileReadTimestamps map[string]time.Time // Track when files were last read
	mu                 sync.RWMutex         // Thread safety for timestamp tracking

	// Tool registry
	tools map[message.ToolName]message.Tool
}

// NewFileSystemToolManager creates a new secure filesystem tool manager
func NewFileSystemToolManager(config repository.FileSystemConfig, workingDir string) *FileSystemToolManager {
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		absWorkingDir = workingDir
	}

	// Ensure working directory is always in allowed directories for backward compatibility
	allowedDirs := ensureWorkingDirectoryInAllowedList(config.AllowedDirectories, absWorkingDir)

	manager := &FileSystemToolManager{
		allowedDirectories: allowedDirs,
		blacklistedFiles:   config.BlacklistedFiles,
		workingDir:         absWorkingDir,
		fileReadTimestamps: make(map[string]time.Time),
		tools:              make(map[message.ToolName]message.Tool),
	}

	// Register filesystem tools
	manager.registerFileSystemTools()

	return manager
}

// ensureWorkingDirectoryInAllowedList ensures the working directory is always included
// in the allowed directories list for backward compatibility.
// It returns a new slice with the working directory included if not already present.
func ensureWorkingDirectoryInAllowedList(configuredDirectories []string, absWorkingDir string) []string {
	// Create a copy of the original slice to avoid modifying the input
	allowedDirs := make([]string, len(configuredDirectories))
	copy(allowedDirs, configuredDirectories)

	// Check if working directory is already present
	workingDirPresent := false
	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue // Skip invalid directories
		}
		if absDir == absWorkingDir {
			workingDirPresent = true
			break
		}
	}

	// Add working directory if not already present
	if !workingDirPresent {
		allowedDirs = append(allowedDirs, absWorkingDir)
	}

	return allowedDirs
}

// Implement domain.ToolManager interface
func (m *FileSystemToolManager) GetTool(name message.ToolName) (message.Tool, bool) {
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *FileSystemToolManager) GetTools() map[message.ToolName]message.Tool {
	return m.tools
}

func (m *FileSystemToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	tool, exists := m.tools[name]
	if !exists {
		return message.NewToolResultError(fmt.Sprintf("tool %s not found", name)), nil
	}

	handler := tool.Handler()
	return handler(ctx, args)
}

func (m *FileSystemToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, args []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	tool := &fileSystemTool{
		name:        name,
		description: description,
		arguments:   args,
		handler:     handler,
	}
	m.tools[name] = tool
}

// registerFileSystemTools registers all secure filesystem tools
func (m *FileSystemToolManager) registerFileSystemTools() {
	// Read file tool
	m.RegisterTool("read_file", "Read file content with access control",
		[]message.ToolArgument{
			{
				Name:        "path",
				Description: "Path to the file to read",
				Required:    true,
				Type:        "string",
			},
		},
		m.handleReadFile)

	// Write file tool
	m.RegisterTool("write_file", "Write file content with read-write semantics validation. IMPORTANT: You must provide both 'path' (file path) and 'content' (full file content) parameters.",
		[]message.ToolArgument{
			{
				Name:        "path",
				Description: "Path to the file to write (required string parameter)",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "content",
				Description: "Full content to write to the file (required string parameter)",
				Required:    true,
				Type:        "string",
			},
		},
		m.handleWriteFile)

	// Enhanced Edit tool (Claude Code-style)
	m.RegisterTool("edit_file", "Advanced file editing with precise string replacement (Claude Code-style)",
		[]message.ToolArgument{
			{
				Name:        "file_path",
				Description: "Absolute path to the file to edit",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "old_string",
				Description: "Exact multiline string to replace (must match exactly once unless replace_all=true)",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "new_string",
				Description: "Precise replacement content (multiline supported)",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "replace_all",
				Description: "Replace all occurrences of old_string (default: false, replaces only first occurrence)",
				Required:    false,
				Type:        "boolean",
			},
		},
		m.handleEnhancedEdit)

	// Directory listing (read-only, still secured)
	m.RegisterTool("list_directory", "List directory contents with access control",
		[]message.ToolArgument{
			{
				Name:        "path",
				Description: "Path to the directory to list (defaults to current directory)",
				Required:    false,
				Type:        "string",
			},
		},
		m.handleListDirectory)

	// Find file tool (restricted to allowed directories)
	m.RegisterTool("find_file", "Find files by name pattern using find command",
		[]message.ToolArgument{
			{
				Name:        "name_pattern",
				Description: "File name pattern to search for (supports wildcards like *.go, *test*, etc.)",
				Required:    true,
				Type:        "string",
			},
			{
				Name:        "path",
				Description: "Directory path to search in (must be within allowed directories)",
				Required:    false,
				Type:        "string",
			},
			{
				Name:        "type",
				Description: "File type filter: 'f' for files, 'd' for directories, or 'both' (default: 'f')",
				Required:    false,
				Type:        "string",
			},
		},
		m.handleFindFile)
}

// Security validation methods

// abs resolves a path to absolute form relative to the tool's working directory
// This replaces filepath.Abs to avoid resolving against the process's current working directory
func (m *FileSystemToolManager) abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// For relative paths, resolve against the tool's working directory
	resolved := filepath.Join(m.workingDir, path)

	// Clean the path to handle . and .. elements
	return filepath.Clean(resolved), nil
}

// resolvePath resolves a path relative to the working directory
func (m *FileSystemToolManager) resolvePath(path string) (string, error) {
	// If path is already absolute, check if it's within working directory
	if filepath.IsAbs(path) {
		// Get absolute form of working directory using our own method
		absWorkingDir, err := m.abs(m.workingDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve working directory: %v", err)
		}

		// Check if the absolute path is within the working directory
		if strings.HasPrefix(path, absWorkingDir+string(os.PathSeparator)) || path == absWorkingDir {
			return path, nil
		}

		// If absolute path is outside working directory, reject it
		return "", fmt.Errorf("absolute path %s is outside working directory %s", path, m.workingDir)
	}

	// Resolve relative path against working directory using our own method
	return m.abs(path)
}

// isPathAllowed checks if a file path is within allowed directories
func (m *FileSystemToolManager) isPathAllowed(path string) error {
	// Note: allowedDirectories always contains at least the working directory (ensured in constructor)

	// Expect path to already be absolute (resolved by caller)
	absPath := path

	// Check if path is within any allowed directory
	for _, allowedDir := range m.allowedDirectories {
		allowedAbs, err := m.abs(allowedDir)
		if err != nil {
			continue // Skip invalid allowed directory
		}

		// Check if the file path is under the allowed directory
		if strings.HasPrefix(absPath, allowedAbs+string(os.PathSeparator)) || absPath == allowedAbs {
			return nil
		}
	}

	return errNotInAllowedDirectory
}

// isFileBlacklisted checks if a file is in the blacklist
func (m *FileSystemToolManager) isFileBlacklisted(path string) error {
	fileName := filepath.Base(path)
	absPath := path // Expect path to already be absolute (resolved by caller)

	for _, blacklisted := range m.blacklistedFiles {
		// Check both filename and full path patterns
		if matched, _ := filepath.Match(blacklisted, fileName); matched {
			return fmt.Errorf("file access denied: %s matches blacklisted pattern %s", fileName, blacklisted)
		}
		if matched, _ := filepath.Match(blacklisted, absPath); matched {
			return fmt.Errorf("file access denied: %s matches blacklisted pattern %s", absPath, blacklisted)
		}
		// Also check for exact matches
		if fileName == blacklisted || absPath == blacklisted {
			return fmt.Errorf("file access denied: %s is blacklisted", path)
		}
	}

	return nil
}

// validateReadWriteSemantics checks if a write operation is safe based on read timestamps
func (m *FileSystemToolManager) validateReadWriteSemantics(path string) error {
	m.mu.RLock()
	lastReadTime, wasRead := m.fileReadTimestamps[path]
	m.mu.RUnlock()

	if !wasRead {
		return fmt.Errorf("read-write semantics violation: file %s was not read before write attempt", path)
	}

	// Check if file was modified since last read
	fileInfo, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check file modification time: %v", err)
	}

	if err == nil && fileInfo.ModTime().After(lastReadTime) {
		return fmt.Errorf("read-write semantics violation: file %s was modified after last read", path)
	}

	return nil
}

// recordFileRead records that a file was successfully read
func (m *FileSystemToolManager) recordFileRead(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fileReadTimestamps[path] = time.Now()
}

// Tool handlers with security

func (m *FileSystemToolManager) handleReadFile(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	pathParam, ok := args["path"].(string)
	if !ok {
		return message.NewToolResultError("path parameter is required"), nil
	}

	// Resolve path relative to working directory
	path, resolveErr := m.resolvePath(pathParam)
	if resolveErr != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", resolveErr)), nil
	}

	// Security checks
	if err := m.isPathAllowed(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	if err := m.isFileBlacklisted(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	// Perform the read operation
	content, err := os.ReadFile(path)
	if err != nil {
		// Even if read fails, record the attempt for read-write semantics
		// This allows creating new files after attempting to read them
		if os.IsNotExist(err) {
			m.recordFileRead(path)
			return message.NewToolResultError(fmt.Sprintf("file does not exist: %s", path)), nil
		}
		return message.NewToolResultError(fmt.Sprintf("failed to read file: %v", err)), nil
	}

	// Record successful read for read-write semantics
	m.recordFileRead(path)

	return message.NewToolResultText(string(content)), nil
}

func (m *FileSystemToolManager) handleWriteFile(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	logger.DebugWithIcon("📝", "Write file operation started",
		"arg_count", len(args))

	pathParam, ok := args["path"].(string)
	if !ok {
		return message.NewToolResultError("path parameter is required and must be a string"), nil
	}

	// Resolve path relative to working directory
	path, resolveErr := m.resolvePath(pathParam)
	if resolveErr != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", resolveErr)), nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return message.NewToolResultError("content parameter is required and must be a string"), nil
	}

	logger.DebugWithIcon("📝", "Write file proceeding",
		"path", path, "content_length", len(content))

	// Security checks
	if err := m.isPathAllowed(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	// Check if the file exists - only validate read-write semantics for existing files
	if _, err := os.Stat(path); err == nil {
		// File exists - validate read-write semantics
		if err := m.validateReadWriteSemantics(path); err != nil {
			return message.NewToolResultError(err.Error()), nil
		}
	} else if !os.IsNotExist(err) {
		// Other error (permission, etc.) - report it
		return message.NewToolResultError(fmt.Sprintf("failed to check file status: %v", err)), nil
	}
	// If file doesn't exist (os.IsNotExist), allow creating new file without validation

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to create directory: %v", err)), nil
	}

	// Perform the write operation
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to write file: %v", err)), nil
	}

	// Update read timestamp after successful write to allow sequential edits
	m.recordFileRead(path)

	// Run auto-validation based on file type
	validationResult := m.autoValidateFile(ctx, path)

	return message.NewToolResultText(fmt.Sprintf("Successfully wrote to %s%s", path, validationResult)), nil
}

func (m *FileSystemToolManager) handleEnhancedEdit(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	filePath, ok := args["file_path"].(string)
	if !ok {
		return message.NewToolResultError("file_path parameter is required"), nil
	}

	oldString, ok := args["old_string"].(string)
	if !ok {
		return message.NewToolResultError("old_string parameter is required"), nil
	}

	newString, ok := args["new_string"].(string)
	if !ok {
		return message.NewToolResultError("new_string parameter is required"), nil
	}

	// Get replace_all parameter (defaults to false)
	replaceAll := false
	if val, exists := args["replace_all"]; exists {
		if boolVal, ok := val.(bool); ok {
			replaceAll = boolVal
		}
	}

	// Validate that old_string and new_string are different
	if oldString == newString {
		return message.NewToolResultError("old_string and new_string cannot be identical"), nil
	}

	// Resolve path relative to working directory
	absPath, err := m.resolvePath(filePath)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", err)), nil
	}

	// Security checks
	if err := m.isPathAllowed(absPath); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	if err := m.isFileBlacklisted(absPath); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	// Read-write semantics validation
	if err := m.validateReadWriteSemantics(absPath); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	// Read the file
	content, err := os.ReadFile(absPath)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to read file %s: %v", absPath, err)), nil
	}

	fileContent := string(content)

	// Validate exact string matching (Claude Code behavior)
	if !strings.Contains(fileContent, oldString) {
		// Provide helpful debugging information
		return message.NewToolResultError(fmt.Sprintf("old_string not found in file %s. Please ensure exact whitespace and formatting match.", absPath)), nil
	}

	// Count occurrences for safety
	occurrences := strings.Count(fileContent, oldString)
	if occurrences == 0 {
		return message.NewToolResultError(fmt.Sprintf("old_string not found in file %s", absPath)), nil
	}

	// Check if multiple occurrences exist and replace_all is false
	if occurrences > 1 && !replaceAll {
		return message.NewToolResultError(fmt.Sprintf("old_string appears %d times in file %s (use replace_all=true to replace all occurrences)", occurrences, absPath)), nil
	}

	// Perform the replacement
	var newContent string
	if replaceAll {
		// Replace all occurrences
		newContent = strings.ReplaceAll(fileContent, oldString, newString)
	} else {
		// Replace only the first occurrence
		newContent = strings.Replace(fileContent, oldString, newString, 1)
	}

	// Verify that the replacement actually changed the content
	if newContent == fileContent {
		return message.NewToolResultError("no changes made to file - old_string and new_string may be identical"), nil
	}

	// Write the modified content back to the file
	if err := os.WriteFile(absPath, []byte(newContent), 0644); err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to write file %s: %v", absPath, err)), nil
	}

	// Update read timestamp after successful edit to allow sequential edits
	m.recordFileRead(absPath)

	// Calculate change statistics for feedback
	oldLines := strings.Count(oldString, "\n") + 1
	newLines := strings.Count(newString, "\n") + 1

	// Create success message with occurrence information
	var occurrenceInfo string
	if replaceAll {
		occurrenceInfo = fmt.Sprintf("Replaced %d occurrence(s)", occurrences)
	} else {
		occurrenceInfo = "Replaced 1 occurrence"
	}

	// Run auto-validation based on file type
	validationResult := m.autoValidateFile(ctx, absPath)

	return message.NewToolResultText(fmt.Sprintf("Successfully edited %s\n%s\nReplaced %d line(s) with %d line(s)\nOld content: %d characters\nNew content: %d characters%s",
		absPath, occurrenceInfo, oldLines, newLines, len(oldString), len(newString), validationResult)), nil
}

func (m *FileSystemToolManager) handleListDirectory(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	pathParam, ok := args["path"].(string)
	if !ok || pathParam == "" {
		pathParam = "."
	}

	// Resolve path relative to working directory
	path, resolveErr := m.resolvePath(pathParam)
	if resolveErr != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", resolveErr)), nil
	}

	// Security checks
	if err := m.isPathAllowed(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to read directory: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of %s:\n", path))
	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString(fmt.Sprintf("  %s/ (directory)\n", entry.Name()))
		} else {
			result.WriteString(fmt.Sprintf("  %s (file)\n", entry.Name()))
		}
	}

	return message.NewToolResultText(result.String()), nil
}

// fileSystemTool is a helper struct for filesystem tool registration
type fileSystemTool struct {
	name        message.ToolName
	description message.ToolDescription
	arguments   []message.ToolArgument
	handler     func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)
}

func (t *fileSystemTool) RawName() message.ToolName {
	return t.name
}

func (t *fileSystemTool) Name() message.ToolName {
	return t.name
}

func (t *fileSystemTool) Description() message.ToolDescription {
	return t.description
}

func (t *fileSystemTool) Arguments() []message.ToolArgument {
	return t.arguments
}

func (t *fileSystemTool) Handler() func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	return t.handler
}
func (m *FileSystemToolManager) handleFindFile(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	namePattern, ok := args["name_pattern"].(string)
	if !ok {
		return message.NewToolResultError("name_pattern parameter is required"), nil
	}

	// Get path (default to current directory)
	pathParam := "."
	if pathArg, exists := args["path"]; exists {
		if pathStr, ok := pathArg.(string); ok {
			pathParam = pathStr
		}
	}

	// Resolve path relative to working directory
	absPath, resolveErr := m.resolvePath(pathParam)
	if resolveErr != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", resolveErr)), nil
	}

	// Security checks - ensure path is within allowed directories
	if err := m.isPathAllowed(absPath); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	// Get type filter (default to 'f' for files)
	typeFilter := "f"
	if typeArg, exists := args["type"]; exists {
		if typeStr, ok := typeArg.(string); ok && (typeStr == "f" || typeStr == "d" || typeStr == "both") {
			typeFilter = typeStr
		}
	}

	// Build find command
	argsList := []string{absPath}

	// Add type filter
	switch typeFilter {
	case "f":
		argsList = append(argsList, "-type", "f")
	case "d":
		argsList = append(argsList, "-type", "d")
		// For "both", don't add type filter
	}

	// Add name pattern
	argsList = append(argsList, "-name", namePattern)

	// Exclude common directories
	argsList = append(argsList, "-not", "-path", "*/.*", "-not", "-path", "*/node_modules/*", "-not", "-path", "*/vendor/*")

	cmd := exec.CommandContext(ctx, "find", argsList...)

	// Execute command
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("find command failed: %v\nOutput: %s", err, outputStr)), nil
	}

	if outputStr == "" {
		return message.NewToolResultText(fmt.Sprintf("No files found matching pattern '%s' in path '%s'", namePattern, absPath)), nil
	}

	// Clean up the output for better readability
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	if len(lines) > 100 {
		// Limit output to prevent overwhelming the LLM
		truncatedOutput := strings.Join(lines[:100], "\n")
		truncatedOutput += fmt.Sprintf("\n\n... (output truncated, showing first 100 matches out of %d total matches)", len(lines))
		return message.NewToolResultText(truncatedOutput), nil
	}

	return message.NewToolResultText(outputStr), nil
}

// ValidationResult represents the result of a Go validation check
type ValidationResult struct {
	Check   string `json:"check"`
	Status  string `json:"status"` // "pass", "fail", "error"
	Output  string `json:"output,omitempty"`
	Summary string `json:"summary"`
}

// autoValidateFile performs automatic validation after write/edit operations based on file type
func (m *FileSystemToolManager) autoValidateFile(ctx context.Context, filePath string) string {
	// Get file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".go":
		return m.autoValidateGoFile(ctx, filePath)
	default:
		// No validation available for this file type
		return ""
	}
}

// autoValidateGoFile performs automatic Go validation after write/edit operations
func (m *FileSystemToolManager) autoValidateGoFile(ctx context.Context, filePath string) string {
	// Find the directory containing go files
	dir := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)
	
	// Check if this looks like a Go project (has .go files)
	hasGoFiles, err := m.hasGoFilesInDirectory(dir)
	if err != nil || !hasGoFiles {
		return ""
	}

	results := []ValidationResult{}
	
	// Run go vet on the specific file
	vetResult := m.runGoVet(ctx, dir, fileName)
	results = append(results, vetResult)
	
	// Run go build -n (dry run) on the specific file
	buildResult := m.runGoBuild(ctx, dir, fileName)
	results = append(results, buildResult)
	
	// Format validation results
	return m.formatValidationResults(results)
}

// hasGoFilesInDirectory checks if directory contains .go files
func (m *FileSystemToolManager) hasGoFilesInDirectory(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true, nil
		}
	}
	return false, nil
}

// runGoVet executes go vet and returns the result
func (m *FileSystemToolManager) runGoVet(ctx context.Context, dir string, fileName string) ValidationResult {
	result := ValidationResult{
		Check: "go vet - Static analysis to find suspicious constructs",
	}
	
	// Validate the specific file that was written/edited
	cmd := exec.CommandContext(ctx, "go", "vet", fileName)
	cmd.Dir = dir
	
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	
	if err != nil {
		if outputStr != "" {
			result.Status = "fail"
			result.Output = outputStr
			lines := strings.Split(outputStr, "\n")
			result.Summary = fmt.Sprintf("Found %d vet issues", len(lines))
		} else {
			result.Status = "error"
			result.Output = err.Error()
			result.Summary = fmt.Sprintf("Could not run go vet: %v", err)
		}
	} else {
		result.Status = "pass"
		result.Summary = "No vet issues found"
	}
	
	return result
}

// runGoBuild executes go build -n (dry run) and returns the result
func (m *FileSystemToolManager) runGoBuild(ctx context.Context, dir string, fileName string) ValidationResult {
	result := ValidationResult{
		Check: "go build -n - Check if code compiles without building",
	}
	
	// Validate the specific file that was written/edited
	cmd := exec.CommandContext(ctx, "go", "build", "-n", fileName)
	cmd.Dir = dir
	
	output, err := cmd.CombinedOutput()
	outputStr := strings.TrimSpace(string(output))
	
	if err != nil {
		result.Status = "fail"
		result.Output = outputStr
		result.Summary = "Build would fail - compilation errors found"
	} else {
		result.Status = "pass"
		result.Summary = "Code compiles successfully"
	}
	
	return result
}

// formatValidationResults formats validation results into a readable string
func (m *FileSystemToolManager) formatValidationResults(results []ValidationResult) string {
	if len(results) == 0 {
		return ""
	}
	
	var output strings.Builder
	output.WriteString("\n\n🔍 **Go Validation Results:**\n")
	
	passed := 0
	failed := 0
	
	for _, result := range results {
		switch result.Status {
		case "pass":
			output.WriteString(fmt.Sprintf("✅ %s: %s\n", result.Check, result.Summary))
			passed++
		case "fail":
			output.WriteString(fmt.Sprintf("❌ %s: %s\n", result.Check, result.Summary))
			if result.Output != "" {
				// Limit output to prevent overwhelming response
				lines := strings.Split(result.Output, "\n")
				if len(lines) > 5 {
					output.WriteString(fmt.Sprintf("```\n%s\n... (%d more lines)\n```\n", 
						strings.Join(lines[:5], "\n"), len(lines)-5))
				} else {
					output.WriteString(fmt.Sprintf("```\n%s\n```\n", result.Output))
				}
			}
			failed++
		case "error":
			output.WriteString(fmt.Sprintf("⚠️  %s: %s\n", result.Check, result.Summary))
		}
	}
	
	if failed == 0 {
		output.WriteString(fmt.Sprintf("\n🎉 All %d validation checks passed!\n", passed))
	} else {
		output.WriteString(fmt.Sprintf("\n📊 Validation Summary: %d passed, %d failed\n", passed, failed))
	}
	
	return output.String()
}
