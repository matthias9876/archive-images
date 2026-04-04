package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dryRun := flag.Bool("dry-run", true, "If true, only log files to be deleted; if false, actually delete them")
	workDir := flag.String("work-dir", "", "Directory to scan for files to delete (required)")
	logFile := flag.String("log", "deletion_list.txt", "Path to log file where files to be deleted are written")
	flag.Parse()

	if *workDir == "" {
		fmt.Fprintln(os.Stderr, "Error: -work-dir is required")
		flag.Usage()
		os.Exit(1)
	}

	log, err := os.Create(*logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	deleteCount := 0
	keepCount := 0

	err = filepath.WalkDir(*workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot access %s: %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if shouldDelete(path) {
			deleteCount++
			fmt.Fprintln(log, path)
		} else {
			keepCount++
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walking directory: %v\n", err)
		os.Exit(1)
	}

	emptyDirCount := countEmptyDirs(*workDir)

	if *dryRun {
		fmt.Fprintf(os.Stderr, "\nDry-run mode:\n")
		fmt.Fprintf(os.Stderr, "  Files to delete : %d\n", deleteCount)
		fmt.Fprintf(os.Stderr, "  Files to keep   : %d\n", keepCount)
		fmt.Fprintf(os.Stderr, "  Empty folders   : %d\n", emptyDirCount)
		fmt.Fprintf(os.Stderr, "  Deletion list   : %s\n", *logFile)
	} else {
		// Show stats and ask for confirmation before deleting.
		fmt.Printf("\nFiles to be deleted : %d\n", deleteCount)
		fmt.Printf("Files to be kept    : %d\n", keepCount)
		fmt.Printf("Empty folders       : %d\n", emptyDirCount)
		fmt.Printf("Deletion list       : %s\n\n", *logFile)
		fmt.Print("Type 'yes' to proceed with deletion: ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if answer != "yes" {
			fmt.Println("Aborted. No files were deleted.")
			os.Exit(0)
		}

		// Re-read the log and delete each file.
		data, readErr := os.ReadFile(*logFile)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "Error reading log file for deletion: %v\n", readErr)
			os.Exit(1)
		}
		actualDeleted := 0
		for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			if line == "" {
				continue
			}
			if removeErr := os.Remove(line); removeErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to delete %s: %v\n", line, removeErr)
			} else {
				actualDeleted++
			}
		}

		// Remove empty directories bottom-up (deepest first).
		removedDirs := removeEmptyDirs(*workDir)

		fmt.Printf("\nDeleted %d files, kept %d files\n", actualDeleted, keepCount)
		fmt.Printf("Removed %d empty folders\n", removedDirs)
		fmt.Printf("Deletion list written to: %s\n", *logFile)
	}
}

// countEmptyDirs counts directories under root that contain no files (recursively).
func countEmptyDirs(root string) int {
	count := 0
	// Walk bottom-up by collecting all dirs, then check each.
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	for _, dir := range dirs {
		if isDirEmpty(dir) {
			count++
		}
	}
	return count
}

// removeEmptyDirs deletes empty directories under root, deepest first, and returns the count removed.
func removeEmptyDirs(root string) int {
	removed := 0
	// Repeat passes until no more empty dirs can be removed (handles newly-emptied parents).
	for {
		var dirs []string
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && path != root {
				dirs = append(dirs, path)
			}
			return nil
		})
		// Process deepest paths first.
		passRemoved := 0
		for i := len(dirs) - 1; i >= 0; i-- {
			if isDirEmpty(dirs[i]) {
				if err := os.Remove(dirs[i]); err == nil {
					passRemoved++
					removed++
				}
			}
		}
		if passRemoved == 0 {
			break
		}
	}
	return removed
}

// isDirEmpty reports whether dir contains no entries.
func isDirEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	return err != nil // io.EOF means empty
}

func shouldDelete(path string) bool {
	lower := strings.ToLower(path)

	// System and installation files that should definitely be deleted
	patterns := []string{
		// Downloaded app installers (Downloads folder)
		"/downloads/",

		// Specific apps that are often pre-installed or cached
		"/googlemaps",
		"/google maps",
		"/komoot",
		"/youtube",

		// System and installation files
		"/appdata/",
		"/localappdata/",
		"/programfiles/",
		"/program files/",
		"/system32/",
		"/windows/system32/",
		"/windowsupdate/",
		"/$recycle.bin",
		"/$recyclebin",
		"/system volume information",
		"/pagefile",
		"/hiberfil",

		// Application caches and temporary files
		"/.vscode/",
		"/.jetbrains/",
		"/.gradle/",
		"/.maven/",
		"/node_modules/",
		"/.npm/",
		"/vendor/",
		"/build/",
		"/dist/",
		"/.cache/",

		// Browser data
		"/appdata/local/google/chrome/",
		"/appdata/local/mozilla/firefox/",

		// Build artifacts and package managers
		"/.git/",
		"/.svn/",
		"/.hg/",
		"/obj/",
		"/bin/",

		// Common temp and cache locations
		"/temp/",
		"/tmp/",
	}

	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	// Extensions for executables, installers, and system files
	ext := strings.ToLower(filepath.Ext(path))
	deleteExtensions := []string{
		".exe", ".msi", ".dmg", ".apk", ".deb", ".rpm",
		".dll", ".so", ".dylib",
		".bat", ".cmd", ".ps1",
	}

	for _, delExt := range deleteExtensions {
		if ext == delExt {
			return true
		}
	}

	return false
}

