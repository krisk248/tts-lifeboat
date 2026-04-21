# CONTEXT.md

Read this first. It tells an AI assistant (or a new human) everything needed
to work on this repo without having to reverse-engineer it.

## What this project is

**TTS Lifeboat** is a single-binary, menu-driven backup tool for Apache Tomcat
web applications. It copies (or compresses) files out of
`webapps_path` into timestamped folders under `backup_path`, and optionally
deletes old copies after N days.

The tool deliberately does **not** have: restore, checkpoints, CLI subcommands,
a TUI, a remote backend, encryption, or notifications. If you are tempted to
add any of these, stop and ask the owner first - simplicity is the explicit
design goal (see Design Principles below).

## How to run it

```bash
go build -o lifeboat ./cmd/lifeboat
cd /some/backup/folder
./lifeboat init        # write lifeboat.toml template
$EDITOR lifeboat.toml  # set name + webapps_path
./lifeboat             # menu
```

The menu has four options: New Backup, View History, Cleanup, Exit.

## Repo layout

```
cmd/lifeboat/main.go                    Entry point + menu loop
internal/app/version.go                 Build-time version/creator constants
internal/config/schema.go               The Config struct (6 fields)
internal/config/config.go               TOML loader + starter template
internal/config/defaults_windows.go     compression default = false
internal/config/defaults_other.go       compression default = true
internal/logger/logger.go               Writes logs/lifeboat.log + stderr
internal/backup/backup.go               All three operations live here
configs/lifeboat.example.toml           Reference config for users
```

Two direct dependencies, zero indirect ones:

- `github.com/BurntSushi/toml` - parses `lifeboat.toml`
- `github.com/klauspost/compress/zstd` - used only when `compression = true`

Total: ~850 lines of Go across 8 files.

## The only data model

`Config` (`internal/config/schema.go`) - flat, six fields:

```go
type Config struct {
    Name          string   `toml:"name"`
    WebappsPath   string   `toml:"webapps_path"`
    BackupPath    string   `toml:"backup_path"`
    Compression   bool     `toml:"compression"`
    RetentionDays int      `toml:"retention_days"`
    ExtraFolders  []string `toml:"extra_folders"`
}
```

There is no database, no `index.json`, no `metadata.json`. Backup state is
**the filesystem itself**: if the folder `backup_path/20260421/2126/` exists,
a backup was taken on that date at that time. That's the whole source of
truth.

## How a backup works (the whole flow in one page)

File: `internal/backup/backup.go`

1. `ListWebapps(cfg)` - reads `webapps_path`, returns `[]Item{Name, Path, Size, IsDir}` sorted by name.
2. User picks indexes (`"1,3"` or blank for all) via `ParseSelection`.
3. `Run(cfg, items, progress)`:
   - Creates `cfg.BackupPath/YYYYMMDD/HHMM/`.
   - For each item + each `ExtraFolders` entry, calls `copyOne(src, name, dest, compress)`:
     - If `compress == false`: plain `copyDir` / `copyFile`.
     - If `compress == true`: `writeTarZst(src, dest/<name>.tar.zst)` - streaming `tar.NewWriter` wrapped in `zstd.NewWriter`.
   - Logs every step to `logs/lifeboat.log` via `logger.Info`.
4. Returns destination path and total bytes copied.

## How history/cleanup works

- `History(cfg)` - walks `BackupPath` for folders matching `YYYYMMDD/HHMM`, parses the timestamp, returns entries newest first. No index file is read.
- `Cleanup(cfg, dryRun)` - calls `History`, filters entries older than `RetentionDays`, either returns them (dry run) or `os.RemoveAll`s each and removes the empty parent date folder. Returns what was (or would be) deleted and bytes freed.

Both functions recognise an entry only if the folder name strictly matches
the `20060102` / `1504` Go time format. Anything else in `BackupPath` (like
`logs/`, `lifeboat.toml`) is ignored.

## Logging

`internal/logger/logger.go`. Call `logger.Init(backupDir)` once at startup and
`defer logger.Close()`. Functions: `Info(fmt, args...)` writes to file only;
`Error(fmt, args...)` writes to file **and** stderr. Log lines look like:

```
2026-04-21 21:25:36 [INFO] backup done dest=/path/to/20260421/2125 size=31 B
```

No rotation, no levels beyond INFO/ERROR. If the log file can't be opened the
program warns and keeps running (stderr-only).

## The menu loop

`cmd/lifeboat/main.go`. Structure:

```
main()
  ├─ if argv[1] == "init" → writeInitTemplate(); return
  ├─ config.Load("")
  ├─ logger.Init(cfg.BackupPath)
  └─ for { printHeader; printMenu; switch readLine() {
        "1" → runNewBackup
        "2" → runHistory
        "3" → runCleanup
        "4" → return
     }}
```

Input is always `bufio.Reader.ReadString('\n')` - trivially scriptable from
cron/Task Scheduler by piping `echo 1 & echo. & echo.`.

There is no Cobra, no Bubble Tea, no build tags. One binary per OS.

## Platform defaults

`internal/config/defaults_{windows,other}.go` - the only OS-specific code.
Sets the default value of `compression` written by `lifeboat init`
(`false` on Windows, `true` elsewhere). Once written, the TOML is the
authority; the default isn't consulted again.

## Where to make common changes

| Change | File(s) to edit |
|---|---|
| Add a new menu option | `cmd/lifeboat/main.go` - `printMenu`, `main`'s switch, a new `runXxx()` |
| Add a new config field | `internal/config/schema.go` (struct), `internal/config/config.go` (`Example` template), `internal/backup/backup.go` (use it) |
| Change backup layout | `internal/backup/backup.go` - `Run()`'s `dest := filepath.Join(...)` |
| Change log format | `internal/logger/logger.go` - `write()` |
| Support a new archive format | `internal/backup/backup.go` - add a branch in `copyOne()`, mirror the tar.zst flow |

## Design principles (non-negotiable)

1. **Simplicity before features.** This project deleted ~5,400 lines of Go and
   7 MB of dependencies to get here. Do not reintroduce Cobra, Bubble Tea,
   checkpoints, index files, or multi-format archive abstractions without
   direct instruction from the owner.
2. **Filesystem is the database.** No hidden state files. Anything a user
   sees in the menu is derived from walking `backup_path`.
3. **One binary per OS.** No build tags beyond the tiny `compression default`
   split. No "legacy" and "modern" variants.
4. **Menu only.** There are no CLI subcommands exposed to users. The only
   non-menu mode is `lifeboat init` which is an implementation convenience,
   not a user-facing CLI.
5. **Logs are append-only and human-readable.** No JSON logs, no structured
   logging library.

## What is intentionally absent

Do not add these without explicit request:

- Restore command - backups are folder copies; users restore manually if needed.
- Checkpoints / never-delete flag - use `retention_days = 0` or move backups out.
- Per-backup metadata files - time is in the folder name; size is on disk.
- Encryption / remote upload - out of scope; pair with `rclone`/`rsync` externally.
- Progress bars, colors, TUIs - plain text menu is the design.

## Testing the tool end-to-end (no unit tests exist yet)

Quick smoke test:

```bash
SANDBOX=/tmp/lifeboat-smoke
rm -rf "$SANDBOX" && mkdir -p "$SANDBOX/webapps/App1" "$SANDBOX/webapps/App2" "$SANDBOX/conf" "$SANDBOX/backup"
echo hi > "$SANDBOX/webapps/App1/index.html"
echo hi > "$SANDBOX/webapps/App2/index.html"
echo srv > "$SANDBOX/conf/server.xml"
go build -o "$SANDBOX/backup/lifeboat" ./cmd/lifeboat
cat > "$SANDBOX/backup/lifeboat.toml" <<EOF
name = "smoke"
webapps_path = "$SANDBOX/webapps"
backup_path = "."
compression = false
retention_days = 30
extra_folders = ["$SANDBOX/conf"]
EOF
cd "$SANDBOX/backup"
printf '1\n\n\n2\n\n4\n' | ./lifeboat   # backup all, show history, exit
```

Check: `find 20*` shows copied tree; `logs/lifeboat.log` shows each step.

## History of this codebase (so you know what not to undo)

This is version **0.3.0**. Prior versions (0.1.x, 0.2.x) contained: Cobra CLI
with 8 subcommands, a Bubble Tea TUI, a separate legacy text UI, a dual-build
system (Go 1.20 legacy + Go 1.24 modern), external 7-Zip integration, tar.gz
compressor, checkpoint system, index.json, metadata.json, storage abstraction
layer, YAML config, `custom_folders` with titles and required/optional flags,
and validator warnings. **All of that was intentionally removed** in the 0.3.0
simplification pass. The git log shows the full before/after.

The commit that collapses everything is labelled in the message as the
"simplify v0.3.0" pass. If you are reading the code and think "this seems
too minimal, surely I should add X back", read that commit first.
