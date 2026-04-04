package filter

import (
	"path/filepath"
	"strings"
)

var blockedExt = map[string]struct{}{
	".exe":      {},
	".msi":      {},
	".dmg":      {},
	".apk":      {},
	".deb":      {},
	".rpm":      {},
	".pkg":      {},
	".iso":      {},
	".appimage": {},
	".bat":      {},
	".cmd":      {},
	".com":      {},
	".ps1":      {},
	".scr":      {},
}

var blockedPathTokens = []string{
	"/downloads/",
	"/installer",
	"/installers",
	"/applications/",
	"/setup/",
	"/drivers/",
	"/steamapps/",
}

// blockedDirectoryTokens is the list of folder name segments (lower-case, with
// surrounding slashes) that should be skipped entirely when scanning a Windows
// source. Tokens are matched case-insensitively against the full path, so the
// slash separators prevent a token like "/windows/" from matching a folder
// called "not-windows-folder".
var blockedDirectoryTokens = []string{
	// Windows system & OS folders
	"/windows/",
	"/$windows.~ws/", // in-place upgrade staging folder
	"/$recycle.bin/",
	"/recycler/",
	"/config.msi/",   // Windows Installer rollback
	"/perflogs/",
	"/esd/",          // encrypted system drive images
	"/inetpub/",      // IIS web root
	"/xboxgames/",
	// Program / data directories
	"/program files/",
	"/program files (x86)/",
	"/programdata/",
	// User-profile noise
	"/appdata/",
	"/onedrivetemp/",
	// Generic cache folders (also catches .cache on Linux mounts)
	"/cache/",
	"/caches/",
	"/.cache/",
	// Driver / chipset installer folders at drive root
	"/amd/",
	// Development toolchain / IDE data (large binaries, not personal files)
	"/.espressif/",
	"/.vscode/",
	// AI model stores (can be dozens of gigabytes, never personal files)
	"/.lmstudio/",
	"/.ollama/",
	// Virtual machine images and config
	"/virtualbox vms/",
	"/.virtualbox/",
	// Thumbnail cache
	"/.thumbnails/",
	// Windows user-profile shell folders with no personal-file content
	"/saved games/",
	"/searches/",
	"/contacts/",
	"/links/",
	"/favorites/",
	// Telemetry and account metadata
	"/.oracle_jre_usage/",
	"/.ms-ad/",
}

func isImportantWisoPath(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(normalized, "wiso steuer") ||
		strings.Contains(normalized, "wiso-steuer") ||
		strings.Contains(normalized, "wisosteuer")
}

func normalizedContainsDirToken(normalizedPath, token string) bool {
	// Add a trailing slash so folder tokens also match when the current path is
	// exactly the directory itself (for example "/x/Program Files").
	return strings.Contains(strings.TrimSuffix(normalizedPath, "/")+"/", token)
}

func ShouldSkipDirectory(path string) bool {
	if isImportantWisoPath(path) {
		return false
	}

	normalized := strings.ToLower(filepath.ToSlash(path))
	for _, token := range blockedDirectoryTokens {
		if normalizedContainsDirToken(normalized, token) {
			return true
		}
	}

	return false
}

func IsLikelyProgram(path string) bool {
	if isImportantWisoPath(path) {
		return false
	}

	ext := strings.ToLower(filepath.Ext(path))
	if _, found := blockedExt[ext]; found {
		return true
	}

	normalized := strings.ToLower(filepath.ToSlash(path))
	for _, token := range blockedPathTokens {
		if strings.Contains(normalized, token) {
			return true
		}
	}

	return false
}
