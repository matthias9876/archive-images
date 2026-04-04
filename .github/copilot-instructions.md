# Copilot Instructions

## What this tool does

`archive-images` is a CLI tool that recursively scans source directories and reorganizes files into personal-data categories (`Documents`, `Pictures`, `Videos`, `Music`, `Other`). It handles archive extraction (zip, tar, tar.gz, rar), deduplication via MD5 hashes, and is resumable across runs.

## Build, test, and lint

```bash
# Build
go build ./cmd/archive-images

# Run all tests
go test ./...

# Run a single package
go test ./internal/classify

# Run a single test
go test ./internal/classify -run TestCategoryFor

# Cross-compile (used in CI)
GOOS=linux GOARCH=amd64 go build -o archive-images-linux-amd64 ./cmd/archive-images
```

No Makefile or linter config — CI runs `go test ./...` via `.github/workflows/test.yml`.

## Architecture

```
cmd/archive-images/main.go   → parses CLI flags, validates inputs, calls runner.Run()
internal/runner/             → orchestrates file walking, copying, dedup, manifest I/O
internal/classify/           → maps file extensions + folder heuristics → category
internal/archive/            → extracts zip, tar, tar.gz, tgz; calls unrar binary for .rar
internal/filter/             → detects and skips program/installer files
```

**Core flow in `runner.Run`:**
1. Load manifest (`.archive-images-manifest.json` in dest) for cross-run dedup
2. Walk sources via a depth-first queue; archives are extracted to a temp dir and re-queued
3. For each file: classify → filter programs → compute MD5 while copying (single-pass via `io.TeeReader`) → check dedup → atomic rename from `.tmp` to final dest
4. Save manifest (skipped in dry-run mode)

**Output layout:**
```
<dest>/
  Documents/<source-name>/...
  Pictures/<source-name>/...
  Videos/<source-name>/...
  Music/<source-name>/...
  Other/<source-name>/...
```

## Key conventions

**No third-party dependencies** — stdlib only. `go.sum` is intentionally empty. Do not add external packages without a strong reason.

**Error handling:**
- Return `error` as the last value; always check `if err != nil`
- Wrap errors with context: `fmt.Errorf("doing X: %w", err)`
- Non-fatal per-file errors are accumulated in `report.Errors` rather than halting the walk

**Testing:**
- All test funcs call `t.Parallel()` at the top
- Use table-driven tests with `t.Run` subtests
- Use `t.TempDir()` for isolated filesystem state — no mocks, no test doubles
- Integration tests live in `run_integration_test.go` and test full end-to-end flows

**Safety defaults:**
- `-dry-run` defaults to `true` — must explicitly pass `-dry-run=false` to write files
- `secureJoin()` in the archive package rejects path traversal (`../`) from archive entries
- `-max-archive-depth` (default 5) limits recursive archive extraction

**Performance:**
- `hashAndCopy` uses `io.TeeReader` to compute MD5 in a single read pass
- 4 MiB copy buffer (`copyBufSize`) to amortize HDD seek latency
- Files are written to a `.tmp` file first, then renamed atomically on success

**Heuristics worth knowing:**
- `classify.CategoryFor` is case-insensitive on extensions and checks if the file is inside a music folder (e.g. path contains `music/`, `albums/`, `podcasts/`) before categorizing images — those become `Other` rather than `Pictures`
- `filter.IsLikelyProgram` blocks `.exe`, `.msi`, `.dmg`, `.apk` and paths containing tokens like `/downloads/`, `/steamapps/`, `/installers/`

**Category CLI aliases** — `main.go` maps user-friendly names to canonical constants before calling `runner.Run`:
- `photos` → `Pictures`
- `docs` / `documents` → `Documents`
- `movies` / `videos` → `Videos`
- `sound` / `music` / `audio` → `Music`

**Deduplication has two stages:**
1. Manifest check (hashes from previous runs, loaded at startup)
2. In-run hash set (hashes seen earlier in the current walk)

Both must miss before a file is written. Manifest is only updated on real runs (not dry-run).

**Filename collision avoidance:** `uniqueDestinationPath` appends `_N` before the extension (e.g. `photo_1.jpg`) when a path is already planned or already exists on disk.

**Import paths:** `archive-images/internal/{runner,classify,archive,filter}`
