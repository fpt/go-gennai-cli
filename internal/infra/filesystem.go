package infra

import (
	"os"
	"path/filepath"

	"github.com/fpt/go-gennai-cli/internal/repository"
)

// DefaultFileSystemConfig returns a secure default configuration for filesystem tools
func DefaultFileSystemConfig() repository.FileSystemConfig {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	return repository.FileSystemConfig{
		// By default, only allow operations in current directory and subdirectories
		AllowedDirectories: []string{
			cwd,
		},
		// Common secret files to blacklist
		BlacklistedFiles: []string{
			".env",
			".env.*",
			"*.key",
			"*.pem",
			"*.crt",
			"*secret*",
			"*password*",
			"*token*",
			".aws/credentials",
			".ssh/id_*",
			"*.p12",
			"*.pfx",
			"config.json",
			"secrets.json",
			".netrc",
			".dockercfg",
			".npmrc",
		},
	}
}

// DevelopmentFileSystemConfig returns a more permissive configuration for development
func DevelopmentFileSystemConfig(workingDir string) repository.FileSystemConfig {
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

// ProductionFileSystemConfig returns a highly restrictive configuration for production
func ProductionFileSystemConfig(workDir string) repository.FileSystemConfig {
	absWorkDir, err := filepath.Abs(workDir)
	if err != nil {
		absWorkDir = workDir
	}

	return repository.FileSystemConfig{
		// Only allow operations in a specific work directory
		AllowedDirectories: []string{
			absWorkDir,
		},
		// Very comprehensive blacklist for production
		BlacklistedFiles: []string{
			// Environment and config files
			".env*",
			"*config*",
			"*secret*",
			"*password*",
			"*token*",
			"*key*",
			"*credential*",

			// Certificates and crypto
			"*.pem",
			"*.crt",
			"*.key",
			"*.p12",
			"*.pfx",
			"*.cer",

			// Cloud provider configs
			".aws/*",
			".azure/*",
			".gcp/*",

			// Version control
			".git/*",
			".svn/*",
			".hg/*",

			// SSH and networking
			".ssh/*",
			".netrc",

			// Development tools
			".docker/*",
			".npmrc",
			".yarnrc",
			".pip/*",

			// Databases
			"*.db",
			"*.sqlite*",
			"*.sql",

			// Logs and temporary files
			"*.log",
			"tmp/*",
			"temp/*",

			// System files
			"/etc/*",
			"/var/*",
			"/proc/*",
			"/sys/*",
		},
	}
}
