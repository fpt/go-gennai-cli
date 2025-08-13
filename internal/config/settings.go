package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fpt/go-gennai-cli/pkg/agent/domain"
	pkgLogger "github.com/fpt/go-gennai-cli/pkg/logger"
)

// Default maximum iterations for agents
const DefaultAgentMaxIterations = 30

// Settings represents the main application settings
type Settings struct {
	LLM   LLMSettings   `json:"llm"`
	MCP   MCPSettings   `json:"mcp"`
	Agent AgentSettings `json:"agent"`
}

// LLMSettings contains LLM client configuration
type LLMSettings struct {
	Backend   string `json:"backend"`              // "ollama", "anthropic", "openai", or "gemini"
	Model     string `json:"model"`                // model name
	BaseURL   string `json:"base_url,omitempty"`   // for ollama or openai (Azure)
	Thinking  bool   `json:"thinking,omitempty"`   // enable thinking mode
	MaxTokens int    `json:"max_tokens,omitempty"` // maximum tokens for model responses (0 = use model default)
}

// MCPSettings contains MCP server configuration
type MCPSettings struct {
	Servers []domain.MCPServerConfig `json:"servers,omitempty"`
}

// AgentSettings contains agent behavior configuration
type AgentSettings struct {
	MaxIterations int    `json:"max_iterations"`
	LogLevel      string `json:"log_level"`
}

// LoadSettings loads application settings from a JSON file
func LoadSettings(configPath string) (*Settings, error) {
	// If config path is empty, search in order of preference
	if configPath == "" {
		configPath = findSettingsFile()
		if configPath == "" {
			// No settings file found, create default one and return defaults
			return createDefaultSettingsFile()
		}
	}

	// Check if specified file exists, create defaults if not
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// If a specific path was provided but doesn't exist, create it
		if configPath != "" {
			settings, _ := createSettingsFileAtPath(configPath)
			return settings, nil
		}
		return GetDefaultSettings(), nil
	}

	// Read and parse the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("failed to parse settings: %w", err)
	}

	// Apply defaults for missing fields
	applyDefaults(&settings)

	return &settings, nil
}

// SaveSettings saves application settings to a JSON file
func SaveSettings(configPath string, settings *Settings) error {
	// If config path is empty, determine where to save
	if configPath == "" {
		// Try to find existing settings file first
		configPath = findSettingsFile()
		if configPath == "" {
			// No existing file, save to .gennai in current directory
			configPath = filepath.Join(".gennai", "settings.json")
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with pretty formatting
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// GetDefaultSettings returns default application settings
func GetDefaultSettings() *Settings {
	return &Settings{
		LLM: LLMSettings{
			Backend:   "ollama",
			Model:     "gpt-oss:latest",
			BaseURL:   "http://localhost:11434",
			Thinking:  true,
			MaxTokens: 0, // 0 = use model-specific defaults
		},
		Agent: AgentSettings{
			MaxIterations: DefaultAgentMaxIterations,
			LogLevel:      "info",
		},
	}
}

// applyDefaults fills in missing fields with default values
func applyDefaults(settings *Settings) {
	defaults := GetDefaultSettings()

	// Apply LLM defaults
	if settings.LLM.Backend == "" {
		settings.LLM.Backend = defaults.LLM.Backend
	}
	if settings.LLM.Model == "" {
		settings.LLM.Model = defaults.LLM.Model
	}
	if settings.LLM.BaseURL == "" && settings.LLM.Backend == "ollama" {
		settings.LLM.BaseURL = defaults.LLM.BaseURL
	}

	// Apply MCP defaults (no config_path needed anymore)

	// Apply Agent defaults
	if settings.Agent.MaxIterations == 0 {
		settings.Agent.MaxIterations = defaults.Agent.MaxIterations
	}
	if settings.Agent.LogLevel == "" {
		settings.Agent.LogLevel = defaults.Agent.LogLevel
	}
}

// ValidateSettings validates the settings configuration
func ValidateSettings(settings *Settings) error {
	// Validate LLM settings
	if settings.LLM.Backend != "ollama" && settings.LLM.Backend != "anthropic" && settings.LLM.Backend != "openai" && settings.LLM.Backend != "gemini" {
		return fmt.Errorf("unsupported LLM backend: %s (must be 'ollama', 'anthropic', 'openai', or 'gemini')", settings.LLM.Backend)
	}

	if settings.LLM.Model == "" {
		return fmt.Errorf("LLM model is required")
	}

	if settings.LLM.Backend == "anthropic" {
		// Check environment variable for API key
		if os.Getenv("ANTHROPIC_API_KEY") == "" {
			return fmt.Errorf("Anthropic API key is required (set ANTHROPIC_API_KEY environment variable)")
		}
	}

	if settings.LLM.Backend == "openai" {
		// Check environment variable for API key
		if os.Getenv("OPENAI_API_KEY") == "" {
			return fmt.Errorf("OpenAI API key is required (set OPENAI_API_KEY environment variable)")
		}
	}

	if settings.LLM.Backend == "gemini" {
		// Check environment variable for API key
		if os.Getenv("GEMINI_API_KEY") == "" {
			return fmt.Errorf("Gemini API key is required (set GEMINI_API_KEY environment variable)")
		}
	}

	// Validate Agent settings
	if settings.Agent.MaxIterations <= 0 {
		return fmt.Errorf("max_iterations must be positive")
	}

	// Validate MCP server configurations
	for _, serverConfig := range settings.MCP.Servers {
		if err := ValidateMCPServerConfig(serverConfig); err != nil {
			return fmt.Errorf("invalid MCP server configuration for %s: %w", serverConfig.Name, err)
		}
	}

	return nil
}

// findSettingsFile searches for settings.json in order of preference:
// 1. .gennai/settings.json in current directory
// 2. $HOME/.gennai/settings.json
// Returns empty string if none found
func findSettingsFile() string {
	// Check .gennai in current directory
	currentDirPath := filepath.Join(".gennai", "settings.json")
	if _, err := os.Stat(currentDirPath); err == nil {
		return currentDirPath
	}

	// Check $HOME/.gennai
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeDirPath := filepath.Join(homeDir, ".gennai", "settings.json")
		if _, err := os.Stat(homeDirPath); err == nil {
			return homeDirPath
		}
	}

	// No settings file found
	return ""
}

// ValidateMCPServerConfig validates an MCP server configuration
func ValidateMCPServerConfig(config domain.MCPServerConfig) error {
	if config.Name == "" {
		return fmt.Errorf("server name is required")
	}

	switch config.Type {
	case domain.MCPServerTypeStdio:
		if config.Command == "" {
			return fmt.Errorf("command is required for stdio servers")
		}
	case domain.MCPServerTypeSSE:
		if config.URL == "" {
			return fmt.Errorf("URL is required for HTTP/SSE servers")
		}
	default:
		return fmt.Errorf("unsupported server type: %s", config.Type)
	}

	return nil
}

// createDefaultSettingsFile creates a default settings.json file in ~/.gennai/
func createDefaultSettingsFile() (*Settings, error) {
	// Determine where to create the file (prefer home directory)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return GetDefaultSettings(), nil // Fall back to defaults without file creation
	}

	settingsPath := filepath.Join(homeDir, ".gennai", "settings.json")
	return createSettingsFileAtPath(settingsPath)
}

// createSettingsFileAtPath creates a default settings file at the specified path
func createSettingsFileAtPath(settingsPath string) (*Settings, error) {
	// Get default settings
	settings := GetDefaultSettings()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return settings, nil // Return defaults if directory creation fails
	}

	// Marshal to JSON with pretty formatting
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return settings, nil // Return defaults if marshaling fails
	}

	// Write to file
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return settings, nil // Return defaults if file writing fails
	}

	// Log success message
	pkgLogger.NewComponentLogger("settings").InfoWithIntention(pkgLogger.IntentionConfig, "Created default settings file", "path", settingsPath)
	pkgLogger.NewComponentLogger("settings").InfoWithIntention(pkgLogger.IntentionStatus, "You can edit this file to customize your configuration")

	return settings, nil
}
