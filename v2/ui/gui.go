package ui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// lightReadableTheme provides better contrast for the GUI.
type lightReadableTheme struct{}

func (t lightReadableTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameForeground {
		return color.NRGBA{R: 0x11, G: 0x11, B: 0x11, A: 0xff}
	}
	if name == theme.ColorNameDisabled {
		return color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff}
	}
	if name == theme.ColorNameInputBackground {
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	}
	return theme.LightTheme().Color(name, variant)
}

func (t lightReadableTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.LightTheme().Font(style)
}

func (t lightReadableTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.LightTheme().Icon(name)
}

func (t lightReadableTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.LightTheme().Size(name)
}

// GUILogger implements Logger with a Fyne-based graphical interface.
type GUILogger struct {
	app              fyne.App
	window           fyne.Window
	titleLabel       *widget.Label
	subtitleLabel    *widget.Label
	stepCircles      []*canvas.Circle
	stepLabels       []*widget.Label
	steps            []string
	currentStepIndex int
	buttonContainer  *fyne.Container
	okBtn             *widget.Button
	cancelBtn         *widget.Button
	quitBtn           *widget.Button
	manualOverrideBtn *widget.Button

	okCh             chan struct{}
	cancelCh         chan struct{}
	manualOverrideCh chan struct{}
	doneCh           chan struct{} // signals when GUI has closed
	closeOnce        sync.Once

	logFile *os.File
}

// NewGUILogger creates a new graphical logger window.
func NewGUILogger(title string) *GUILogger {
	a := app.New()
	a.Settings().SetTheme(lightReadableTheme{})
	w := a.NewWindow(title)
	w.Resize(fyne.NewSize(700, 400))

	// Default steps - will be configured based on install type
	defaultSteps := []string{
		"Initialize",
		"Mount ISO",
		"Add to Steam",
		"Run Installer",
		"Find Game",
		"Cleanup",
		"Finalize",
	}

	titleLabel := widget.NewLabelWithStyle("Installing to Steam", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	subtitleLabel := widget.NewLabelWithStyle("Starting installation process...", fyne.TextAlignCenter, fyne.TextStyle{})

	// Create timeline circles and labels
	var stepCircles []*canvas.Circle
	var stepLabels []*widget.Label
	var stepContainers []fyne.CanvasObject

	inactiveColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}

	for _, step := range defaultSteps {
		circle := canvas.NewCircle(color.White)
		circle.StrokeColor = inactiveColor
		circle.StrokeWidth = 3
		stepCircles = append(stepCircles, circle)

		circleBox := canvas.NewRectangle(color.Transparent)
		circleBox.SetMinSize(fyne.NewSize(20, 20))
		circleContainer := container.NewStack(circleBox, circle)

		label := widget.NewLabel(step)
		label.Alignment = fyne.TextAlignCenter
		stepLabels = append(stepLabels, label)

		stepBox := container.NewVBox(
			container.NewCenter(circleContainer),
			label,
		)
		stepContainers = append(stepContainers, stepBox)
	}

	// Build horizontal row with spacers
	stepsRow := container.NewHBox()
	for i, stepBox := range stepContainers {
		stepsRow.Add(stepBox)
		if i < len(stepContainers)-1 {
			stepsRow.Add(layout.NewSpacer())
		}
	}

	// Background line
	lineColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	backgroundLine := canvas.NewRectangle(lineColor)
	backgroundLine.SetMinSize(fyne.NewSize(1, 3))

	linePadding := canvas.NewRectangle(color.Transparent)
	linePadding.SetMinSize(fyne.NewSize(1, 5))

	leftPad := canvas.NewRectangle(color.Transparent)
	leftPad.SetMinSize(fyne.NewSize(30, 3))
	rightPad := canvas.NewRectangle(color.Transparent)
	rightPad.SetMinSize(fyne.NewSize(30, 3))

	lineRow := container.NewBorder(nil, nil, leftPad, rightPad, backgroundLine)
	lineWithOffset := container.NewVBox(linePadding, lineRow)

	timelineContainer := container.NewStack(lineWithOffset, stepsRow)

	okCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{}, 1)
	manualOverrideCh := make(chan struct{}, 1)

	okBtn := widget.NewButton("OK", func() {
		select {
		case okCh <- struct{}{}:
		default:
		}
	})

	cancelBtn := widget.NewButton("Cancel", func() {
		select {
		case cancelCh <- struct{}{}:
		default:
		}
		a.Quit()
	})

	quitBtn := widget.NewButton("Quit", func() {
		a.Quit()
	})

	manualOverrideBtn := widget.NewButton("I finished the installation. Please proceed.", func() {
		select {
		case manualOverrideCh <- struct{}{}:
		default:
		}
	})

	buttonContainer := container.NewCenter(container.NewHBox(okBtn, cancelBtn))
	buttonContainer.Hide()

	// Main layout
	content := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(titleLabel),
		widget.NewLabel(""),
		container.NewCenter(timelineContainer),
		widget.NewLabel(""),
		container.NewCenter(subtitleLabel),
		layout.NewSpacer(),
		buttonContainer,
		widget.NewLabel(""),
	)

	w.SetContent(content)

	// Open log file
	logPath := filepath.Join(os.TempDir(), "deck-game-installer-v2-debug.log")
	logFile, _ := os.Create(logPath)

	doneCh := make(chan struct{})

	g := &GUILogger{
		app:               a,
		window:            w,
		titleLabel:        titleLabel,
		subtitleLabel:     subtitleLabel,
		stepCircles:       stepCircles,
		stepLabels:        stepLabels,
		steps:             defaultSteps,
		currentStepIndex:  0,
		buttonContainer:   buttonContainer,
		okBtn:             okBtn,
		cancelBtn:         cancelBtn,
		quitBtn:           quitBtn,
		manualOverrideBtn: manualOverrideBtn,
		okCh:              okCh,
		cancelCh:          cancelCh,
		manualOverrideCh:  manualOverrideCh,
		doneCh:            doneCh,
		logFile:           logFile,
	}

	if logFile != nil {
		fmt.Fprintf(logFile, "=== Deck Game Installer v2 Debug Log ===\n")
		fmt.Fprintf(logFile, "Log file: %s\n\n", logPath)
	}

	w.SetCloseIntercept(func() {
		select {
		case cancelCh <- struct{}{}:
		default:
		}
		a.Quit()
	})

	return g
}

func (g *GUILogger) runOnUI(fn func()) {
	if driver, ok := g.app.Driver().(interface{ RunOnMain(func()) }); ok {
		driver.RunOnMain(fn)
		return
	}
	fn()
}

// Run starts the GUI event loop. This blocks until the window is closed.
func (g *GUILogger) Run() {
	g.window.Show()
	g.app.Run()
	// Signal that GUI has closed
	close(g.doneCh)
}

// WaitForClose blocks until the GUI window is closed.
func (g *GUILogger) WaitForClose() {
	<-g.doneCh
}

// Close shuts down the GUI.
func (g *GUILogger) Close() {
	g.closeOnce.Do(func() {
		g.app.Quit()
	})
}

// Log writes a message to the debug log file.
func (g *GUILogger) Log(message string) {
	if g.logFile != nil {
		fmt.Fprintln(g.logFile, message)
		g.logFile.Sync()
	}
}

// SetStep updates the UI to show the current step.
func (g *GUILogger) SetStep(name string) {
	g.runOnUI(func() {
		stepIndex := -1
		for i, step := range g.steps {
			if step == name {
				stepIndex = i
				break
			}
		}

		if stepIndex == -1 {
			g.subtitleLabel.SetText(name + "...")
			return
		}

		completeColor := color.NRGBA{R: 76, G: 175, B: 80, A: 255}
		inactiveColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}

		g.currentStepIndex = stepIndex

		for i := range g.stepCircles {
			if i < stepIndex {
				g.stepCircles[i].FillColor = inactiveColor
				g.stepCircles[i].StrokeColor = inactiveColor
				g.stepLabels[i].TextStyle.Bold = false
			} else if i == stepIndex {
				g.stepCircles[i].FillColor = completeColor
				g.stepCircles[i].StrokeColor = completeColor
				g.stepLabels[i].TextStyle.Bold = true
			} else {
				g.stepCircles[i].FillColor = color.White
				g.stepCircles[i].StrokeColor = inactiveColor
				g.stepLabels[i].TextStyle.Bold = false
			}
			g.stepCircles[i].Refresh()
			g.stepLabels[i].Refresh()
		}

		g.subtitleLabel.SetText(name + "...")
	})
}

// ConfigureSteps sets up the list of steps to display.
func (g *GUILogger) ConfigureSteps(stepNames []string) {
	g.runOnUI(func() {
		g.steps = stepNames

		for i := range g.stepLabels {
			if i < len(stepNames) {
				g.stepLabels[i].SetText(stepNames[i])
				g.stepLabels[i].Show()
				g.stepCircles[i].Show()
			} else {
				g.stepLabels[i].Hide()
				g.stepCircles[i].Hide()
			}
		}
	})
}

// Confirm shows a yes/no confirmation dialog.
func (g *GUILogger) Confirm(title, message string) bool {
	resp := make(chan bool, 1)
	g.runOnUI(func() {
		d := dialog.NewConfirm(title, message, func(ok bool) {
			resp <- ok
		}, g.window)
		d.Show()
	})
	return <-resp
}

// Select shows a selection dialog with multiple options.
func (g *GUILogger) Select(title, prompt string, options []string) (string, bool) {
	resultCh := make(chan string, 1)
	cancelCh := make(chan struct{}, 1)
	selected := ""

	commonPrefix := findCommonPrefix(options)
	displayPrefix := ""
	if len(commonPrefix) > 20 {
		displayPrefix = commonPrefix
		if idx := strings.Index(displayPrefix, "/pfx/"); idx != -1 {
			displayPrefix = displayPrefix[:idx+4]
		}
	}

	displayPrompt := prompt
	if displayPrefix != "" {
		displayPrompt += "\nPrefix: " + displayPrefix
	}

	list := widget.NewList(
		func() int { return len(options) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			text := options[i]
			if displayPrefix != "" {
				text = strings.TrimPrefix(text, displayPrefix)
				text = strings.TrimPrefix(text, "/")
			}
			text = truncateMiddle(text, 80)
			o.(*widget.Label).SetText(text)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(options) {
			selected = options[id]
		}
	}

	if len(options) == 1 {
		selected = options[0]
		list.Select(0)
	}

	content := container.NewBorder(
		widget.NewLabel(displayPrompt),
		nil, nil, nil,
		container.NewVScroll(list),
	)

	g.runOnUI(func() {
		d := dialog.NewCustomConfirm(title, "OK", "Cancel", content, func(ok bool) {
			if !ok || selected == "" {
				cancelCh <- struct{}{}
				return
			}
			resultCh <- selected
		}, g.window)
		d.Resize(fyne.NewSize(520, 420))
		d.Show()
	})

	select {
	case value := <-resultCh:
		return value, true
	case <-cancelCh:
		return "", false
	}
}

// Error shows an error dialog.
func (g *GUILogger) Error(title, message string) {
	g.runOnUI(func() {
		dialog.ShowError(fmt.Errorf("%s", message), g.window)
	})
}

// Info shows an informational dialog.
func (g *GUILogger) Info(title, message string) {
	done := make(chan struct{})
	g.runOnUI(func() {
		d := dialog.NewInformation(title, message, g.window)
		d.SetOnClosed(func() {
			close(done)
		})
		d.Show()
	})
	<-done
}

// RollbackPrompt shows a dialog asking the user if they want to roll back.
func (g *GUILogger) RollbackPrompt(errorMsg string, completedOps []string) bool {
	message := fmt.Sprintf("Error: %s\n\nThe following changes were made:\n", errorMsg)
	for _, op := range completedOps {
		message += "  • " + op + "\n"
	}
	message += "\nWould you like to undo these changes?"

	return g.Confirm("Installation Failed", message)
}

// Wait blocks until the user clicks OK.
func (g *GUILogger) Wait() {
	g.WaitWithMessage("Click OK to continue...")
}

// WaitWithMessage shows a message and OK/Cancel buttons.
func (g *GUILogger) WaitWithMessage(message string) bool {
	g.runOnUI(func() {
		g.subtitleLabel.SetText(message)
		g.buttonContainer.Objects = []fyne.CanvasObject{container.NewHBox(g.okBtn, g.cancelBtn)}
		g.buttonContainer.Show()
		g.buttonContainer.Refresh()
	})

	var res bool
	select {
	case <-g.okCh:
		res = true
	case <-g.cancelCh:
		res = false
	}

	g.runOnUI(func() {
		g.buttonContainer.Hide()
		g.okBtn.SetText("OK")
		g.cancelBtn.SetText("Cancel")
	})
	return res
}

// WaitWithManualOverride shows a single button for manual override while waiting
// for the installer to complete. Returns a channel that receives when clicked.
func (g *GUILogger) WaitWithManualOverride() <-chan struct{} {
	g.runOnUI(func() {
		g.subtitleLabel.SetText("Waiting for installer to finish...")
		g.buttonContainer.Objects = []fyne.CanvasObject{container.NewHBox(g.manualOverrideBtn)}
		g.buttonContainer.Show()
		g.buttonContainer.Refresh()
	})
	return g.manualOverrideCh
}

// ShowComplete shows the completion screen with a quit button.
func (g *GUILogger) ShowComplete() {
	g.runOnUI(func() {
		g.subtitleLabel.SetText("Installation completed successfully!")
		g.buttonContainer.Objects = []fyne.CanvasObject{container.NewHBox(g.quitBtn)}
		g.buttonContainer.Show()
		g.buttonContainer.Refresh()
	})
}

// Helper functions

func findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs {
		for !strings.HasPrefix(s, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	if idx := strings.LastIndex(prefix, "/"); idx != -1 {
		prefix = prefix[:idx+1]
	}
	return prefix
}

func truncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	partLen := (maxLen - 3) / 2
	return s[:partLen] + "..." + s[len(s)-partLen:]
}
