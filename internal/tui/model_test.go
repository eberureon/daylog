package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/eberureon/daylog/internal/domain"
)

func TestJournalItemsGroupsByDisplayedDate(t *testing.T) {
	dayOne := time.Date(2026, 9, 16, 10, 0, 0, 0, time.Local)
	dayTwo := time.Date(2026, 9, 15, 10, 0, 0, 0, time.Local)
	entries := []domain.JournalEntry{
		{Description: "New Title", CreatedAt: dayOne},
		{Description: "Done Yesterday", CreatedAt: dayTwo},
	}
	items := journalItems(entries)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if got := items[0].(item).dateHeading; got != "2026-09-16" {
		t.Fatalf("first heading = %q", got)
	}
	if got := items[0].(item).journal.Description; got != "New Title" {
		t.Fatalf("first entry = %q", got)
	}
	if got := items[1].(item).dateHeading; got != "2026-09-15" {
		t.Fatalf("second heading = %q", got)
	}
	if got := items[1].(item).journal.Description; got != "Done Yesterday" {
		t.Fatalf("second entry = %q", got)
	}
}

func TestFilterInputReceivesCommandCharacters(t *testing.T) {
	delegate := list.NewDefaultDelegate()
	model := Model{list: list.New([]list.Item{item{task: &domain.Task{Description: "edit this"}}}, delegate, 80, 20)}

	var updated tea.Model
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updated.(Model)
	if !model.list.SettingFilter() {
		t.Fatal("expected list to enter filter mode")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model = updated.(Model)
	if got := model.list.FilterValue(); got != "e" {
		t.Fatalf("filter value = %q, want %q", got, "e")
	}
}

func TestTodayTaskItemsCreatesSections(t *testing.T) {
	today := time.Now().In(time.Local)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	overdue := todayDate.AddDate(0, 0, -1)
	completed := today.UTC()
	items := todayTaskItems([]domain.Task{
		{Description: "late", Status: domain.TaskOpen, DueDate: &overdue},
		{Description: "now", Status: domain.TaskOpen, DueDate: &todayDate},
		{Description: "important", Status: domain.TaskOpen, Important: true},
		{Description: "done", Status: domain.TaskDone, CompletedAt: &completed},
	})
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4", len(items))
	}
	for i, want := range []string{"OVERDUE", "DUE TODAY", "IMPORTANT", "COMPLETED TODAY"} {
		if got := items[i].(item).dateHeading; got != want {
			t.Errorf("section %d = %q, want %q", i, got, want)
		}
	}
}
