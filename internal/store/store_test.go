package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/eberureon/daylog/internal/app"
	"github.com/eberureon/daylog/internal/domain"
	"github.com/eberureon/daylog/internal/store"
	"github.com/google/uuid"
)

func TestTaskAndJournalPersistence(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "daylog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := app.New(s)
	task, err := a.AddTask(context.Background(), "cook dinner", "Home", []string{"Evening"}, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := a.AddJournalEntry(context.Background(), "Dinner is done", "home", []string{"evening"}, nil, []uuid.UUID{task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if entries.Description != "Dinner is done" {
		t.Fatalf("unexpected entry: %+v", entries)
	}
	tasks, err := a.ListTasks(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || !tasks[0].Important || len(tasks[0].Tags) != 1 || tasks[0].Tags[0] != "Evening" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	journal, err := a.ListJournalEntries(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(journal) != 1 || len(journal[0].TaskIDs) != 1 || journal[0].TaskIDs[0] != task.ID {
		t.Fatalf("unexpected journal: %+v", journal)
	}
}

func TestTaskLifecycleAndSoftDelete(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "daylog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := app.New(s)
	task, err := a.AddTask(context.Background(), "review", "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteTask(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteTask(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	visible, err := a.ListTasks(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(visible) != 0 {
		t.Fatalf("deleted task visible: %+v", visible)
	}
	all, err := a.ListTasks(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Status != domain.TaskDone || all[0].CompletedAt == nil || all[0].DeletedAt == nil {
		t.Fatalf("unexpected deleted task: %+v", all)
	}
	if err := a.RestoreTask(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.ReopenTask(context.Background(), task.ID); err != nil {
		t.Fatal(err)
	}
	all, err = a.ListTasks(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Status != domain.TaskOpen || all[0].CompletedAt != nil {
		t.Fatalf("unexpected restored task: %+v", all)
	}
}
