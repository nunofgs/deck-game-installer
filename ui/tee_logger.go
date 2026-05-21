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
	t.writeLine(fmt.Sprintf("log: %s", path))
	return t
}

func (t *TeeLogger) ts() string {
	return time.Now().Format("15:04:05")
}

func (t *TeeLogger) writeLine(msg string) {
	line := t.ts() + "  " + msg + "\n"
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
	t.writeLine(fmt.Sprintf("[step:start] %s", name))
	t.inner.StepStarted(name)
}

func (t *TeeLogger) StepCompleted(name string, err error) {
	if err != nil {
		t.writeLine(fmt.Sprintf("[step:fail]  %s — %v", name, err))
	} else {
		t.writeLine(fmt.Sprintf("[step:done]  %s", name))
	}
	t.inner.StepCompleted(name, err)
}

func (t *TeeLogger) ConfigureSteps(stepNames []string) {
	t.writeLine(fmt.Sprintf("[steps] %v", stepNames))
	t.inner.ConfigureSteps(stepNames)
}

func (t *TeeLogger) Confirm(title, message string) bool {
	t.writeLine(fmt.Sprintf("[confirm] %s — %s", title, message))
	result := t.inner.Confirm(title, message)
	t.writeLine(fmt.Sprintf("[confirm:result] %v", result))
	return result
}

func (t *TeeLogger) Select(title, prompt string, options []string) (string, bool) {
	t.writeLine(fmt.Sprintf("[select] %s — %s — options: %v", title, prompt, options))
	result, ok := t.inner.Select(title, prompt, options)
	t.writeLine(fmt.Sprintf("[select:result] %q ok=%v", result, ok))
	return result, ok
}

func (t *TeeLogger) WaitWithManualOverride() <-chan struct{} {
	t.writeLine("[wait] showing manual override button")
	return t.inner.WaitWithManualOverride()
}

func (t *TeeLogger) EnableManualOverride() {
	t.writeLine("[wait] manual override enabled")
	t.inner.EnableManualOverride()
}

func (t *TeeLogger) ConfirmRetryOrWait(title, message string) ConfirmAction {
	t.writeLine(fmt.Sprintf("[confirm-retry] %s — %s", title, message))
	result := t.inner.ConfirmRetryOrWait(title, message)
	t.writeLine(fmt.Sprintf("[confirm-retry:result] %v", result))
	return result
}

func (t *TeeLogger) PromptText(title, message, defaultValue string) (string, bool) {
	t.writeLine(fmt.Sprintf("[prompt] %s — %s (default: %q)", title, message, defaultValue))
	text, ok := t.inner.PromptText(title, message, defaultValue)
	t.writeLine(fmt.Sprintf("[prompt:result] %q ok=%v", text, ok))
	return text, ok
}
