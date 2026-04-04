#!/usr/bin/env bash
# mount-win.sh — Mount the Windows NTFS boot partition to /mnt/win
#
# Usage:
#   sudo ./mount-win.sh           # auto-detect, mount read-only (safe)
#   sudo ./mount-win.sh --rw      # mount read-write (only if Windows is fully shut down)
#   sudo ./mount-win.sh /dev/sdXY # explicit partition, read-only
#   sudo ./mount-win.sh /dev/sdXY --rw
#
# Requires: ntfs-3g (apt install ntfs-3g)

set -euo pipefail

MOUNT_POINT="/mnt/win"
RW=false
DEVICE=""

# ── Parse arguments ──────────────────────────────────────────────────────────
for arg in "$@"; do
    case "$arg" in
        --rw) RW=true ;;
        /dev/*) DEVICE="$arg" ;;
        *)
            echo "Unknown argument: $arg" >&2
            echo "Usage: sudo $0 [/dev/PARTITION] [--rw]" >&2
            exit 1
            ;;
    esac
done

# ── Root check ───────────────────────────────────────────────────────────────
if [[ $EUID -ne 0 ]]; then
    echo "Error: this script must be run as root (use sudo)." >&2
    exit 1
fi

# ── Check ntfs-3g is available ───────────────────────────────────────────────
if ! command -v ntfs-3g &>/dev/null && ! command -v mount.ntfs &>/dev/null; then
    echo "Error: ntfs-3g is not installed. Run: sudo apt install ntfs-3g" >&2
    exit 1
fi

# ── Auto-detect Windows NTFS partition if none given ─────────────────────────
if [[ -z "$DEVICE" ]]; then
    echo "Auto-detecting Windows NTFS partition..."

    # Collect all NTFS block devices with their labels and sizes
    # Output columns: NAME  FSTYPE  LABEL  SIZE
    mapfile -t candidates < <(
        lsblk -rpno NAME,FSTYPE,LABEL,SIZE \
        | awk '$2 == "ntfs" {print}'
    )

    if [[ ${#candidates[@]} -eq 0 ]]; then
        echo "Error: no NTFS partitions found." >&2
        exit 1
    fi

    # Prefer a partition whose label looks like a Windows system drive
    for line in "${candidates[@]}"; do
        label=$(echo "$line" | awk '{print tolower($3)}')
        if [[ "$label" == *windows* || "$label" == *os* || "$label" == "c" ]]; then
            DEVICE=$(echo "$line" | awk '{print $1}')
            echo "  Found by label '$label': $DEVICE"
            break
        fi
    done

    # Fall back to the largest NTFS partition
    if [[ -z "$DEVICE" ]]; then
        DEVICE=$(
            lsblk -rpno NAME,FSTYPE,SIZE \
            | awk '$2 == "ntfs" {print $3, $1}' \
            | sort -rh \
            | head -1 \
            | awk '{print $2}'
        )
        if [[ -z "$DEVICE" ]]; then
            echo "Error: could not select an NTFS partition." >&2
            exit 1
        fi
        echo "  No Windows-labelled partition found, using largest NTFS: $DEVICE"
    fi
fi

# Verify device exists
if [[ ! -b "$DEVICE" ]]; then
    echo "Error: '$DEVICE' is not a block device." >&2
    exit 1
fi

# ── Check if already mounted ──────────────────────────────────────────────────
if mountpoint -q "$MOUNT_POINT"; then
    echo "$MOUNT_POINT is already mounted."
    mount | grep "$MOUNT_POINT"
    exit 0
fi

if grep -q "^$DEVICE " /proc/mounts 2>/dev/null; then
    existing=$(awk -v d="$DEVICE" '$1==d {print $2; exit}' /proc/mounts)
    echo "Error: $DEVICE is already mounted at $existing" >&2
    exit 1
fi

# ── Create mount point ────────────────────────────────────────────────────────
mkdir -p "$MOUNT_POINT"

# ── Hibernation / fast-startup check ─────────────────────────────────────────
# Mount temporarily read-only to check for hiberfil.sys before any RW mount.
if $RW; then
    echo "Checking for Windows hibernation / fast startup..."
    mount -t ntfs-3g -o ro,noatime "$DEVICE" "$MOUNT_POINT"

    if [[ -f "$MOUNT_POINT/hiberfil.sys" ]]; then
        # A non-empty hiberfil.sys with magic bytes 'hibr' means Windows is hibernated.
        magic=$(xxd -p -l 4 "$MOUNT_POINT/hiberfil.sys" 2>/dev/null || true)
        umount "$MOUNT_POINT"
        if [[ "$magic" == "6869627" || "$magic" == "68696272" ]]; then
            echo ""
            echo "WARNING: Windows is hibernated (fast startup is on)." >&2
            echo "Mounting read-write on a hibernated partition can corrupt it." >&2
            echo "Either:" >&2
            echo "  1. Boot Windows, disable Fast Startup, then shut down fully." >&2
            echo "  2. Remove '$MOUNT_POINT/hiberfil.sys' from within Windows." >&2
            echo "  3. Run without --rw to mount safely in read-only mode." >&2
            exit 1
        fi
        echo "  hiberfil.sys present but Windows is not hibernated — safe to continue."
    else
        umount "$MOUNT_POINT"
        echo "  No hibernation detected."
    fi
fi

# ── Mount ─────────────────────────────────────────────────────────────────────
MOUNT_OPTS="noatime"
if $RW; then
    MOUNT_OPTS="${MOUNT_OPTS},uid=$(id -u matthias 2>/dev/null || echo 1000),gid=$(id -g matthias 2>/dev/null || echo 1000),umask=022"
    echo "Mounting $DEVICE → $MOUNT_POINT (read-write)..."
else
    MOUNT_OPTS="ro,${MOUNT_OPTS}"
    echo "Mounting $DEVICE → $MOUNT_POINT (read-only)..."
fi

mount -t ntfs-3g -o "$MOUNT_OPTS" "$DEVICE" "$MOUNT_POINT"

echo ""
echo "Mounted successfully."
mount | grep "$MOUNT_POINT"
echo ""
echo "Disk usage:"
df -h "$MOUNT_POINT"
