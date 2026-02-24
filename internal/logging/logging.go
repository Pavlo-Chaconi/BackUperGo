package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var logFile *os.File

// Init configures global logging to a per-run file and cleans up old logs.
// It returns the log file path for reference.
func Init(appName string, retentionDays int) (string, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}

	baseDir, err := os.UserConfigDir()
	if err != nil || baseDir == "" {
		baseDir = os.TempDir()
	}

	logDir := filepath.Join(baseDir, appName, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}

	_ = cleanupOldLogs(logDir, retentionDays)

	fileName := fmt.Sprintf("%s_%s.log", strings.ToLower(appName), time.Now().Format("20060102_150405"))
	path := filepath.Join(logDir, fileName)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("open log file: %w", err)
	}

	logFile = f
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	return path, nil
}

// Close closes the current log file if opened.
func Close() {
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

func cleanupOldLogs(dir string, retentionDays int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}
