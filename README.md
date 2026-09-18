# daylog

`daylog` is a local-first task tracker and daily journal for the terminal.

The project is under active development. The current implementation plan and agreed decisions are documented in:

- [`CONTEXT.md`](CONTEXT.md)

The intended workflow is:

```text
daylog add "cook dinner" --project home --tag evening
daylog journal add "Prepared dinner" --task <task-id>
daylog
```

The application will use a local SQLite database, a keyboard-driven terminal UI, and native end-of-day reminders on macOS and Linux.

## Useful Commands

```sh
daylog task add "write report" --project work --tag focus --important --due today
daylog task list
daylog journal add "Finished the report" --date today
daylog search report --project work --tag focus --status open --date today
daylog review done
daylog doctor
daylog data export --json > daylog.json
daylog data backup
```

Run `daylog help` for the full command reference. Running `daylog` without a subcommand opens the terminal UI.
