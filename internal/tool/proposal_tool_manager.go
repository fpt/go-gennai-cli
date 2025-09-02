package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fpt/go-gennai-cli/internal/repository"
	"github.com/fpt/go-gennai-cli/pkg/message"
	"github.com/pkg/errors"

	diff "github.com/hexops/gotextdiff"
	myers "github.com/hexops/gotextdiff/myers"
)

// ProposalToolManager provides editor-safe proposal tools (no direct writes).
// It exposes a single powerful tool: ProposeChanges, which returns diffs and new contents
// without modifying the filesystem. Intended for GUI/editor integration.
type ProposalToolManager struct {
	allowedDirectories []string
	blacklistedFiles   []string
	workingDir         string

	tools map[message.ToolName]message.Tool
}

// NewProposalToolManager creates a new proposal-only tool manager
func NewProposalToolManager(config repository.FileSystemConfig, workingDir string) *ProposalToolManager {
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		absWorkingDir = workingDir
	}

	allowedDirs := ensureWorkingDirectoryInAllowedList(config.AllowedDirectories, absWorkingDir)

	m := &ProposalToolManager{
		allowedDirectories: allowedDirs,
		blacklistedFiles:   config.BlacklistedFiles,
		workingDir:         absWorkingDir,
		tools:              make(map[message.ToolName]message.Tool),
	}

	m.registerProposalTools()
	return m
}

// Implement domain.ToolManager interface
func (m *ProposalToolManager) RegisterTool(name message.ToolName, description message.ToolDescription, arguments []message.ToolArgument, handler func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)) {
	m.tools[name] = &proposalTool{
		name:        name,
		description: description,
		arguments:   arguments,
		handler:     handler,
	}
}

func (m *ProposalToolManager) GetTools() map[message.ToolName]message.Tool { return m.tools }

func (m *ProposalToolManager) GetTool(name message.ToolName) (message.Tool, bool) {
	t, ok := m.tools[name]
	return t, ok
}

func (m *ProposalToolManager) CallTool(ctx context.Context, name message.ToolName, args message.ToolArgumentValues) (message.ToolResult, error) {
	t, ok := m.tools[name]
	if !ok {
		return message.NewToolResultError(fmt.Sprintf("tool %s not found", name)), nil
	}
	return t.Handler()(ctx, args)
}

// Tool registration
func (m *ProposalToolManager) registerProposalTools() {
	// ProposeChanges: declare a batch of file operations and receive diffs/new contents.
	m.RegisterTool("ProposeChanges", "Propose a set of file changes (create/update/delete) and receive unified diffs plus new contents. Does not write files.",
		[]message.ToolArgument{
			{Name: "changes", Description: "Array of changes: {path, operation:create|update|delete, new_content?}", Required: true, Type: "array"},
		}, m.handleProposeChanges)

	// Read-only helpers
	m.RegisterTool("Read", "Read a file with optional offset/limit and line-numbered output",
		[]message.ToolArgument{
			{Name: "file_path", Description: "Path to the file to read", Required: true, Type: "string"},
			{Name: "offset", Description: "1-based line start (optional)", Required: false, Type: "number"},
			{Name: "limit", Description: "Number of lines to return (optional)", Required: false, Type: "number"},
		}, m.handleRead)

	m.RegisterTool("LS", "List directory contents with optional ignore globs",
		[]message.ToolArgument{
			{Name: "path", Description: "Directory path to list", Required: true, Type: "string"},
			{Name: "ignore", Description: "Array of glob patterns to ignore", Required: false, Type: "array"},
		}, m.handleLS)
}

// Internal helpers (path security)
var errNotInAllowedDirectoryProposal = errors.New("file access denied: path is not within allowed directories")

func (m *ProposalToolManager) abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	resolved := filepath.Join(m.workingDir, path)
	return filepath.Clean(resolved), nil
}

func (m *ProposalToolManager) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		absWorkingDir, err := m.abs(m.workingDir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve working directory: %v", err)
		}
		if strings.HasPrefix(path, absWorkingDir+string(os.PathSeparator)) || path == absWorkingDir {
			return path, nil
		}
		return "", fmt.Errorf("absolute path %s is outside working directory %s", path, m.workingDir)
	}
	return m.abs(path)
}

func (m *ProposalToolManager) isPathAllowed(path string) error {
	absPath := path
	for _, allowedDir := range m.allowedDirectories {
		allowedAbs, err := m.abs(allowedDir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, allowedAbs+string(os.PathSeparator)) || absPath == allowedAbs {
			return nil
		}
	}
	return errNotInAllowedDirectoryProposal
}

func (m *ProposalToolManager) isFileBlacklisted(path string) error {
	fileName := filepath.Base(path)
	absPath := path
	for _, blacklisted := range m.blacklistedFiles {
		if matched, _ := filepath.Match(blacklisted, fileName); matched {
			return fmt.Errorf("file access denied: %s matches blacklisted pattern %s", fileName, blacklisted)
		}
		if matched, _ := filepath.Match(blacklisted, absPath); matched {
			return fmt.Errorf("file access denied: %s matches blacklisted pattern %s", absPath, blacklisted)
		}
		if fileName == blacklisted || absPath == blacklisted {
			return fmt.Errorf("file access denied: %s is blacklisted", path)
		}
	}
	return nil
}

// Input/Output structures for JSON result payload
type proposedChangeInput struct {
	Path       string `json:"path"`
	Operation  string `json:"operation"`             // create|update|delete
	NewContent string `json:"new_content,omitempty"` // for create/update
}

type filePatch struct {
	Path        string `json:"path"`
	Operation   string `json:"operation"`
	UnifiedDiff string `json:"unified_diff"`
	NewContent  string `json:"new_content,omitempty"`
	Conflict    bool   `json:"conflict"`
	Warning     string `json:"warning,omitempty"`
}

type proposedChangesResult struct {
	Changes      []filePatch `json:"changes"`
	HasConflicts bool        `json:"has_conflicts"`
	Warnings     []string    `json:"warnings,omitempty"`
}

// Handler
func (m *ProposalToolManager) handleProposeChanges(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	raw, ok := args["changes"]
	if !ok {
		return message.NewToolResultError("changes parameter is required"), nil
	}

	// Accept []interface{} (native tool calling) or JSON string
	var inputs []proposedChangeInput
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			mapp, ok := item.(map[string]interface{})
			if !ok {
				return message.NewToolResultError("each change must be an object"), nil
			}
			inp := proposedChangeInput{}
			if s, ok := mapp["path"].(string); ok {
				inp.Path = s
			}
			if s, ok := mapp["operation"].(string); ok {
				inp.Operation = strings.ToLower(s)
			}
			if s, ok := mapp["new_content"].(string); ok {
				inp.NewContent = s
			}
			if inp.Path == "" || inp.Operation == "" {
				return message.NewToolResultError("each change requires path and operation"), nil
			}
			inputs = append(inputs, inp)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &inputs); err != nil {
			return message.NewToolResultError("invalid JSON for changes"), nil
		}
	default:
		return message.NewToolResultError("unsupported 'changes' parameter format"), nil
	}

	var result proposedChangesResult
	var patches []filePatch
	var warnings []string
	hasConflicts := false

	for _, ch := range inputs {
		// Security: resolve and validate path
		absPath, err := m.resolvePath(ch.Path)
		if err != nil {
			patches = append(patches, filePatch{Path: ch.Path, Operation: ch.Operation, Conflict: true, Warning: fmt.Sprintf("path resolution failed: %v", err)})
			hasConflicts = true
			continue
		}
		if err := m.isPathAllowed(absPath); err != nil {
			patches = append(patches, filePatch{Path: ch.Path, Operation: ch.Operation, Conflict: true, Warning: err.Error()})
			hasConflicts = true
			continue
		}
		if err := m.isFileBlacklisted(absPath); err != nil {
			patches = append(patches, filePatch{Path: ch.Path, Operation: ch.Operation, Conflict: true, Warning: err.Error()})
			hasConflicts = true
			continue
		}

		// Determine old/new contents based on operation
		var oldContent string
		var newContent string
		switch ch.Operation {
		case "create":
			newContent = ch.NewContent
			// oldContent empty; if file exists, flag as conflict
			if _, err := os.Stat(absPath); err == nil {
				warnings = append(warnings, fmt.Sprintf("create requested but file already exists: %s", ch.Path))
			}
		case "update":
			// Read old content; if missing, mark warning/conflict?? We'll warn, not block.
			if b, err := os.ReadFile(absPath); err == nil {
				oldContent = string(b)
			} else {
				warnings = append(warnings, fmt.Sprintf("update requested but file does not exist: %s", ch.Path))
			}
			newContent = ch.NewContent
		case "delete":
			// Read old content for diff; if missing warn
			if b, err := os.ReadFile(absPath); err == nil {
				oldContent = string(b)
			} else {
				warnings = append(warnings, fmt.Sprintf("delete requested but file does not exist: %s", ch.Path))
			}
			newContent = ""
		default:
			patches = append(patches, filePatch{Path: ch.Path, Operation: ch.Operation, Conflict: true, Warning: "unsupported operation (use create|update|delete)"})
			hasConflicts = true
			continue
		}

		// Compute unified diff
		edits := myers.ComputeEdits("", oldContent, newContent)
        unified := diff.ToUnified("a/"+ch.Path, "b/"+ch.Path, oldContent, edits)
        unifiedStr := fmt.Sprint(unified)

		patches = append(patches, filePatch{
			Path:        ch.Path,
			Operation:   ch.Operation,
			UnifiedDiff: unifiedStr,
			NewContent:  newContent,
			Conflict:    false,
		})
	}

	result.Changes = patches
	result.HasConflicts = hasConflicts
	if len(warnings) > 0 {
		result.Warnings = warnings
	}

	buf, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to encode result: %v", err)), nil
	}

	// Return as JSON text for consumers (CLI/extension) to parse
	return message.NewToolResultText(string(buf)), nil
}

// proposalTool implements message.Tool
type proposalTool struct {
	name        message.ToolName
	description message.ToolDescription
	arguments   []message.ToolArgument
	handler     func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error)
}

func (t *proposalTool) RawName() message.ToolName            { return t.name }
func (t *proposalTool) Name() message.ToolName               { return t.name }
func (t *proposalTool) Description() message.ToolDescription { return t.description }
func (t *proposalTool) Arguments() []message.ToolArgument    { return t.arguments }
func (t *proposalTool) Handler() func(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	return t.handler
}

// Read-only helpers
func (m *ProposalToolManager) handleRead(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	pathParam, ok := args["file_path"].(string)
	if !ok {
		return message.NewToolResultError("file_path parameter is required"), nil
	}

	path, resolveErr := m.resolvePath(pathParam)
	if resolveErr != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", resolveErr)), nil
	}
	if err := m.isPathAllowed(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}
	if err := m.isFileBlacklisted(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return message.NewToolResultError(fmt.Sprintf("file does not exist: %s", path)), nil
		}
		return message.NewToolResultError(fmt.Sprintf("failed to read file: %v", err)), nil
	}

	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	// Determine paging
	start := 0
	if off, ok := args["offset"]; ok {
		switch v := off.(type) {
		case float64:
			if v > 1 {
				start = int(v) - 1
			}
		case int:
			if v > 1 {
				start = v - 1
			}
		}
	}
	end := len(lines)
	if lim, ok := args["limit"]; ok {
		switch v := lim.(type) {
		case float64:
			if v > 0 && start+int(v) < end {
				end = start + int(v)
			}
		case int:
			if v > 0 && start+v < end {
				end = start + v
			}
		}
	}
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	if end < start {
		end = start
	}

	var b strings.Builder
	ln := start + 1
	for i := start; i < end; i++ {
		b.WriteString(fmt.Sprintf("%6d\t%s\n", ln, lines[i]))
		ln++
	}
	return message.NewToolResultText(b.String()), nil
}

func (m *ProposalToolManager) handleLS(ctx context.Context, args message.ToolArgumentValues) (message.ToolResult, error) {
	pathParam, ok := args["path"].(string)
	if !ok {
		return message.NewToolResultError("path parameter is required"), nil
	}

	path, resolveErr := m.resolvePath(pathParam)
	if resolveErr != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", resolveErr)), nil
	}
	if err := m.isPathAllowed(path); err != nil {
		return message.NewToolResultError(err.Error()), nil
	}

	// Parse ignore patterns
	var ignores []string
	if raw, ok := args["ignore"]; ok {
		if list, ok := raw.([]interface{}); ok {
			for _, v := range list {
				if s, ok := v.(string); ok {
					ignores = append(ignores, s)
				}
			}
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return message.NewToolResultError(fmt.Sprintf("failed to read directory: %v", err)), nil
	}

	matchesIgnore := func(name string) bool {
		for _, pat := range ignores {
			if ok, _ := filepath.Match(pat, name); ok {
				return true
			}
		}
		return false
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Contents of %s:\n", path))
	for _, e := range entries {
		name := e.Name()
		if matchesIgnore(name) {
			continue
		}
		if e.IsDir() {
			b.WriteString(fmt.Sprintf("  %s/ (directory)\n", name))
		} else {
			b.WriteString(fmt.Sprintf("  %s (file)\n", name))
		}
	}
	return message.NewToolResultText(b.String()), nil
}
