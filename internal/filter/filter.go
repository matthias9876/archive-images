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
	"/program files",
	"/applications/",
	"/setup/",
	"/drivers/",
	"/steamapps/",
}

func IsLikelyProgram(path string) bool {
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
