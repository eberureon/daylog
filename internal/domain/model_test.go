package domain

import (
	"testing"
	"time"
)

func TestNewTaskDefaults(t *testing.T) {
	now := time.Date(2026, time.September, 16, 12, 34, 56, 123456789, time.FixedZone("test", 3600))
	task, err := NewTask("  write plan  ", now)
	if err != nil {
		t.Fatal(err)
	}
	if task.Description != "write plan" || task.Status != TaskOpen || task.Important {
		t.Fatalf("unexpected task defaults: %+v", task)
	}
	if task.CreatedAt.Location() != time.UTC || task.CreatedAt.Nanosecond()%1000 != 0 {
		t.Fatalf("timestamp was not normalized: %v", task.CreatedAt)
	}
}

func TestParseDateShortcuts(t *testing.T) {
	now := time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)
	for input, expected := range map[string]string{"today": "2026-09-16", "tomorrow": "2026-09-17", "+7d": "2026-09-23", "2026-09-20": "2026-09-20"} {
		got, err := ParseDate(input, now)
		if err != nil {
			t.Fatalf("ParseDate(%q): %v", input, err)
		}
		if got.Format("2006-01-02") != expected {
			t.Errorf("ParseDate(%q) = %s, want %s", input, got.Format("2006-01-02"), expected)
		}
	}
}

func TestParseDateRejectsInvalidValue(t *testing.T) {
	if _, err := ParseDate("next-week", time.Now()); err == nil {
		t.Fatal("expected invalid date error")
	}
}
