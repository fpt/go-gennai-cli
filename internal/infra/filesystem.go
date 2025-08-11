package infra

import (
	"path/filepath"

	"github.com/fpt/go-gennai-cli/internal/repository"
)

// DefaultFileSystemConfig returns a more permissive configuration for development
func DefaultFileSystemConfig(workingDir string) repository.FileSystemConfig {
	absWorkingDir, err := filepath.Abs(workingDir)
	if err != nil {
		absWorkingDir = workingDir
	}

	return repository.FileSystemConfig{
		// Simple and secure: only allow operations under the working directory
		// This automatically handles any working directory (project root, test dir, etc.)
		AllowedDirectories: []string{
			absWorkingDir,
		},
		// More comprehensive blacklist for development
		BlacklistedFiles: []string{
			".env",
			".env.*",
			"*.key",
			"*.pem",
			"*.crt",
			"*secret*",
			"*password*",
			"*token*",
			"*api_key*",
			".aws/credentials",
			".aws/config",
			".ssh/id_*",
			".ssh/known_hosts",
			"*.p12",
			"*.pfx",
			"config.json",
			"secrets.json",
			"credentials.json",
			".netrc",
			".dockercfg",
			".docker/config.json",
			".npmrc",
			".yarnrc",
			".gitconfig",
			"*.sqlite",
			"*.db",
			"node_modules/*",
			".git/*",
			"vendor/*",
			"*.log",
		},
	}
}
