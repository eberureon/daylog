package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ReminderTime string
	NoColor      bool
}

func Default() Config { return Config{ReminderTime: "17:00"} }

func DataDir() (string, error) {
	if value := os.Getenv("DAYLOG_DATA_DIR"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".daylog"), nil
}

func DatabasePath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daylog.db"), nil
}

func ConfigPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (Config, error) {
	c := Default()
	path, err := ConfigPath()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(value), "\"")
		switch key {
		case "reminder_time":
			c.ReminderTime = value
		case "no_color":
			c.NoColor, err = strconv.ParseBool(value)
			if err != nil {
				return c, fmt.Errorf("invalid no_color: %w", err)
			}
		}
	}
	if _, err := time.Parse("15:04", c.ReminderTime); err != nil {
		return c, fmt.Errorf("invalid reminder_time %q: use HH:MM", c.ReminderTime)
	}
	return c, nil
}

func Save(c Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data := fmt.Sprintf("reminder_time = %q\nno_color = %t\n", c.ReminderTime, c.NoColor)
	return os.WriteFile(path, []byte(data), 0o600)
}
