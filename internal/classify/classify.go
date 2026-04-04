package classify

import (
	"path/filepath"
	"strings"
)

const (
	CategoryDocuments = "Documents"
	CategoryPhotos    = "Photos"
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
	normalized := strings.ToLower(filepath.ToSlash(path))
	ext := strings.ToLower(filepath.Ext(path))

	// WISO tax project and backup files often use custom suffixes like
	// .steuer2024 or .eur2023 (including autosave variants). Keep these in
	// Documents so tax data is easy to find with regular statement PDFs.
	if strings.Contains(normalized, ".steuer") || strings.Contains(normalized, ".eur") {
		return CategoryDocuments
	}

	// Check for embedded pictures in music folders
	if isInMusicFolder(path) {
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".tif", ".tiff", ".heic", ".raw", ".svg":
			return CategoryOther
		}
	}

	switch ext {
	case ".txt", ".md", ".rtf", ".doc", ".docx", ".pdf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx", ".csv":
		return CategoryDocuments
	case ".jpg", ".jpeg":
		return CategoryPhotos
	case ".png", ".gif", ".bmp", ".webp", ".tif", ".tiff", ".heic", ".raw", ".svg":
		return CategoryPictures
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".m4v", ".flv", ".webm":
		return CategoryVideos
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a":
		return CategoryMusic
	default:
		return CategoryOther
	}
}
