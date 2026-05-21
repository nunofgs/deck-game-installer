// Package ui provides interfaces for user interaction during installation.
package ui

// ConfirmAction represents the user's choice in a retry/wait dialog.
type ConfirmAction int

const (
	ActionCancel      ConfirmAction = iota // User cancelled; leave shortcut in place.
	ActionScanAgain                        // User wants to scan for executables immediately.
	ActionKeepWaiting                      // User wants to wait longer before scanning.
)

// Logger is the interface for user interaction during the installation workflow.
// This abstraction allows the workflow to work with different UI implementations
// (GUI, CLI, or testing mocks).
type Logger interface {
	// Log writes a message to the log output.
	Log(message string)

	// StepStarted marks a step as started and records the start time.
	StepStarted(name string)

	// StepCompleted marks a step as completed (or failed if err != nil).
	StepCompleted(name string, err error)

	// ConfigureSteps sets up the list of steps to show in the progress UI.
	ConfigureSteps(stepNames []string)

	// Confirm shows a yes/no confirmation dialog.
	// Returns true if the user clicked "Yes".
	Confirm(title, message string) bool

	// Select shows a selection dialog with multiple options.
	// Returns the selected option and true if confirmed, or empty string and false if cancelled.
	Select(title, prompt string, options []string) (string, bool)

	// WaitWithManualOverride shows a button the user can click to proceed manually.
	// Used while waiting for the installer to complete; returns a channel that fires on click.
	WaitWithManualOverride() <-chan struct{}

	// ConfirmRetryOrWait shows a dialog with "Scan Again", "Keep Waiting", and "Cancel" buttons.
	// Used when no game executables are found after installation.
	ConfirmRetryOrWait(title, message string) ConfirmAction

	// PromptText shows a text-input dialog.
	// Returns the entered text and true if confirmed, or empty string and false if cancelled.
	PromptText(title, message, defaultValue string) (string, bool)
}
