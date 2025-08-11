package repository

// FileSystemConfig holds configuration for the filesystem tool manager
type FileSystemConfig struct {
	AllowedDirectories []string `json:"allowed_directories"` // Paths where file operations are allowed
	BlacklistedFiles   []string `json:"blacklisted_files"`   // Files that cannot be read
}
