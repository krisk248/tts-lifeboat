# TTS Lifeboat

> Enterprise backup solution for Tomcat web applications

[![Go Version](https://img.shields.io/badge/Go-1.20%2B-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

```
████████╗████████╗███████╗    ██╗     ██╗███████╗███████╗██████╗  ██████╗  █████╗ ████████╗
╚══██╔══╝╚══██╔══╝██╔════╝    ██║     ██║██╔════╝██╔════╝██╔══██╗██╔═══██╗██╔══██╗╚══██╔══╝
   ██║      ██║   ███████╗    ██║     ██║█████╗  █████╗  ██████╔╝██║   ██║███████║   ██║
   ██║      ██║   ╚════██║    ██║     ██║██╔══╝  ██╔══╝  ██╔══██╗██║   ██║██╔══██║   ██║
   ██║      ██║   ███████║    ███████╗██║██║     ███████╗██████╔╝╚██████╔╝██║  ██║   ██║
   ╚═╝      ╚═╝   ╚══════╝    ╚══════╝╚═╝╚═╝     ╚══════╝╚═════╝  ╚═════╝ ╚═╝  ╚═╝   ╚═╝

                        "Your Tomcat's Best Friend"
                            Created by Kannan
```

## Features

- **Intelligent Backup** - Smart compression that skips already-compressed files (WAR, JAR, ZIP)
- **Checkpoint System** - Mark important backups that never auto-delete
- **Retention Policy** - Automatic cleanup with configurable retention days
- **Custom Folders** - Backup Tomcat config, logs, and other folders alongside webapps
- **Hybrid Interface** - TUI for interactive use, CLI for automation
- **Cross-Platform** - Windows (2008 R2, 10, 11) and Linux (RHEL 8+)

## Quick Start

### 1. Download

Download the appropriate binary for your platform:

| Platform | Binary |
|----------|--------|
| Windows x64 | `lifeboat_windows_amd64.exe` |
| Windows ARM64 | `lifeboat_windows_arm64.exe` |
| Linux x64 | `lifeboat_linux_amd64` |
| Linux ARM64 | `lifeboat_linux_arm64` |

### 2. Place in Backup Folder

Place `lifeboat.exe` (or `lifeboat`) in your backup destination folder:

```
C:\TTS\REManagement\UAE\ADX\
├── backup\                    ← Place lifeboat.exe HERE
│   ├── lifeboat.exe
│   ├── lifeboat.yaml          ← Create config here
│   └── logs\
│
└── Tomcat\
    └── webapps\               ← Your webapps (full path in YAML)
```

### 3. Create Configuration

Create `lifeboat.yaml` in the same folder:

```yaml
# Minimal configuration
name: "UAE-ADX"
environment: "production"

webapps_path: "C:\\TTS\\REManagement\\UAE\\ADX\\Tomcat\\webapps"
backup_path: "."

retention:
  days: 30
  min_keep: 5
```

Or generate a template:

```bash
lifeboat config init
```

### 4. Run Backup

**Interactive TUI:**
```bash
lifeboat
```

**CLI Backup:**
```bash
lifeboat backup --all
lifeboat backup --all --note "Pre-deployment"
lifeboat backup --checkpoint --note "Release v2.0"
```

## Commands

| Command | Description |
|---------|-------------|
| `lifeboat` | Launch interactive TUI |
| `lifeboat backup --all` | Backup all configured apps |
| `lifeboat backup --checkpoint` | Create checkpoint (never auto-deletes) |
| `lifeboat restore latest` | Restore most recent backup |
| `lifeboat restore <id>` | Restore specific backup |
| `lifeboat list` | List all backups |
| `lifeboat cleanup --force` | Remove expired backups |
| `lifeboat checkpoint <id>` | Mark backup as checkpoint |
| `lifeboat config validate` | Validate configuration |
| `lifeboat version --verbose` | Show version with Easter egg |

## Configuration

### Full Configuration Example

```yaml
# Instance identification
name: "UAE-ADX"
environment: "production"

# Paths
webapps_path: "C:\\TTS\\REManagement\\UAE\\ADX\\Tomcat\\webapps"
backup_path: "."

# Specific webapps to backup (empty = all)
webapps:
  - "AIWS.war"
  - "AIWS"
  - "IWS"
  - "MEET-ADX"

# Additional folders
custom_folders:
  - title: "Tomcat Config"
    path: "C:\\TTS\\REManagement\\UAE\\ADX\\Tomcat\\conf"
    required: true
  - title: "Shared Configs"
    path: "C:\\TTS\\shared-config"
    required: false

# Retention policy
retention:
  enabled: true
  days: 30
  min_keep: 5

# Compression
compression:
  enabled: true
  level: 6
  skip_extensions:
    - ".war"
    - ".jar"
    - ".zip"
    - ".gz"

# Logging
logging:
  path: "./logs/lifeboat.log"
  level: "info"
```

## Backup Structure

```
backup/
├── lifeboat.exe
├── lifeboat.yaml
├── index.json                    ← Quick lookup index
├── logs/
│   └── lifeboat.log
├── 20251230/
│   ├── 1104/
│   │   ├── webapp.tar.gz         ← Compressed webapps
│   │   ├── custom.tar.gz         ← Custom folders
│   │   └── metadata.json         ← Backup metadata
│   └── 1530/
│       └── ...
└── 20251228_Release_v2/          ← Checkpoint backup
    └── ...
```

## Building from Source

### Requirements
- Go 1.20+ (for Windows 2008 R2 / RHEL 8 compatibility)
- Go 1.24+ (for modern systems)

### Build

```bash
# Clone repository
git clone https://github.com/kannan/tts-lifeboat.git
cd tts-lifeboat

# Get dependencies
go mod tidy

# Build for current platform
go build -o lifeboat ./cmd/lifeboat

# Cross-compile
GOOS=windows GOARCH=amd64 go build -o lifeboat_windows_amd64.exe ./cmd/lifeboat
GOOS=linux GOARCH=amd64 go build -o lifeboat_linux_amd64 ./cmd/lifeboat
```

### Using GoReleaser

```bash
goreleaser release --snapshot --clean
```

## Platform Support

| Platform | Go Version | Notes |
|----------|------------|-------|
| Windows 2008 R2 | Go 1.20 | Use legacy binary |
| Windows 10/11 | Go 1.24 | Use modern binary |
| RHEL 8 | Go 1.20 | Use legacy binary |
| Modern Linux | Go 1.24 | Use modern binary |

## Author

**Kannan**

*"In case of sinking Tomcat, grab the Lifeboat!"*

## License

MIT License - see [LICENSE](LICENSE) for details.
