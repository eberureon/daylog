package reminder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/eberureon/daylog/internal/config"
)

func Run() error {
	title, body := "daylog review", "Review today's tasks and journal"
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("osascript", "-e", fmt.Sprintf(`display notification %q with title %q`, body, title)).Run()
	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			return fmt.Errorf("notify-send is not installed")
		}
		return exec.Command("notify-send", title, body).Run()
	default:
		return fmt.Errorf("notifications unsupported on %s", runtime.GOOS)
	}
}

func Install() error {
	if _, err := config.Load(); err != nil {
		return err
	}
	dir, err := config.DataDir()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c, err := config.Load()
	if err != nil {
		return err
	}
	hour, minute, err := parseTime(c.ReminderTime)
	if err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		path := filepath.Join(dir, "launchd.plist")
		content := fmt.Sprintf("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict><key>Label</key><string>com.eberureon.daylog</string><key>ProgramArguments</key><array><string>%s</string><string>remind</string><string>run</string></array><key>StartCalendarInterval</key><dict><key>Hour</key><integer>%d</integer><key>Minute</key><integer>%d</integer></dict></dict></plist>\n", exe, hour, minute)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		return nil
	}
	if runtime.GOOS == "linux" {
		path := filepath.Join(dir, "daylog.cron")
		content := fmt.Sprintf("%d %d * * * %s remind run\n", minute, hour, exe)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("scheduler unsupported on %s", runtime.GOOS)
}

func parseTime(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid reminder time %q", value)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid reminder hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid reminder minute")
	}
	return hour, minute, nil
}

func Status(ctx context.Context, jsonOutput bool) error {
	if _, err := config.Load(); err != nil {
		return err
	}
	dir, err := config.DataDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "daylog.cron")
	if runtime.GOOS == "darwin" {
		path = filepath.Join(dir, "launchd.plist")
	}
	_, statErr := os.Stat(path)
	installed := statErr == nil
	if !os.IsNotExist(statErr) && statErr != nil {
		return statErr
	}
	if jsonOutput {
		fmt.Printf("{\"platform\":%q,\"installed\":%t,\"path\":%q}\n", runtime.GOOS, installed, path)
	} else {
		fmt.Printf("reminder platform: %s\ninstalled: %t\nconfiguration: %s\n", runtime.GOOS, installed, path)
	}
	_ = ctx
	return nil
}

func Uninstall() error {
	dir, err := config.DataDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "daylog.cron")
	if runtime.GOOS == "darwin" {
		path = filepath.Join(dir, "launchd.plist")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
