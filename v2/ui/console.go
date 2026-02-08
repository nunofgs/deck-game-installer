package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ConsoleLogger implements Logger for command-line usage.
type ConsoleLogger struct {
	currentStep string
	steps       []string
	stepStart   time.Time
}

// NewConsoleLogger creates a new console-based logger.
func NewConsoleLogger() *ConsoleLogger {
	return &ConsoleLogger{}
}

// Log prints a message to stdout.
func (c *ConsoleLogger) Log(message string) {
	fmt.Println(message)
}

// SetStep updates the current step display.
func (c *ConsoleLogger) SetStep(name string) {
	c.currentStep = name
	fmt.Printf("\n=== %s ===\n", name)
}

// StepStarted marks a step as started.
func (c *ConsoleLogger) StepStarted(name string) {
	c.currentStep = name
	c.stepStart = time.Now()
	fmt.Printf("\n=== %s ===\n", name)
}

// StepCompleted marks a step as completed or failed.
func (c *ConsoleLogger) StepCompleted(name string, err error) {
	duration := time.Since(c.stepStart)
	if err != nil {
		fmt.Printf("✗ %s failed (%.1fs): %v\n", name, duration.Seconds(), err)
	} else {
		fmt.Printf("✓ %s completed (%.1fs)\n", name, duration.Seconds())
	}
}

// ConfigureSteps stores the list of steps for display.
func (c *ConsoleLogger) ConfigureSteps(stepNames []string) {
	c.steps = stepNames
	fmt.Println("Installation steps:")
	for i, step := range stepNames {
		fmt.Printf("  %d. %s\n", i+1, step)
	}
	fmt.Println()
}

// Confirm shows a yes/no prompt and returns the user's choice.
func (c *ConsoleLogger) Confirm(title, message string) bool {
	fmt.Printf("\n%s\n", title)
	fmt.Println(strings.Repeat("-", len(title)))
	fmt.Println(message)
	fmt.Print("\nContinue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}

// Select shows a selection prompt and returns the user's choice.
func (c *ConsoleLogger) Select(title, prompt string, options []string) (string, bool) {
	fmt.Printf("\n%s\n", title)
	fmt.Println(strings.Repeat("-", len(title)))
	fmt.Println(prompt)
	fmt.Println()

	for i, opt := range options {
		fmt.Printf("  %d. %s\n", i+1, opt)
	}

	fmt.Print("\nEnter number (or 'q' to cancel): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "q" || input == "" {
		return "", false
	}

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(options) {
		fmt.Println("Invalid selection")
		return "", false
	}

	return options[num-1], true
}

// Error displays an error message.
func (c *ConsoleLogger) Error(title, message string) {
	fmt.Printf("\n❌ %s\n", title)
	fmt.Println(strings.Repeat("-", len(title)+3))
	fmt.Println(message)
	fmt.Println()
}

// Info displays an informational message.
func (c *ConsoleLogger) Info(title, message string) {
	fmt.Printf("\nℹ️  %s\n", title)
	fmt.Println(strings.Repeat("-", len(title)+4))
	fmt.Println(message)
	fmt.Println()
}

// RollbackPrompt asks the user if they want to roll back changes.
func (c *ConsoleLogger) RollbackPrompt(errorMsg string, completedOps []string) bool {
	fmt.Println("\n⚠️  Installation Failed")
	fmt.Println(strings.Repeat("=", 25))
	fmt.Printf("\nError: %s\n", errorMsg)
	fmt.Println("\nThe following changes were made:")
	for _, op := range completedOps {
		fmt.Printf("  • %s\n", op)
	}
	fmt.Print("\nWould you like to undo these changes? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	// Default to yes
	return input == "" || input == "y" || input == "yes"
}

// Wait blocks until the user presses Enter.
func (c *ConsoleLogger) Wait() {
	fmt.Print("\nPress Enter to continue...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')
}

// WaitWithMessage shows a message and waits for user confirmation.
func (c *ConsoleLogger) WaitWithMessage(message string) bool {
	fmt.Println(message)
	fmt.Print("\nPress Enter to continue, or 'q' to cancel: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input != "q"
}
