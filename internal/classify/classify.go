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

func CategoryFor(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

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
