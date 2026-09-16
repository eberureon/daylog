# daylog Context

This document records the agreed product and engineering decisions for future work.

## Product

- `daylog` is a local-first personal task tracker and daily journal.
- Tasks and journal entries are separate record types.
- Each journal bullet is independently stored and may link to multiple tasks.
- Version one is single-user, offline, and has no accounts, sync, telemetry, or network services.
- The Go module is `github.com/eberureon/daylog`.

## Tasks

- Statuses are `open`, `done`, and `archived`.
- Deletion is soft deletion; deleted records are recoverable through Trash.
- Tasks have an optional date-only due date.
- Due-date shortcuts include `today`, `tomorrow`, `+7d`, and `YYYY-MM-DD`.
- Tasks have a boolean `important` flag, defaulting to false.
- Importance persists across completion, archive, reopen, and restore.
- Creation accepts `--important`; the TUI uses `p` to toggle it.
- Important tasks use a `!` marker and supplementary color.
- Important tasks without due dates appear in Today.

## Journaling

- Journal entries preserve their original `created_at` timestamp.
- An optional `entry_date` supports backfilling an entry to another local day.
- Entries group by `entry_date` when present, otherwise by local creation date.
- Entries sort by original creation time.
- Task completion does not automatically create a journal entry.

## Metadata

- A record has at most one project and any number of tags.
- Projects and tags are shared across tasks and journal entries.
- Names match case-insensitively while preserving display casing.
- Metadata is auto-created when first referenced and can be explicitly archived/restored.
- Existing associations survive metadata archiving.

## Time And IDs

- Domain entities use UUIDv7.
- Timestamps are stored as UTC with microsecond precision.
- Display and calendar grouping use the system local timezone.
- There is no custom timezone setting in v1.

## Technology

- Go 1.24+
- Bubble Tea, Bubbles, and Lip Gloss for the TUI
- Cobra for the CLI
- `modernc.org/sqlite` through `database/sql`
- TOML preferences separate from SQLite domain data
- Versioned transactional migrations
- Layered domain, application, storage, CLI, TUI, and platform packages

## CLI And TUI

- Bare `daylog` launches the TUI in a TTY and remains usable with plain output outside a TTY.
- Vim-style movement has arrow-key fallbacks.
- Core actions: `a` add, `e` edit, `d` delete, `x` complete, `p` toggle importance, `/` search, `?` help, `q` quit.
- TUI forms preserve entered values and show validation errors inline.
- CLI output is human-readable by default and supports `--json`.

## Today And Reminders

- Today shows due-today tasks, overdue tasks, all open important tasks, today's journal entries, and collapsed completed tasks.
- Review completion is explicit through `review done`.
- Reminders run daily at local `17:00` by default, are configurable, and do not catch up after missed schedules.
- macOS uses `launchd` and `osascript`; Linux uses user systemd where available, cron otherwise, and `notify-send` for notifications.

## Data Safety And Documentation

- Lossless JSON export includes deleted records and internal metadata.
- Backups are timestamped, consistent SQLite snapshots with no automatic retention deletion.
- Imports default to dry-run, require `--apply`, create a safety backup, and abort on merge conflicts unless a preference is explicit.
- Documentation includes `README.md`, `docs/usage.md`, and one comprehensive `docs/daylog.1` man page, plus CLI and in-app help.

## V1 Exclusions

Recurring tasks, calendar integration, cloud sync, collaboration, attachments, Markdown, exact deadlines, mobile/web clients, automatic task journaling, bulk actions, generic undo, full audit history, database encryption, telemetry, and catch-up reminders.
