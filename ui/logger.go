// Package ui provides interfaces for user interaction during installation.
package ui

// Logger is the interface for user interaction during the installation workflow.
// This abstraction allows the workflow to work with different UI implementations
// (GUI, CLI, or testing mocks).
type Logger interface {
	// Log writes a message to the log output.
	Log(message string)

	// SetStep updates the UI to show the current step being executed.
	// Deprecated: Use StepStarted instead for better progress tracking.
	SetStep(name string)

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

	// Error shows an error dialog to the user.
	Error(title, message string)

	// Wait blocks until the user acknowledges (clicks OK).
	Wait()

	// WaitWithMessage shows a message and OK/Cancel buttons.
	// Returns true if OK was clicked, false if cancelled.
	WaitWithMessage(message string) bool

	// WaitWithManualOverride shows a single button for manual override.
	// This is used when waiting for installer to complete - monitoring runs in
	// background and this provides a way for user to continue manually.
	// Returns a channel that will receive when user clicks the button.
	WaitWithManualOverride() <-chan struct{}
}
