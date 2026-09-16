package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskOpen     TaskStatus = "open"
	TaskDone     TaskStatus = "done"
	TaskArchived TaskStatus = "archived"
)

type Task struct {
	ID          uuid.UUID
	Description string
	Project     string
	Tags        []string
	DueDate     *time.Time
	Important   bool
	Status      TaskStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
	DeletedAt   *time.Time
}

type JournalEntry struct {
	ID          uuid.UUID
	Description string
	Project     string
	Tags        []string
	EntryDate   *time.Time
	TaskIDs     []uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

var ErrEmptyDescription = errors.New("description must not be empty")

func NewTask(description string, now time.Time) (Task, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return Task{}, ErrEmptyDescription
	}
	now = now.UTC().Truncate(time.Microsecond)
	return Task{ID: uuid.Must(uuid.NewV7()), Description: description, Status: TaskOpen, CreatedAt: now, UpdatedAt: now}, nil
}

func NewJournalEntry(description string, now time.Time) (JournalEntry, error) {
	description = strings.TrimSpace(description)
	if description == "" {
		return JournalEntry{}, ErrEmptyDescription
	}
	now = now.UTC().Truncate(time.Microsecond)
	return JournalEntry{ID: uuid.Must(uuid.NewV7()), Description: description, CreatedAt: now, UpdatedAt: now}, nil
}

func (t *Task) Validate() error {
	if strings.TrimSpace(t.Description) == "" {
		return ErrEmptyDescription
	}
	if t.Status != TaskOpen && t.Status != TaskDone && t.Status != TaskArchived {
		return fmt.Errorf("invalid task status %q", t.Status)
	}
	return nil
}

func (j *JournalEntry) Validate() error {
	if strings.TrimSpace(j.Description) == "" {
		return ErrEmptyDescription
	}
	return nil
}

func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("name must not be empty")
	}
	return strings.ToLower(name), nil
}

func ParseDate(value string, now time.Time) (time.Time, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	localNow := now.In(time.Local)
	base := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, time.Local)
	switch value {
	case "today":
		return base, nil
	case "tomorrow":
		return base.AddDate(0, 0, 1), nil
	}
	if strings.HasPrefix(value, "+") && strings.HasSuffix(value, "d") {
		var days int
		if _, err := fmt.Sscanf(value, "+%dd", &days); err == nil && days >= 0 {
			return base.AddDate(0, 0, days), nil
		}
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: use today, tomorrow, +Nd, or YYYY-MM-DD", value)
	}
	return parsed, nil
}
