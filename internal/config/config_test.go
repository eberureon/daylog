package config

import (
	"os"
	"testing"
)

func TestLoadAndSave(t *testing.T) {
	t.Setenv("DAYLOG_DATA_DIR", t.TempDir())
	want := Config{ReminderTime: "18:30", NoColor: true}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if _, err := os.Stat("config.toml"); err == nil {
		t.Fatal("unexpected config in current directory")
	}
}
