# daylog Usage

## Tasks

Create a task without a due date:

```sh
daylog add "document todos"
```

Add metadata and importance:

```sh
daylog add "call the dentist" --project personal --tag health --important
```

List active tasks:

```sh
daylog task list
daylog task list --json
```

Importance can be changed after creation with `task important`, `task unimportant`, or `task toggle-important`.

## Journal

Add an independently stored journal bullet:

```sh
daylog journal add "Finished the proposal" --project work --tag writing
```

Journal entries can be associated with tasks through task IDs as the linking workflow is completed.

## Data

The default local data directory is `~/.daylog`. Set `DAYLOG_DATA_DIR` to use another directory. The database is SQLite and is initialized automatically.

The TUI, Today review, reminders, search, backups, import/export, and full metadata management are implemented in later plan phases.

## Search

```sh
daylog search report
daylog search --project work --tag focus --status open --date today
```

Use `--all` to include soft-deleted records. Task `--date` filters due dates; journal entries use their displayed local entry date.

## Reminders And Diagnostics

```sh
daylog config set reminder-time 18:30
daylog remind install
daylog remind status
daylog remind uninstall
daylog doctor
```

## Import And Backup

```sh
daylog data export --json > daylog.json
daylog data import daylog.json
daylog data import daylog.json --apply --prefer-import
daylog data backup
```

Import is dry-run by default. Applying an import creates a mandatory pre-import backup.
