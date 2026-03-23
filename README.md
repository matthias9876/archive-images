# archive-images

Reusable Go CLI for reorganizing mixed backup files into personal-data categories.

## Features

- Recursive scan of one or more source folders
- Program/download exclusion using extension and folder heuristics
- Embedded media in music folders (like AlbumArtSmall.jpg) classified as `Other` instead of `Pictures`
- Category mapping: Documents, Pictures, Videos, Music, Other
- Duplicate detection using MD5 (keep first, skip later duplicates)
- Archive extraction and recursion for: **zip, tar, tar.gz, tgz, rar**
- Cross-source deduplication: same destination with different sources automatically handles already-copied files
- **Resumable processing**: use the same parameters to resume an interrupted run (restarts from where it stopped)
- Optional category filter — copy only the file types you want
- Dry-run mode (default) and optional JSON report output

## Build

```bash
go build ./cmd/archive-images
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-sources` | *(required)* | Comma-separated source directories to scan |
| `-dest` | *(required)* | Destination directory (e.g. mounted USB path) |
| `-dry-run` | `true` | Print planned actions without copying anything |
| `-categories` | *(all)* | Comma-separated file types to include (see below) |
| `-report` | *(none)* | Path for a JSON report file |
| `-max-archive-depth` | `5` | Maximum nested archive extraction depth |

### Category values for `-categories`

| Value(s) | Destination folder |
|---|---|
| `pictures`, `photos` | `Pictures/` |
| `movies`, `videos` | `Videos/` |
| `documents`, `docs` | `Documents/` |
| `sound`, `music`, `audio` | `Music/` |
| `other` | `Other/` |

Multiple values are comma-separated. Omitting the flag includes **all** categories.

## Usage examples

**Dry-run across two sources (default — no files are written):**

```bash
./archive-images \
  -sources "/mnt/nas/backup1,/mnt/nas/backup2" \
  -dest "/media/usb/organized"
```

**Copy only pictures and movies:**

```bash
./archive-images \
  -sources "/mnt/nas/backup" \
  -dest "/media/usb/organized" \
  -categories "pictures,movies" \
  -dry-run=false
```

**Copy only documents:**

```bash
./archive-images \
  -sources "/mnt/nas/backup" \
  -dest "/media/usb/organized" \
  -categories "documents" \
  -dry-run=false
```

**Copy sound files and pictures with a JSON report:**

```bash
./archive-images \
  -sources "/mnt/nas/backup" \
  -dest "/media/usb/organized" \
  -categories "sound,pictures" \
  -report "/tmp/report.json" \
  -dry-run=false
```

**Copy everything (all categories):**

```bash
./archive-images \
  -sources "/mnt/nas/backup" \
  -dest "/media/usb/organized" \
  -dry-run=false
```

## Output structure

```
<dest>/
  Documents/
  Pictures/
  Videos/
  Music/
  Other/
```

Files that are duplicates (same MD5) or identified as programs/installers are skipped and counted in the summary line printed at the end.

## Resumability and cross-source deduplication

The tool automatically creates a hidden manifest file (`.archive-images-manifest.json`) in the destination directory to track all processed files. This enables:

**Resumability**: If the process is interrupted, run the same command again with the same parameters. The tool will skip already-processed files and continue from where it left off.

```bash
# First run (interrupted after some files)
./archive-images -sources "/mnt/nas/backup1" -dest "/media/usb/organized" -dry-run=false

# Resume later (same command, same parameters)
./archive-images -sources "/mnt/nas/backup1" -dest "/media/usb/organized" -dry-run=false
```

**Cross-source deduplication**: When combining multiple backup sources into the same destination, duplicates are detected and skipped—even if they come from different sources. You can safely run multiple times with different sources:

```bash
# Run with first source
./archive-images -sources "/mnt/nas/backup1" -dest "/media/usb/organized" -dry-run=false

# Run with second source (duplicates from backup1 are skipped)
./archive-images -sources "/mnt/nas/backup2" -dest "/media/usb/organized" -dry-run=false

# Run with both sources together (all duplicates skipped)
./archive-images -sources "/mnt/nas/backup1,/mnt/nas/backup2" -dest "/media/usb/organized" -dry-run=false
```

The manifest is only updated during real runs (`-dry-run=false`). Dry runs do not modify the manifest.

## Safety tips

- Always run with `-dry-run` (the default) first to review planned copies before writing anything.
- Use `-report` to save a full JSON log of every action and any errors.
- Nested archive extraction is bounded by `-max-archive-depth` (default `5`) to protect against archive bombs.
- RAR archives require the `unrar` utility. Install it with: `apt-get install unrar` (Linux) or `brew install unrar` (macOS).
- Image files found inside music folders (like `Music/album/Cover.jpg`) are classified as `Other` to avoid cluttering your `Pictures` folder with album artwork.
- Do not manually edit or delete the `.archive-images-manifest.json` file unless you want to re-process previously copied files.

