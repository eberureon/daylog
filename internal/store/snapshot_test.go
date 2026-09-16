package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/eberureon/daylog/internal/app"
	"github.com/eberureon/daylog/internal/store"
)

func TestSnapshotIncludesMetadataAndReview(t *testing.T) {
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "daylog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := app.New(s)
	if _, err := a.AddTask(context.Background(), "prepare", "Work", []string{"Focus"}, nil, true); err != nil {
		t.Fatal(err)
	}
	if err := a.MarkReview(context.Background(), "2026-09-17"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.ExportSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Projects) != 1 || len(snapshot.Tags) != 1 || len(snapshot.Reviews) != 1 {
		t.Fatalf("snapshot omitted metadata: %+v", snapshot)
	}
}
