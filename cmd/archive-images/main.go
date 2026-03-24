package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"archive-images/internal/classify"
	"archive-images/internal/runner"
)

func main() {
	var sourceCSV string
	var destination string
	var dryRun bool
	var reportPath string
	var maxArchiveDepth int
	var categoriesCSV string
	var debug bool

	flag.StringVar(&sourceCSV, "sources", "", "Comma-separated source directories to scan")
	flag.StringVar(&destination, "dest", "", "Destination directory (e.g. mounted USB path)")
	flag.BoolVar(&dryRun, "dry-run", true, "If true, only print planned actions without copying")
	flag.StringVar(&reportPath, "report", "", "Optional path for JSON report output")
	flag.IntVar(&maxArchiveDepth, "max-archive-depth", 5, "Maximum nested archive extraction depth")
	flag.StringVar(&categoriesCSV, "categories", "", "Comma-separated categories to include: pictures, movies, documents, sound, other (default: all)")
	flag.BoolVar(&debug, "debug", false, "Print verbose debug output (directories entered, archives, skips, duplicates)")
	flag.Parse()

	sources := splitSources(sourceCSV)
	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "error: provide at least one source via -sources")
		os.Exit(2)
	}
	if destination == "" {
		fmt.Fprintln(os.Stderr, "error: provide destination via -dest")
		os.Exit(2)
	}

	enabledCategories, err := parseCategories(categoriesCSV)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	cfg := runner.Config{
		Sources:           sources,
		DestinationRoot:   destination,
		DryRun:            dryRun,
		ReportPath:        reportPath,
		MaxArchiveDepth:   maxArchiveDepth,
		EnabledCategories: enabledCategories,
		Logf:              func(format string, args ...any) { fmt.Printf(format+"\n", args...) },
	}
	if debug {
		cfg.Debugf = func(format string, args ...any) { fmt.Printf("[DEBUG] "+format+"\n", args...) }
	}

	report, err := runner.Run(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done: files=%d copied=%d duplicates=%d skipped_program=%d archives=%d failures=%d dry_run=%t\n",
		report.TotalFilesSeen,
		report.CopiedFiles,
		report.SkippedDuplicates,
		report.SkippedPrograms,
		report.ArchivesProcessed,
		report.Failures,
		report.DryRun,
	)
}

// parseCategories maps user-friendly category names to canonical category constants.
// Returns nil (all categories enabled) when csv is empty.
func parseCategories(csv string) ([]string, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, nil
	}
	aliases := map[string]string{
		"pictures":  classify.CategoryPictures,
		"picture":   classify.CategoryPictures,
		"photos":    classify.CategoryPictures,
		"photo":     classify.CategoryPictures,
		"movies":    classify.CategoryVideos,
		"movie":     classify.CategoryVideos,
		"videos":    classify.CategoryVideos,
		"video":     classify.CategoryVideos,
		"documents": classify.CategoryDocuments,
		"document":  classify.CategoryDocuments,
		"docs":      classify.CategoryDocuments,
		"doc":       classify.CategoryDocuments,
		"sound":     classify.CategoryMusic,
		"sounds":    classify.CategoryMusic,
		"music":     classify.CategoryMusic,
		"audio":     classify.CategoryMusic,
		"other":     classify.CategoryOther,
	}

	seen := map[string]struct{}{}
	var result []string
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cat, ok := aliases[strings.ToLower(p)]
		if !ok {
			return nil, fmt.Errorf("unknown category %q; valid values: pictures, movies, documents, sound, other", p)
		}
		if _, exists := seen[cat]; !exists {
			seen[cat] = struct{}{}
			result = append(result, cat)
		}
	}
	return result, nil
}

func splitSources(csv string) []string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
