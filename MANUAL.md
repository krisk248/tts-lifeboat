# TTS Lifeboat - User Manual

> Complete guide to using TTS Lifeboat for Tomcat webapp backups

## Table of Contents

1. [Installation](#installation)
2. [Configuration](#configuration)
3. [Using the TUI](#using-the-tui)
4. [Using the CLI](#using-the-cli)
5. [Backup Operations](#backup-operations)
6. [Restore Operations](#restore-operations)
7. [Retention & Cleanup](#retention--cleanup)
8. [Checkpoints](#checkpoints)
9. [Automation](#automation)
10. [Troubleshooting](#troubleshooting)

---

## Installation

### Step 1: Download the Binary

Download the appropriate binary for your operating system:

**Windows:**
- `lifeboat_windows_amd64.exe` - Windows x64 (most common)
- `lifeboat_windows_arm64.exe` - Windows ARM64

**Linux:**
- `lifeboat_linux_amd64` - Linux x64
- `lifeboat_linux_arm64` - Linux ARM64

### Step 2: Place in Backup Location

**IMPORTANT:** The lifeboat executable should be placed in your **backup folder**, NOT in the webapps folder.

**Correct Structure:**
```
C:\TTS\REManagement\UAE\ADX\
├── backup\                              ← PUT LIFEBOAT HERE
│   ├── lifeboat.exe
│   ├── lifeboat.yaml
│   └── (backups will be created here)
│
└── Tomcat\
    └── webapps\                         ← REFERENCE THIS IN YAML
        ├── AIWS.war
        ├── AIWS\
        └── ...
```

### Step 3: Make Executable (Linux Only)

```bash
chmod +x lifeboat
```

---

## Configuration

### Creating a Configuration File

You can create a configuration file in two ways:

**Option 1: Generate Template**
```bash
lifeboat config init
```

**Option 2: Create Manually**

Create `lifeboat.yaml` in the same folder as the executable:

```yaml
# Instance identification
name: "UAE-ADX"
environment: "production"

# Path to webapps (REQUIRED - use full absolute path)
webapps_path: "C:\\TTS\\REManagement\\UAE\\ADX\\Tomcat\\webapps"

# Backup destination (. means current folder)
backup_path: "."

# Specific webapps to backup (leave empty for ALL)
webapps:
  - "AIWS.war"
  - "AIWS"
  - "IWS"
  - "MEET-ADX"
  - "MeetNVote"

# Additional folders to include
custom_folders:
  - title: "Tomcat Config"
    path: "C:\\TTS\\REManagement\\UAE\\ADX\\Tomcat\\conf"
    required: true      # Backup fails if missing
  - title: "Shared Configs"
    path: "C:\\TTS\\shared-config"
    required: false     # Skips if missing

# Retention policy
retention:
  enabled: true
  days: 30              # Delete after 30 days
  min_keep: 5           # Always keep at least 5 backups

# Compression settings
compression:
  enabled: true
  level: 6              # 1-9 (higher = smaller, slower)
  skip_extensions:
    - ".war"
    - ".jar"
    - ".zip"
    - ".gz"

# Logging
logging:
  path: "./logs/lifeboat.log"
  level: "info"         # debug, info, warn, error
```

### Validating Configuration

Before running your first backup, validate the configuration:

```bash
lifeboat config validate
```

This checks:
- All required fields are present
- Paths exist and are accessible
- Values are within valid ranges

### Showing Current Configuration

```bash
lifeboat config show
```

---

## Using the TUI

Launch the TUI by running lifeboat without any arguments:

```bash
lifeboat
```

On Windows, double-click `lifeboat.exe` or run from Command Prompt.

### TUI Navigation

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Select |
| `b` | New Backup |
| `r` | Restore |
| `l` | List Backups |
| `c` | Cleanup |
| `q` | Quit |
| `?` | Help |
| `Esc` | Cancel/Back |

### Easter Egg

Type "kannan" anywhere in the TUI for a special surprise!

---

## Using the CLI

The CLI is ideal for scripting and automation.

### Basic Commands

```bash
# Show help
lifeboat --help

# Show version
lifeboat version

# Show version with Easter egg
lifeboat version --verbose
```

### Using a Custom Config File

```bash
lifeboat -c /path/to/custom.yaml backup --all
lifeboat --config C:\configs\production.yaml list
```

### Verbose Output

```bash
lifeboat -v backup --all
```

---

## Backup Operations

### Interactive Backup (TUI)

1. Run `lifeboat`
2. Press `b` or select "New Backup"
3. Monitor progress
4. Review summary

### Non-Interactive Backup (CLI)

```bash
# Backup all configured webapps
lifeboat backup --all

# Backup with a note
lifeboat backup --all --note "Pre-deployment backup"

# Create a checkpoint backup (never auto-deletes)
lifeboat backup --checkpoint --note "Release v2.5.0"

# Preview backup (dry run)
lifeboat backup --dry-run
```

### Backup Output

Backups are stored in date/time folders:

```
backup/
├── 20251230/               ← Date folder (YYYYMMDD)
│   ├── 1104/               ← Time folder (HHMM = 11:04)
│   │   ├── webapp.tar.gz
│   │   ├── custom.tar.gz
│   │   └── metadata.json
│   └── 1530/
│       └── ...
```

### Metadata

Each backup includes `metadata.json`:

```json
{
  "id": "backup-20251230-110432",
  "created_at": "2025-12-30T11:04:32+05:30",
  "duration_seconds": 23,
  "files": {
    "count": 82,
    "original_size": "12.8 MB",
    "compressed_size": "4.2 MB"
  },
  "note": "Pre-deployment backup"
}
```

---

## Restore Operations

### List Available Backups

```bash
lifeboat list
lifeboat list --limit 5
lifeboat list --checkpoints
lifeboat list --json
```

### Restore Latest Backup

```bash
lifeboat restore latest
lifeboat restore latest --target ./rollback
```

### Restore Specific Backup

```bash
lifeboat restore backup-20251230-110432
lifeboat restore backup-20251230-110432 --target C:\restore\test
```

### Restore Process

1. Creates target directory (default: `./rollback`)
2. Extracts webapp archive
3. Extracts custom folder archive
4. Preserves original file structure

**IMPORTANT:** Review restored files before replacing production files!

---

## Retention & Cleanup

### How Retention Works

1. Backups older than `retention.days` are marked for deletion
2. `min_keep` ensures minimum backups are always kept
3. **Checkpoint backups are NEVER auto-deleted**

### Preview Cleanup

```bash
lifeboat cleanup
lifeboat cleanup --dry-run
```

### Execute Cleanup

```bash
lifeboat cleanup --force
```

### Manual Deletion

For force-deleting a specific backup (even checkpoints):

```bash
# Currently only via direct file deletion
# Be careful!
```

---

## Checkpoints

Checkpoints are special backups that never auto-delete, regardless of retention policy.

### Creating a Checkpoint

**During Backup:**
```bash
lifeboat backup --checkpoint --note "Release v2.5.0"
```

**Mark Existing Backup:**
```bash
lifeboat checkpoint backup-20251230-110432 --note "Important milestone"
lifeboat checkpoint latest --note "Known good state"
```

### Checkpoint Naming

Checkpoint folders use a special format:
```
20251230_Release_v2_5/
20251228_Pre_migration/
```

### Viewing Checkpoints

```bash
lifeboat list --checkpoints
```

---

## Automation

### Windows Task Scheduler

1. Open Task Scheduler
2. Create Basic Task
3. Set trigger (daily, weekly, etc.)
4. Action: Start a program
   - Program: `C:\path\to\backup\lifeboat.exe`
   - Arguments: `backup --all --note "Scheduled backup"`
   - Start in: `C:\path\to\backup\`

### Linux Cron

```bash
# Edit crontab
crontab -e

# Add daily backup at 2 AM
0 2 * * * /path/to/backup/lifeboat backup --all --note "Cron backup" >> /path/to/backup/logs/cron.log 2>&1

# Weekly checkpoint on Sunday at 3 AM
0 3 * * 0 /path/to/backup/lifeboat backup --checkpoint --note "Weekly checkpoint" >> /path/to/backup/logs/cron.log 2>&1

# Daily cleanup at 4 AM
0 4 * * * /path/to/backup/lifeboat cleanup --force >> /path/to/backup/logs/cron.log 2>&1
```

### PowerShell Script

```powershell
# backup-tomcat.ps1
$lifeboat = "C:\TTS\backup\lifeboat.exe"
$config = "C:\TTS\backup\lifeboat.yaml"

# Run backup
& $lifeboat -c $config backup --all --note "PowerShell backup"

# Check result
if ($LASTEXITCODE -eq 0) {
    Write-Host "Backup completed successfully"
} else {
    Write-Host "Backup failed with exit code $LASTEXITCODE"
    # Send notification, etc.
}
```

### Bash Script

```bash
#!/bin/bash
# backup-tomcat.sh

LIFEBOAT="/opt/backup/lifeboat"
CONFIG="/opt/backup/lifeboat.yaml"
LOG="/opt/backup/logs/backup.log"

echo "$(date) - Starting backup" >> $LOG

$LIFEBOAT -c $CONFIG backup --all --note "Script backup" >> $LOG 2>&1

if [ $? -eq 0 ]; then
    echo "$(date) - Backup completed" >> $LOG
else
    echo "$(date) - Backup FAILED" >> $LOG
    # Send alert
fi
```

---

## Troubleshooting

### Common Issues

#### "Config file not found"

Ensure `lifeboat.yaml` is in the same directory as the executable, or specify with `-c`:

```bash
lifeboat -c /full/path/to/lifeboat.yaml backup --all
```

#### "webapps_path does not exist"

Check that the path in your YAML is correct:
- Use double backslashes on Windows: `C:\\TTS\\Tomcat\\webapps`
- Use forward slashes on Linux: `/opt/tomcat/webapps`
- Ensure the path is accessible

#### "Permission denied"

- Windows: Run as Administrator if needed
- Linux: Check file permissions (`chmod +x lifeboat`)

#### "Backup too large"

- Enable compression in YAML
- Exclude unnecessary files with patterns
- Consider backing up specific webapps instead of all

### Logs

Check logs for detailed information:

```bash
cat ./logs/lifeboat.log
```

### Verbose Mode

Run with `-v` for debug output:

```bash
lifeboat -v backup --all
```

### Validate Configuration

Always validate after changes:

```bash
lifeboat config validate
```

---

## Support

Created by **Kannan**

*"In case of sinking Tomcat, grab the Lifeboat!"*

For issues, check the logs first, then validate configuration. Most problems are path-related!
