package common

import (
	"os"
	"path/filepath"
	"strings"
)

// These variables are set via ldflags during build
var (
	// Version is the semantic version from .version file
	Version = "dev"
	// Build is the build timestamp from .version file
	Build = "unknown"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetBuild returns the build timestamp
func GetBuild() string {
	return Build
}

// GetGitCommit returns the git commit hash
func GetGitCommit() string {
	return GitCommit
}

// GetFullVersion returns the complete version information
func GetFullVersion() string {
	if Build != "unknown" {
		return Version + "-" + Build
	}
	return Version
}

// GetExtensionVersion reads the extension version from .version file
func GetExtensionVersion() string {
	// Try multiple possible locations for .version file
	possiblePaths := []string{
		".version",
		filepath.Join("..", ".version"),
		filepath.Join("..", "..", ".version"),
	}

	for _, versionPath := range possiblePaths {
		data, err := os.ReadFile(versionPath)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "extension_version:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}

	return "unknown"
}
