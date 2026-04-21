# TTS Lifeboat

> Simple menu-driven backup tool for Tomcat webapps.

```
===============================================
   TTS LIFEBOAT v0.3.0
   Created by Kannan from TTS
   Project: IPO-MIGRATION
===============================================

What would you like to do?

  1. Create New Backup
  2. View Backup History
  3. Cleanup Old Backups (older than 30 days)
  4. Exit
```

One binary. One TOML file. One menu. That's it.

## Install

1. Copy `lifeboat.exe` (Windows) or `lifeboat` (Linux) into a folder next to
   your Tomcat install, e.g. `C:\TTS\MyApp\backup\`.
2. Run it once in that folder to generate a starter config:

   ```
   lifeboat init
   ```

3. Edit `lifeboat.toml` and set `name` and `webapps_path`.
4. Run `lifeboat` (no arguments) to open the menu.

## Configuration (`lifeboat.toml`)

Six fields, flat, no sections:

```toml
name          = "IPO-MIGRATION"
webapps_path  = "C:/TTS/IPO-MIGRATION/Tomcat/webapps"
backup_path   = "."          # . = same folder as lifeboat.exe
compression   = false        # false = plain copy, true = .tar.zst
retention_days = 30          # 0 = keep forever
extra_folders = []           # optional: Tomcat conf, shared configs, ...
```

`compression` defaults to `false` on Windows and `true` on Linux when you run
`lifeboat init`. Flip it any time.

## What each menu option does

- **1. Create New Backup** - Lists every entry in `webapps_path` with a number
  and size. Type the numbers you want (`1,3,10`) or press Enter for all. Items
  are copied (or compressed to `.tar.zst`) into
  `backup_path/YYYYMMDD/HHMM/`. Extra folders are backed up alongside.

- **2. View Backup History** - Lists every past backup, newest first, with
  timestamp, size, and path.

- **3. Cleanup Old Backups** - Previews backups older than `retention_days`,
  asks for confirmation, then deletes them. Empty date folders are removed too.

- **4. Exit** - Quits.

## Where things live

```
C:\TTS\MyApp\
├── backup\
│   ├── lifeboat.exe
│   ├── lifeboat.toml
│   ├── logs\
│   │   └── lifeboat.log         ← every action is logged here
│   └── 20260421\
│       ├── 2117\                ← one backup: 21 Apr 2026 at 21:17
│       │   ├── AIWS\            ← plain copy (compression=false)
│       │   ├── IWS\
│       │   ├── app.war
│       │   └── conf\            ← from extra_folders
│       └── 2340\
│           ├── AIWS.tar.zst     ← archive (compression=true)
│           └── ...
└── Tomcat\
    └── webapps\                 ← referenced by webapps_path
```

## Automation (optional)

Scheduled non-interactive backup of everything:

**Windows Task Scheduler**

- Program: `lifeboat.exe`
- Start in: `C:\TTS\MyApp\backup`
- Arguments: leave blank, pipe stdin via a `.cmd` wrapper:

  ```cmd
  @echo off
  cd /d C:\TTS\MyApp\backup
  (echo 1 & echo. & echo.) | lifeboat.exe
  ```

  (`1` = New Backup, blank = all items, blank = Press Enter to continue.)

**Linux cron**

```
0 2 * * * cd /opt/tts/backup && printf '1\n\n\n4\n' | ./lifeboat >> /dev/null
```

## Build from source

Requires Go 1.21+.

```bash
go build -o lifeboat ./cmd/lifeboat                                # current OS
GOOS=windows GOARCH=amd64 go build -o lifeboat.exe ./cmd/lifeboat  # cross
```

Dependencies (two):

- `github.com/BurntSushi/toml` - config parsing
- `github.com/klauspost/compress` - zstd encoder for `compression = true`

## License

MIT.

*"In case of sinking Tomcat, grab the Lifeboat."*
