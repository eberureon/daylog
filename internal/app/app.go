package app

import (
	"context"
	"time"

	"github.com/eberureon/daylog/internal/domain"
	"github.com/eberureon/daylog/internal/store"
	"github.com/google/uuid"
)

type App struct {
	store *store.Store
	now   func() time.Time
}

func New(s *store.Store) *App { return &App{store: s, now: time.Now} }

func (a *App) AddTask(ctx context.Context, description, project string, tags []string, due *time.Time, important bool) (domain.Task, error) {
	task, err := domain.NewTask(description, a.now())
	if err != nil {
		return domain.Task{}, err
	}
	task.Project, task.Tags, task.DueDate, task.Important = project, tags, due, important
	if err := task.Validate(); err != nil {
		return domain.Task{}, err
	}
	if err := a.store.CreateTask(ctx, task); err != nil {
		return domain.Task{}, err
	}
	return task, nil
}

func (a *App) AddJournalEntry(ctx context.Context, description, project string, tags []string, entryDate *time.Time, taskIDs []uuid.UUID) (domain.JournalEntry, error) {
	entry, err := domain.NewJournalEntry(description, a.now())
	if err != nil {
		return domain.JournalEntry{}, err
	}
	entry.Project, entry.Tags, entry.EntryDate, entry.TaskIDs = project, tags, entryDate, taskIDs
	if err := entry.Validate(); err != nil {
		return domain.JournalEntry{}, err
	}
	if err := a.store.CreateJournalEntry(ctx, entry); err != nil {
		return domain.JournalEntry{}, err
	}
	return entry, nil
}

func (a *App) ListTasks(ctx context.Context, includeDeleted bool) ([]domain.Task, error) {
	return a.store.ListTasks(ctx, includeDeleted)
}
func (a *App) ListJournalEntries(ctx context.Context, includeDeleted bool) ([]domain.JournalEntry, error) {
	return a.store.ListJournalEntries(ctx, includeDeleted)
}
func (a *App) CompleteTask(ctx context.Context, id uuid.UUID) error {
	return a.store.SetTaskStatus(ctx, id, domain.TaskDone)
}
func (a *App) ReopenTask(ctx context.Context, id uuid.UUID) error {
	return a.store.SetTaskStatus(ctx, id, domain.TaskOpen)
}
func (a *App) ArchiveTask(ctx context.Context, id uuid.UUID) error {
	return a.store.SetTaskStatus(ctx, id, domain.TaskArchived)
}
func (a *App) DeleteTask(ctx context.Context, id uuid.UUID) error {
	return a.store.SetTaskDeleted(ctx, id, true)
}
func (a *App) RestoreTask(ctx context.Context, id uuid.UUID) error {
	return a.store.SetTaskDeleted(ctx, id, false)
}
func (a *App) DeleteJournalEntry(ctx context.Context, id uuid.UUID) error {
	return a.store.SetJournalDeleted(ctx, id, true)
}
func (a *App) RestoreJournalEntry(ctx context.Context, id uuid.UUID) error {
	return a.store.SetJournalDeleted(ctx, id, false)
}
func (a *App) ToggleImportant(ctx context.Context, id uuid.UUID) error {
	return a.store.ToggleImportant(ctx, id)
}
func (a *App) SetImportant(ctx context.Context, id uuid.UUID, important bool) error {
	return a.store.SetImportant(ctx, id, important)
}
func (a *App) UpdateTask(ctx context.Context, task domain.Task) error {
	task.UpdatedAt = a.now().UTC()
	return a.store.UpdateTask(ctx, task)
}
func (a *App) UpdateJournalEntry(ctx context.Context, entry domain.JournalEntry) error {
	entry.UpdatedAt = a.now().UTC()
	return a.store.UpdateJournalEntry(ctx, entry)
}
func (a *App) MarkReview(ctx context.Context, date string) error {
	return a.store.MarkReview(ctx, date)
}
func (a *App) IsReviewed(ctx context.Context, date string) (bool, error) {
	return a.store.IsReviewed(ctx, date)
}
func (a *App) Search(ctx context.Context, options store.SearchOptions) ([]domain.Task, []domain.JournalEntry, error) {
	return a.store.Search(ctx, options)
}
