# File Cleanup Tool

This tool helps remove unnecessary files from your archived data, such as:
- Downloaded files (from `Downloads/` folders)
- Executable and installer files (`.exe`, `.msi`, `.apk`, `.dll`, etc.)
- System and application cache directories
- Build artifacts and package manager directories
- Development tool caches

## Building

```bash
go build -o cleanup-tool ./cmd/cleanup
```

## Usage

### Dry-run Mode (default - safe to use)

Preview what would be deleted without actually deleting anything:

```bash
./cleanup-tool -dry-run=true -input=all-files.txt | head -50
```

Check the summary at the end of the output to see how many files would be deleted:

```bash
./cleanup-tool -dry-run=true -input=all-files.txt 2>&1 | tail -5
```

### Actual Deletion

Once you're satisfied with the dry-run results, delete the files:

```bash
./cleanup-tool -dry-run=false -input=all-files.txt
```

## Options

- `-dry-run` (default: `true`) - When `true`, prints files to be deleted to stdout without removing them. When `false`, actually deletes the files.
- `-input` (default: `all-files.txt`) - Path to the file list to process.

## What Gets Deleted

The tool removes:

1. **Files in Downloads folders**
   - Everything under `/downloads/` paths

2. **Executables and installers**
   - `.exe`, `.msi`, `.dmg`, `.apk`, `.deb`, `.rpm`
   - `.dll`, `.so`, `.dylib`
   - `.bat`, `.cmd`, `.ps1`

3. **System files and cache**
   - Windows system directories (`System32`, `Windows`, `AppData`, `LocalAppData`, `ProgramFiles`)
   - System volumes and recycle bins
   - Browser caches
   - Application caches (vscode, jetbrains, gradle, maven, npm, etc.)
   - Build directories (`.git`, `/build`, `/dist`, `/obj`, `/bin`)

4. **Specific apps** (often pre-installed or cached)
   - Google Maps, Komoot, YouTube

## Safety

- **Dry-run is the default** - you must explicitly set `-dry-run=false` to delete files
- Files are removed one at a time - errors on individual files don't stop the process
- Errors are reported to stderr while dry-run output goes to stdout
