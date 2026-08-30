package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const logDirName = "Lumivue"
const logFileName = "lumivue.log"

// logPath returns the path to the application log file,
// creating the containing directory if necessary.
func logPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(home, "Library", "Logs", logDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}
	return filepath.Join(dir, logFileName), nil
}

// initLogger opens (or creates) the log file and installs a slog handler
// that writes JSON records to it.  The returned io.Closer must be closed
// when the application exits.  Errors opening the log file are non-fatal;
// the application continues with stderr-only logging.
func initLogger() io.Closer {
	path, err := logPath()
	if err != nil {
		slog.Warn("could not resolve log path", "err", err)
		return io.NopCloser(nil)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("could not open log file", "path", path, "err", err)
		return io.NopCloser(nil)
	}

	// Write JSON logs to the file; keep default text logs on stderr.
	multi := io.MultiWriter(os.Stderr, f)
	slog.SetDefault(slog.New(slog.NewJSONHandler(multi, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("log file opened", "path", path)
	return f
}
