package classify

import (
	"path/filepath"
	"strings"
)

const (
	CategoryDocuments = "Documents"
	CategoryPictures  = "Pictures"
	CategoryVideos    = "Videos"
	CategoryMusic     = "Music"
	CategoryOther     = "Other"
)

var musicFolderTokens = []string{
	"music/",
	"shared music/",
	"audio/",
	"podcasts/",
	"soundtracks/",
	"songs/",
	"albums/",
	"playlists/",
}

func isInMusicFolder(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	for _, token := range musicFolderTokens {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
}

func CategoryFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	// Check for embedded pictures in music folders
	if isInMusicFolder(path) {
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tif", ".tiff", ".heic", ".raw", ".svg":
			return CategoryOther
		}
	}

	switch ext {
	case ".txt", ".md", ".rtf", ".doc", ".docx", ".pdf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx", ".csv", ".json", ".xml":
		return CategoryDocuments
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tif", ".tiff", ".heic", ".raw", ".svg":
		return CategoryPictures
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".m4v", ".flv", ".webm":
		return CategoryVideos
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a":
		return CategoryMusic
	default:
		return CategoryOther
	}
}
