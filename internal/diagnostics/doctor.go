package diagnostics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eberureon/daylog/internal/config"
	"github.com/eberureon/daylog/internal/store"
)

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func Run(ctx context.Context) []Check {
	checks := []Check{}
	dir, err := config.DataDir()
	checks = append(checks, Check{"data directory", err == nil, dir})
	if err != nil {
		return checks
	}
	info, err := os.Stat(dir)
	checks = append(checks, Check{"data directory writable", err == nil && info.IsDir(), dir})
	path, err := config.DatabasePath()
	if err != nil {
		checks = append(checks, Check{"database path", false, err.Error()})
		return checks
	}
	s, err := store.Open(ctx, path)
	if err != nil {
		checks = append(checks, Check{"database", false, err.Error()})
		return checks
	}
	defer s.Close()
	checks = append(checks, Check{"database", true, filepath.Base(path)})
	var integrity string
	if err := s.DB().QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		checks = append(checks, Check{"database integrity", false, err.Error()})
	} else {
		checks = append(checks, Check{"database integrity", integrity == "ok", integrity})
	}
	return checks
}

func Summary(checks []Check) error {
	for _, check := range checks {
		mark := "OK"
		if !check.OK {
			mark = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", mark, check.Name, check.Detail)
	}
	for _, check := range checks {
		if !check.OK {
			return fmt.Errorf("diagnostic check failed: %s", check.Name)
		}
	}
	return nil
}
