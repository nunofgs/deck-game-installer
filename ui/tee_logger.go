package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TeeLogger wraps a Logger and mirrors every call to stdout and a log file.
// The log file is truncated (cleared) on creation so it only contains the last run.
type TeeLogger struct {
	inner Logger
	file  *os.File
}

// LogPath returns the path used for the log file.
func LogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "deck-game-installer", "last-run.log")
}

// NewTeeLogger creates a TeeLogger wrapping inner, writing to stdout and the log file.
// The log file is truncated on each call so it only ever holds the last run.
func NewTeeLogger(inner Logger) *TeeLogger {
	path := LogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Printf("[TeeLogger] WARNING: could not create log directory: %v\n", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Printf("[TeeLogger] WARNING: could not open log file %s: %v\n", path, err)
	}

	t := &TeeLogger{inner: inner, file: f}
	t.writeLine(fmt.Sprintf("=== deck-game-installer started at %s ===", time.Now().Format(time.RFC3339)))
	t.writeLine(fmt.Sprintf("Log file: %s", path))
	return t
}

func (t *TeeLogger) writeLine(msg string) {
	line := msg + "\n"
	fmt.Print(line)
	if t.file != nil {
		t.file.WriteString(line) //nolint:errcheck
	}
}

func (t *TeeLogger) Log(msg string) {
	t.writeLine(msg)
	t.inner.Log(msg)
}

func (t *TeeLogger) StepStarted(name string) {
	t.writeLine(fmt.Sprintf("[STEP STARTED] %s", name))
	t.inner.StepStarted(name)
}

func (t *TeeLogger) StepCompleted(name string, err error) {
	if err != nil {
		t.writeLine(fmt.Sprintf("[STEP FAILED]  %s — %v", name, err))
	} else {
		t.writeLine(fmt.Sprintf("[STEP DONE]    %s", name))
	}
	t.inner.StepCompleted(name, err)
}

func (t *TeeLogger) ConfigureSteps(stepNames []string) {
	t.writeLine(fmt.Sprintf("[STEPS] %v", stepNames))
	t.inner.ConfigureSteps(stepNames)
}

func (t *TeeLogger) Confirm(title, message string) bool {
	t.writeLine(fmt.Sprintf("[CONFIRM] %s — %s", title, message))
	result := t.inner.Confirm(title, message)
	t.writeLine(fmt.Sprintf("[CONFIRM RESULT] %v", result))
	return result
}

func (t *TeeLogger) Select(title, prompt string, options []string) (string, bool) {
	t.writeLine(fmt.Sprintf("[SELECT] %s — %s — options: %v", title, prompt, options))
	result, ok := t.inner.Select(title, prompt, options)
	t.writeLine(fmt.Sprintf("[SELECT RESULT] %q ok=%v", result, ok))
	return result, ok
}

func (t *TeeLogger) WaitWithManualOverride() <-chan struct{} {
	t.writeLine("[WAIT] Showing manual override button...")
	return t.inner.WaitWithManualOverride()
}
