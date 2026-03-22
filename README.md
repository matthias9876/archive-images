# archive-images

Reusable Go CLI for reorganizing mixed backup files into personal-data categories.

## Implemented (initial slice)

- Recursive scan of one or more source folders
- Program/download exclusion using extension and folder heuristics
- Category mapping: Documents, Pictures, Videos, Music, Other
- Duplicate detection using MD5 (keep first, skip later duplicates)
- Archive extraction and recursion for: zip, tar, tar.gz, tgz
- Dry-run mode (default) and optional JSON report output

## Usage

Build:

```bash
go build ./cmd/archive-images
```

Dry-run (default):

```bash
./archive-images -sources "/path/backup1,/path/backup2" -dest "/media/usb/organized"
```

Real copy run:

```bash
./archive-images -sources "/path/backup" -dest "/media/usb/organized" -dry-run=false
```

With report:

```bash
./archive-images -sources "/path/backup" -dest "/media/usb/organized" -report "/tmp/archive-report.json"
```

## Notes

- Nested archive extraction is bounded with `-max-archive-depth` (default `5`).
- Unsupported archive formats (such as rar/7z) are counted in the report but not extracted.
- Run dry-run first to verify the planned actions.

