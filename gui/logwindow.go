package gui

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

type LogWindow struct {
	app              fyne.App
	window           fyne.Window
	gameName         string
	titleLabel       *widget.Label
	subtitleLabel    *widget.Label
	stepCircles      []*canvas.Circle
	stepLabels       []*widget.Label
	backgroundLine   *canvas.Rectangle
	steps            []string
	currentStepIndex int
	buttonContainer  *fyne.Container
	okBtn            *widget.Button
	cancelBtn        *widget.Button
	quitBtn          *widget.Button

	okCh      chan struct{}
	cancelCh  chan struct{}
	closeOnce sync.Once
	
	logFile   *os.File
}

func (l *LogWindow) runOnUI(fn func()) {
	if driver, ok := l.app.Driver().(interface{ RunOnMain(func()) }); ok {
		driver.RunOnMain(fn)
		return
	}
	fn()
}

func NewLogWindow(title string) *LogWindow {
	a := app.New()
	a.Settings().SetTheme(lightReadableTheme{})
	w := a.NewWindow(title)
	w.Resize(fyne.NewSize(700, 400))

	steps := []string{
		"Initialize",
		"Mount ISO",
		"Add to Steam",
		"Run Installer",
		"Find Game",
		"Finalize",
	}

	titleLabel := widget.NewLabelWithStyle("Installing to Steam", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	titleLabel.TextStyle.Bold = true
	
	subtitleLabel := widget.NewLabelWithStyle("Starting installation process...", fyne.TextAlignCenter, fyne.TextStyle{})
	
	// Create horizontal timeline with circles on a line
	var stepCircles []*canvas.Circle
	var stepLabels []*widget.Label
	
	inactiveColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	lineColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}
	
	// Create the background line that spans the entire width
	backgroundLine := canvas.NewRectangle(lineColor)
	backgroundLine.SetMinSize(fyne.NewSize(550, 3))
	
	// Create paired circle+label items with fixed widths for alignment
	var stepContainers []fyne.CanvasObject
	
	for _, step := range steps {
		// Circle for step - use white fill so the color shows as the stroke
		circle := canvas.NewCircle(color.White)
		circle.StrokeColor = inactiveColor
		circle.StrokeWidth = 3
		stepCircles = append(stepCircles, circle)
		
		// Wrap circle in a sized container to ensure visibility
		circleBox := canvas.NewRectangle(color.Transparent)
		circleBox.SetMinSize(fyne.NewSize(20, 20))
		circleContainer := container.NewStack(circleBox, circle)
		
		// Label below circle
		label := widget.NewLabel(step)
		label.Alignment = fyne.TextAlignCenter
		stepLabels = append(stepLabels, label)
		
		// Create a vertical container with circle and label, with min width
		stepBox := container.NewVBox(
			container.NewCenter(circleContainer),
			label,
		)
		
		stepContainers = append(stepContainers, stepBox)
	}
	
	// Build the horizontal row with spacers
	stepsRow := container.NewHBox()
	for i, stepBox := range stepContainers {
		if i > 0 {
			stepsRow.Add(layout.NewSpacer())
		}
		stepsRow.Add(stepBox)
		if i < len(stepContainers)-1 {
			stepsRow.Add(layout.NewSpacer())
		}
	}
	
	// Create a container with padding to position the line at circle center (5px down for 20px circles)
	linePadding := canvas.NewRectangle(color.Transparent)
	linePadding.SetMinSize(fyne.NewSize(1, 5))
	
	// Add horizontal padding to inset the line
	leftPad := canvas.NewRectangle(color.Transparent)
	leftPad.SetMinSize(fyne.NewSize(35, 3))
	rightPad := canvas.NewRectangle(color.Transparent)
	rightPad.SetMinSize(fyne.NewSize(35, 3))
	
	lineWithHorizontalPadding := container.NewHBox(
		leftPad,
		backgroundLine,
		rightPad,
	)
	
	lineWithOffset := container.NewVBox(
		linePadding,
		lineWithHorizontalPadding,
	)
	
	// Stack the line behind the circles
	timelineWithCircles := container.NewStack(
		lineWithOffset,
		stepsRow,
	)
	
	timelineContainer := timelineWithCircles

	okCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{}, 1)

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

	buttonContainer := container.NewCenter(container.NewHBox(okBtn, cancelBtn))
	buttonContainer.Hide()

	// Main layout
	content := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(titleLabel),
		widget.NewLabel(""), // spacer
		container.NewCenter(timelineContainer),
		widget.NewLabel(""), // spacer
		container.NewCenter(subtitleLabel),
		layout.NewSpacer(),
		buttonContainer,
		widget.NewLabel(""), // bottom padding
	)

	w.SetContent(content)

	// Open log file
	logPath := filepath.Join(os.TempDir(), "steamer-debug.log")
	logFile, _ := os.Create(logPath)

	lw := &LogWindow{
		app:              a,
		window:           w,
		gameName:         "",
		titleLabel:       titleLabel,
		subtitleLabel:    subtitleLabel,
		stepCircles:      stepCircles,
		stepLabels:       stepLabels,
		backgroundLine:   backgroundLine,
		steps:            steps,
		currentStepIndex: 0,
		buttonContainer:  buttonContainer,
		okBtn:            okBtn,
		cancelBtn:        cancelBtn,
		quitBtn:          quitBtn,
		okCh:             okCh,
		cancelCh:         cancelCh,
		logFile:          logFile,
	}

	if logFile != nil {
		fmt.Fprintf(logFile, "=== Steamer Debug Log ===\n")
		fmt.Fprintf(logFile, "Log file: %s\n\n", logPath)
	}

	w.SetCloseIntercept(func() {
		select {
		case cancelCh <- struct{}{}:
		default:
		}
		a.Quit()
	})

	return lw
}

func (l *LogWindow) Run() {
	l.window.Show()
	l.app.Run()
}

func (l *LogWindow) Close() {
	l.closeOnce.Do(func() {
		l.app.Quit()
	})
}

func (l *LogWindow) Log(message string) {
	// Write to log file for debugging
	if l.logFile != nil {
		fmt.Fprintln(l.logFile, message)
		l.logFile.Sync()
	}
}

func (l *LogWindow) SetGameName(name string) {
	l.runOnUI(func() {
		l.gameName = name
		l.titleLabel.SetText("Installing " + name)
	})
}

func (l *LogWindow) SetStep(name string) {
	l.runOnUI(func() {
		// Map step names to indices
		stepMap := map[string]int{
			"Initializing":      0,
			"Mounting ISO":      1,
			"Adding to Steam":   2,
			"Running Installer": 3,
			"Finding Game":      4,
			"Finalizing":        5,
		}

		stepIndex, ok := stepMap[name]
		if !ok {
			return
		}

		completeColor := color.NRGBA{R: 76, G: 175, B: 80, A: 255}     // Green
		inactiveColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}   // Gray

		l.currentStepIndex = stepIndex

		// Update circles and labels
		for i := range l.stepCircles {
			if i < stepIndex {
				// Completed steps - gray filled circle
				l.stepCircles[i].FillColor = inactiveColor
				l.stepCircles[i].StrokeColor = inactiveColor
				l.stepLabels[i].TextStyle.Bold = false
			} else if i == stepIndex {
				// Current step - green filled circle with bold label
				l.stepCircles[i].FillColor = completeColor
				l.stepCircles[i].StrokeColor = completeColor
				l.stepLabels[i].TextStyle.Bold = true
			} else {
				// Pending steps - white circle with gray outline
				l.stepCircles[i].FillColor = color.White
				l.stepCircles[i].StrokeColor = inactiveColor
				l.stepLabels[i].TextStyle.Bold = false
			}
			l.stepCircles[i].Refresh()
			l.stepLabels[i].Refresh()
		}

		// Keep the background line gray always
		// The circles will show progress with their colors

		// Update subtitle
		subtitle := l.getStepSubtitle(name)
		l.subtitleLabel.SetText(subtitle)
	})
}

func (l *LogWindow) getStepSubtitle(step string) string {
	switch step {
	case "Initializing":
		return "Starting installation process..."
	case "Mounting ISO":
		return "Please wait while the ISO is being mounted..."
	case "Adding to Steam":
		return "Configuring Steam library shortcut..."
	case "Running Installer":
		return "Complete the installation in the game window, then click OK below"
	case "Finding Game":
		return "Scanning for installed game files..."
	case "Finalizing":
		return "Creating final Steam library entry..."
	default:
		return "Processing..."
	}
}

func (l *LogWindow) Wait() bool {
	l.runOnUI(func() {
		l.subtitleLabel.SetText("Waiting for user input...")
		l.buttonContainer.Objects = []fyne.CanvasObject{container.NewHBox(l.okBtn, l.cancelBtn)}
		l.buttonContainer.Show()
		l.buttonContainer.Refresh()
	})

	var res bool
	select {
	case <-l.okCh:
		res = true
	case <-l.cancelCh:
		res = false
	}

	l.runOnUI(func() {
		l.buttonContainer.Hide()
		l.okBtn.SetText("OK")
		l.cancelBtn.SetText("Cancel")
		// Restore the step-specific subtitle
		if l.currentStepIndex < len(l.steps) {
			stepName := []string{"Initializing", "Mounting ISO", "Adding to Steam", "Running Installer", "Finding Game", "Finalizing"}[l.currentStepIndex]
			subtitle := l.getStepSubtitle(stepName)
			l.subtitleLabel.SetText(subtitle)
		}
	})
	return res
}

func (l *LogWindow) ShowComplete() {
	l.runOnUI(func() {
		l.subtitleLabel.SetText("Installation completed successfully!")
		l.buttonContainer.Objects = []fyne.CanvasObject{container.NewHBox(l.quitBtn)}
		l.buttonContainer.Show()
		l.buttonContainer.Refresh()
	})
}

func (l *LogWindow) SetButtons(okLabel, cancelLabel string) {
	l.runOnUI(func() {
		if okLabel != "" {
			l.okBtn.SetText(okLabel)
		}
		if cancelLabel != "" {
			l.cancelBtn.SetText(cancelLabel)
		}
	})
}

func (l *LogWindow) Confirm(title, message string) bool {
	resp := make(chan bool, 1)
	l.runOnUI(func() {
		d := dialog.NewConfirm(title, message, func(ok bool) {
			resp <- ok
		}, l.window)
		d.Show()
	})
	return <-resp
}

func (l *LogWindow) Info(title, message string) {
	l.runOnUI(func() {
		dialog.ShowInformation(title, message, l.window)
	})
}

func (l *LogWindow) Error(title, message string) {
	l.runOnUI(func() {
		dialog.ShowError(fmt.Errorf(message), l.window)
	})
}

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

func (l *LogWindow) Select(title, prompt string, options []string) (string, bool) {
	resultCh := make(chan string, 1)
	cancelCh := make(chan struct{}, 1)
	selected := ""

	commonPrefix := findCommonPrefix(options)
	displayPrefix := ""
	if len(commonPrefix) > 20 {
		displayPrefix = commonPrefix
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
			}
			text = truncateMiddle(text, 55)
			o.(*widget.Label).SetText(text)
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(options) {
			selected = options[id]
		}
	}

	content := container.NewBorder(
		widget.NewLabel(displayPrompt),
		nil,
		nil,
		nil,
		container.NewVScroll(list),
	)

	l.runOnUI(func() {
		d := dialog.NewCustomConfirm(title, "OK", "Cancel", content, func(ok bool) {
			if !ok || selected == "" {
				cancelCh <- struct{}{}
				return
			}
			resultCh <- selected
		}, l.window)
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
