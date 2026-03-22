package runner

type Config struct {
	Sources         []string
	DestinationRoot string
	DryRun          bool
	ReportPath      string
	MaxArchiveDepth int
	Logf            func(format string, args ...any)
}

type Report struct {
	DryRun bool `json:"dryRun"`

	TotalFilesSeen     int `json:"totalFilesSeen"`
	CopiedFiles        int `json:"copiedFiles"`
	SkippedDuplicates  int `json:"skippedDuplicates"`
	SkippedPrograms    int `json:"skippedPrograms"`
	ArchivesProcessed  int `json:"archivesProcessed"`
	ArchivesExtracted  int `json:"archivesExtracted"`
	UnsupportedArchive int `json:"unsupportedArchive"`
	Failures           int `json:"failures"`

	ByCategory map[string]int `json:"byCategory"`
	Errors     []string       `json:"errors"`
}
