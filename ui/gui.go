package ui

import (
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	fyneapp "fyne.io/fyne/v2/app"
)

// Step status constants
type StepStatusType int

const (
	StepPending StepStatusType = iota
	StepRunning
	StepCompleted
	StepFailed
)

// Colors
var (
	colPending   = color.NRGBA{R: 150, G: 150, B: 150, A: 255} // Gray
	colRunning   = color.NRGBA{R: 255, G: 193, B: 7, A: 255}   // Amber
	colCompleted = color.NRGBA{R: 76, G: 175, B: 80, A: 255}   // Green
	colFailed    = color.NRGBA{R: 244, G: 67, B: 54, A: 255}   // Red
	colLine      = color.NRGBA{R: 180, G: 180, B: 180, A: 255} // Neutral gray for connector lines
)

// StepStatus tracks the state of a single installation step
type StepStatus struct {
	Name      string
	Status    StepStatusType
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Logs      []string
	Error     error
	Expanded  bool
}

// GUILogger implements Logger with a Fyne-based graphical interface.
type GUILogger struct {
	app    fyne.App
	window fyne.Window

	// Step tracking
	stepStatuses   map[string]*StepStatus
	stepOrder      []string
	currentStep    string
	totalSteps     int
	completedSteps int
	mu             sync.Mutex

	// UI elements
	titleLabel     *widget.Label
	statusLabel    *widget.Label
	progressBar    *widget.ProgressBar
	stepListScroll *container.Scroll
	stepListBox    *fyne.Container
	bottomBox      *fyne.Container
	themeBtn       *widget.Button

	// Buttons
	okBtn        *widget.Button
	cancelBtn    *widget.Button
	quitBtn      *widget.Button
	proceedBtn   *widget.Button
	openSteamBtn *widget.Button
	closeBtn     *widget.Button

	// Channels
	okCh      chan struct{}
	cancelCh  chan struct{}
	proceedCh chan struct{}
	doneCh    chan struct{}
	closeOnce sync.Once

	// Theme
	darkMode bool
	filename string

	logFile *os.File
}

// NewGUILogger creates a new graphical logger window.
func NewGUILogger(windowTitle, filename string) *GUILogger {
	a := fyneapp.New()
	a.Settings().SetTheme(theme.LightTheme())
	w := a.NewWindow(windowTitle)
	w.Resize(fyne.NewSize(500, 600))

	okCh := make(chan struct{}, 1)
	cancelCh := make(chan struct{}, 1)
	proceedCh := make(chan struct{}, 1)
	doneCh := make(chan struct{})

	g := &GUILogger{
		app:          a,
		window:       w,
		stepStatuses: make(map[string]*StepStatus),
		stepOrder:    nil,
		darkMode:     false,
		okCh:         okCh,
		cancelCh:     cancelCh,
		proceedCh:    proceedCh,
		doneCh:       doneCh,
		filename:     filename,
	}

	// Create buttons
	g.okBtn = widget.NewButton("OK", func() {
		select {
		case okCh <- struct{}{}:
		default:
		}
	})

	g.cancelBtn = widget.NewButton("Cancel", func() {
		select {
		case cancelCh <- struct{}{}:
		default:
		}
		a.Quit()
	})

	g.quitBtn = widget.NewButton("Quit", func() {
		a.Quit()
	})

			       g.proceedBtn = widget.NewButton("I finished the installation. Please proceed.", func() {
				       select {
				       case proceedCh <- struct{}{}:
				       default:
				       }
			       })
			       g.proceedBtn.Disable()

			       g.openSteamBtn = widget.NewButton("Open in Steam", func() {
				       exec.Command("steam", "steam://open/library").Start()
				       // Start the countdown after Steam is restarted
				       g.proceedBtn.Disable()
				       go func(btn *widget.Button) {
					       for i := 10; i > 0; i-- {
						       btn.SetText(fmt.Sprintf("I finished the installation. Please proceed. (%ds)", i))
						       time.Sleep(time.Second)
					       }
					       btn.SetText("I finished the installation. Please proceed.")
					       btn.Enable()
				       }(g.proceedBtn)
			       })

	g.closeBtn = widget.NewButton("Close", func() {
		a.Quit()
	})

	// Build the UI
	g.buildUI()

	// Log file
	logPath := filepath.Join(os.TempDir(), "deck-game-installer.log")
	g.logFile, _ = os.Create(logPath)

	w.SetCloseIntercept(func() {
		select {
		case cancelCh <- struct{}{}:
		default:
		}
		a.Quit()
	})

	return g
}

func (g *GUILogger) buildUI() {
	// Header: icon + title + spacer + theme toggle + progress counter
	appIcon := canvas.NewImageFromResource(theme.ComputerIcon())
	appIcon.SetMinSize(fyne.NewSize(40, 40))
	appIcon.FillMode = canvas.ImageFillContain

	g.titleLabel = widget.NewLabelWithStyle(fmt.Sprintf("Installing %s", g.filename), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	g.statusLabel = widget.NewLabel("Initializing...")
	g.statusLabel.TextStyle = fyne.TextStyle{}

	// Theme toggle button (sun/moon icon)
	g.themeBtn = widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
		g.toggleTheme()
	})

	titleBox := container.NewVBox(g.titleLabel, g.statusLabel)

	header := container.NewBorder(nil, nil, container.NewHBox(appIcon, titleBox), g.themeBtn)

	// Progress bar
	g.progressBar = widget.NewProgressBar()
	g.progressBar.Min = 0
	g.progressBar.Max = 1

	// Step list (will be populated by ConfigureSteps)
	g.stepListBox = container.NewVBox()
	g.stepListScroll = container.NewVScroll(g.stepListBox)
	g.stepListScroll.SetMinSize(fyne.NewSize(380, 400))

	// Bottom box for buttons and completion message
	g.bottomBox = container.NewVBox()

	// Main content
	content := container.NewBorder(
		container.NewVBox(header, widget.NewSeparator(), g.progressBar),
		g.bottomBox,
		nil, nil,
		container.NewPadded(g.stepListScroll),
	)

	g.window.SetContent(container.NewPadded(content))
}

func (g *GUILogger) toggleTheme() {
	g.darkMode = !g.darkMode
	if g.darkMode {
		g.app.Settings().SetTheme(theme.DarkTheme())
	} else {
		g.app.Settings().SetTheme(theme.LightTheme())
	}
}

func (g *GUILogger) runOnUI(fn func()) {
	if d, ok := g.app.Driver().(interface{ RunOnMain(func()) }); ok {
		d.RunOnMain(fn)
		return
	}
	fn()
}

func (g *GUILogger) Run() {
	g.window.Show()
	g.app.Run()
	close(g.doneCh)
}

func (g *GUILogger) WaitForClose() {
	<-g.doneCh
}

func (g *GUILogger) Close() {
	g.closeOnce.Do(func() { g.app.Quit() })
}

func (g *GUILogger) Log(msg string) {
	if g.logFile != nil {
		fmt.Fprintln(g.logFile, msg)
		g.logFile.Sync()
	}

	// Add log to current step
	g.mu.Lock()
	if g.currentStep != "" {
		if status, ok := g.stepStatuses[g.currentStep]; ok {
			status.Logs = append(status.Logs, msg)
		}
	}
	g.mu.Unlock()

	// Refresh the UI to show new log
	g.runOnUI(func() {
		g.refreshStepList()
	})
}

func (g *GUILogger) SetStep(name string) {
	// For backward compatibility, delegate to StepStarted
	g.StepStarted(name)
}

func (g *GUILogger) StepStarted(name string) {
	g.mu.Lock()
	g.currentStep = name
	if status, ok := g.stepStatuses[name]; ok {
		status.Status = StepRunning
		status.StartTime = time.Now()
	}
	g.mu.Unlock()

	g.runOnUI(func() {
		g.updateProgress()
		g.refreshStepList()
	})
}

func (g *GUILogger) StepCompleted(name string, err error) {
	g.mu.Lock()
	if status, ok := g.stepStatuses[name]; ok {
		status.EndTime = time.Now()
		status.Duration = status.EndTime.Sub(status.StartTime)
		if err != nil {
			status.Status = StepFailed
			status.Error = err
		} else {
			status.Status = StepCompleted
			g.completedSteps++
		}
	}
	g.mu.Unlock()

	g.runOnUI(func() {
		g.updateProgress()
		g.refreshStepList()
	})
}

func (g *GUILogger) ConfigureSteps(names []string) {
	// Deduplicate step names
	seen := make(map[string]bool)
	var unique []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			unique = append(unique, n)
		}
	}

	g.mu.Lock()
	g.stepOrder = unique
	g.totalSteps = len(unique)
	g.stepStatuses = make(map[string]*StepStatus)
	for _, name := range unique {
		g.stepStatuses[name] = &StepStatus{
			Name:   name,
			Status: StepPending,
		}
	}
	g.mu.Unlock()

	g.runOnUI(func() {
		g.updateProgress()
		g.refreshStepList()
	})
}

func (g *GUILogger) updateProgress() {
	g.mu.Lock()
	completed := g.completedSteps
	total := g.totalSteps
	current := g.currentStep
	g.mu.Unlock()

	// Count running as partial progress
	progress := float64(completed) / float64(total)
	g.progressBar.SetValue(progress)

	if current != "" {
		g.statusLabel.SetText(fmt.Sprintf("Installing... step %d of %d", completed+1, total))
	}
}

func (g *GUILogger) refreshStepList() {
	g.mu.Lock()
	order := g.stepOrder
	statuses := make(map[string]*StepStatus)
	for k, v := range g.stepStatuses {
		statuses[k] = v
	}
	g.mu.Unlock()

	g.stepListBox.Objects = nil

	for i, name := range order {
		status := statuses[name]
		isLast := i == len(order)-1
		stepWidget := g.createStepWidget(status, isLast, i)
		g.stepListBox.Add(stepWidget)
	}

	g.stepListBox.Refresh()

	// Auto-scroll to show the current running step
	g.stepListScroll.ScrollToBottom()
}

func (g *GUILogger) createStepWidget(status *StepStatus, isLast bool, index int) fyne.CanvasObject {
	// Circle indicator
	circleSize := float32(24)
	circle := canvas.NewCircle(g.getStatusColor(status.Status))
	circle.StrokeWidth = 0

	// Status icon inside circle
	var iconText string
	switch status.Status {
	case StepCompleted:
		iconText = "✓"
	case StepFailed:
		iconText = "✗"
	default:
		iconText = ""
	}

	iconLabel := canvas.NewText(iconText, color.White)
	iconLabel.TextSize = 14
	iconLabel.TextStyle = fyne.TextStyle{Bold: true}
	iconLabel.Alignment = fyne.TextAlignCenter

	circleContainer := container.NewStack(
		container.NewCenter(newSizedCircle(circle, circleSize)),
		container.NewCenter(iconLabel),
	)

	// Connector line (vertical, below the circle)
	var lineContainer fyne.CanvasObject
	if !isLast {
		line := canvas.NewRectangle(colLine)
		lineHeight := float32(20)
		if status.Expanded && len(status.Logs) > 0 {
			// Extend line when logs are expanded
			lineHeight = float32(100)
		}
		line.SetMinSize(fyne.NewSize(2, lineHeight))
		lineContainer = container.NewVBox(
			container.NewCenter(line),
		)
	} else {
		lineContainer = layout.NewSpacer()
	}

	// Left column: circle + line
	leftColumn := container.NewVBox(
		circleContainer,
		lineContainer,
	)

	// Step name
	nameLabel := widget.NewLabel(status.Name)
	nameLabel.TextStyle = fyne.TextStyle{Bold: status.Status == StepRunning}

	// Duration label
	durationLabel := widget.NewLabel("")
	if status.Status == StepCompleted || status.Status == StepFailed {
		durationLabel.SetText(fmt.Sprintf("%.1fs", status.Duration.Seconds()))
	}
	durationLabel.Alignment = fyne.TextAlignTrailing

	// Chevron for expand/collapse
	chevronText := "▶"
	if status.Expanded {
		chevronText = "▼"
	}
	chevron := widget.NewLabel(chevronText)

	// Main row: name + spacer + duration + chevron
	mainRow := container.NewBorder(
		nil, nil,
		nameLabel,
		container.NewHBox(durationLabel, chevron),
	)

	// Build content area (logs if expanded)
	var contentArea fyne.CanvasObject
	if status.Expanded && len(status.Logs) > 0 {
		logsText := strings.Join(status.Logs, "\n")
		logsLabel := widget.NewLabel(logsText)
		logsLabel.TextStyle = fyne.TextStyle{Monospace: true}
		logsLabel.Wrapping = fyne.TextWrapBreak

		logsScroll := container.NewVScroll(logsLabel)
		logsScroll.SetMinSize(fyne.NewSize(300, 80)) // ~5 lines

		// Scroll to bottom
		logsScroll.ScrollToBottom()

		contentArea = container.NewVBox(
			mainRow,
			container.NewPadded(logsScroll),
		)
	} else {
		contentArea = mainRow
	}

	// Right column: content
	rightColumn := container.NewBorder(nil, nil, nil, nil, contentArea)

	// Full step row
	stepRow := container.NewBorder(
		nil, nil,
		leftColumn,
		nil,
		rightColumn,
	)

	// Make tappable
	tappable := newTappableContainer(stepRow, func() {
		g.mu.Lock()
		status.Expanded = !status.Expanded
		g.mu.Unlock()
		g.runOnUI(func() {
			g.refreshStepList()
		})
	})

	return tappable
}

func (g *GUILogger) getStatusColor(status StepStatusType) color.Color {
	switch status {
	case StepPending:
		return colPending
	case StepRunning:
		return colRunning
	case StepCompleted:
		return colCompleted
	case StepFailed:
		return colFailed
	default:
		return colPending
	}
}

func (g *GUILogger) Confirm(title, msg string) bool {
	ch := make(chan bool, 1)
	g.runOnUI(func() {
		dialog.NewConfirm(title, msg, func(ok bool) { ch <- ok }, g.window).Show()
	})
	return <-ch
}

func (g *GUILogger) Select(title, prompt string, opts []string) (string, bool) {
	resCh := make(chan string, 1)
	canCh := make(chan struct{}, 1)
	sel := ""

	pfx := findCommonPrefix(opts)
	dispPfx := ""
	if len(pfx) > 20 {
		dispPfx = pfx
		if i := strings.Index(dispPfx, "/pfx/"); i != -1 {
			dispPfx = dispPfx[:i+4]
		}
	}

	p := prompt
	if dispPfx != "" {
		p += "\n\nPrefix: " + dispPfx
	}

	list := widget.NewList(
		func() int { return len(opts) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			t := opts[i]
			if dispPfx != "" {
				t = strings.TrimPrefix(t, dispPfx)
				t = strings.TrimPrefix(t, "/")
			}
			o.(*widget.Label).SetText(truncMid(t, 60))
		},
	)
	list.OnSelected = func(i widget.ListItemID) {
		if i >= 0 && i < len(opts) {
			sel = opts[i]
		}
	}
	if len(opts) == 1 {
		sel = opts[0]
		list.Select(0)
	}

	g.runOnUI(func() {
		d := dialog.NewCustomConfirm(title, "OK", "Cancel",
			container.NewBorder(widget.NewLabel(p), nil, nil, nil, container.NewVScroll(list)),
			func(ok bool) {
				if !ok || sel == "" {
					canCh <- struct{}{}
				} else {
					resCh <- sel
				}
			}, g.window)
		d.Resize(fyne.NewSize(480, 340))
		d.Show()
	})

	select {
	case v := <-resCh:
		return v, true
	case <-canCh:
		return "", false
	}
}

func (g *GUILogger) Error(title, msg string) {
	g.runOnUI(func() { dialog.ShowError(fmt.Errorf("%s", msg), g.window) })
}

func (g *GUILogger) Info(title, msg string) {
	ch := make(chan struct{})
	g.runOnUI(func() {
		d := dialog.NewInformation(title, msg, g.window)
		d.SetOnClosed(func() { close(ch) })
		d.Show()
	})
	<-ch
}

func (g *GUILogger) RollbackPrompt(err string, ops []string) bool {
	m := fmt.Sprintf("Error: %s\n\nChanges made:\n", err)
	for _, o := range ops {
		m += "  • " + o + "\n"
	}
	m += "\nUndo these changes?"
	return g.Confirm("Installation Failed", m)
}

func (g *GUILogger) Wait() {
	g.WaitWithMessage("Click OK to continue...")
}

func (g *GUILogger) WaitWithMessage(msg string) bool {
	g.runOnUI(func() {
		g.statusLabel.SetText(msg)
		g.bottomBox.Objects = []fyne.CanvasObject{
			container.NewCenter(container.NewHBox(g.okBtn, g.cancelBtn)),
		}
		g.bottomBox.Refresh()
	})

	var r bool
	select {
	case <-g.okCh:
		r = true
	case <-g.cancelCh:
		r = false
	}

	g.runOnUI(func() {
		g.bottomBox.Objects = nil
		g.bottomBox.Refresh()
	})
	return r
}

func (g *GUILogger) WaitWithManualOverride() <-chan struct{} {
	       g.runOnUI(func() {
		       g.statusLabel.SetText("Waiting for installer to finish...")
		       g.bottomBox.Objects = []fyne.CanvasObject{
			       container.NewCenter(container.NewHBox(g.proceedBtn)),
		       }
		       g.bottomBox.Refresh()
		       // Start countdown when button becomes visible
		       g.proceedBtn.Disable()
		       go func(btn *widget.Button) {
			       for i := 10; i > 0; i-- {
				       btn.SetText(fmt.Sprintf("I finished the installation. Please proceed. (%ds)", i))
				       time.Sleep(time.Second)
			       }
			       btn.SetText("I finished the installation. Please proceed.")
			       btn.Enable()
		       }(g.proceedBtn)
	       })
	return g.proceedCh
}

func (g *GUILogger) ShowComplete() {
	g.runOnUI(func() {
		g.mu.Lock()
		g.completedSteps = g.totalSteps
		g.mu.Unlock()

		g.progressBar.SetValue(1)
		g.statusLabel.SetText("Installation complete")

		// Success banner
		successIcon := canvas.NewText("✓", colCompleted)
		successIcon.TextSize = 24
		successIcon.TextStyle = fyne.TextStyle{Bold: true}

		successTitle := widget.NewLabelWithStyle("Your Game is ready to play", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		successMsg := widget.NewLabel("Your game has been added to Steam and can\nbe found in your library under Non-Steam games.")
		successMsg.Wrapping = fyne.TextWrapWord

		successContent := container.NewVBox(
			container.NewHBox(successIcon, successTitle),
			successMsg,
			widget.NewSeparator(),
			container.NewHBox(g.openSteamBtn, g.closeBtn),
		)

		// Create a bordered success box
		successBox := container.NewPadded(successContent)

		g.bottomBox.Objects = []fyne.CanvasObject{
			widget.NewSeparator(),
			successBox,
		}
		g.bottomBox.Refresh()
	})
}

func (g *GUILogger) ShowFailed(errMsg string) {
	g.runOnUI(func() {
		g.statusLabel.SetText("Installation failed")

		// Failure banner
		failIcon := canvas.NewText("✗", colFailed)
		failIcon.TextSize = 24
		failIcon.TextStyle = fyne.TextStyle{Bold: true}

		failTitle := widget.NewLabelWithStyle("Installation Failed", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		failMsg := widget.NewLabel(errMsg)
		failMsg.Wrapping = fyne.TextWrapWord

		failContent := container.NewVBox(
			container.NewHBox(failIcon, failTitle),
			failMsg,
			widget.NewSeparator(),
			container.NewCenter(g.closeBtn),
		)

		failBox := container.NewPadded(failContent)

		g.bottomBox.Objects = []fyne.CanvasObject{
			widget.NewSeparator(),
			failBox,
		}
		g.bottomBox.Refresh()
	})
}

// Helper: sized circle wrapper
type sizedCircle struct {
	widget.BaseWidget
	circle *canvas.Circle
	size   float32
}

func newSizedCircle(circle *canvas.Circle, size float32) *sizedCircle {
	s := &sizedCircle{circle: circle, size: size}
	s.ExtendBaseWidget(s)
	return s
}

func (s *sizedCircle) CreateRenderer() fyne.WidgetRenderer {
	return &sizedCircleRenderer{circle: s.circle, size: s.size}
}

type sizedCircleRenderer struct {
	circle *canvas.Circle
	size   float32
}

func (r *sizedCircleRenderer) Layout(size fyne.Size) {
	r.circle.Resize(fyne.NewSize(r.size, r.size))
}

func (r *sizedCircleRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.size, r.size)
}

func (r *sizedCircleRenderer) Refresh() {
	r.circle.Refresh()
}

func (r *sizedCircleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.circle}
}

func (r *sizedCircleRenderer) Destroy() {}

// Helper: tappable container
type tappableContainer struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	onTapped func()
}

func newTappableContainer(content fyne.CanvasObject, onTapped func()) *tappableContainer {
	t := &tappableContainer{content: content, onTapped: onTapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableContainer) CreateRenderer() fyne.WidgetRenderer {
	return &tappableContainerRenderer{content: t.content}
}

func (t *tappableContainer) Tapped(*fyne.PointEvent) {
	if t.onTapped != nil {
		t.onTapped()
	}
}

func (t *tappableContainer) TappedSecondary(*fyne.PointEvent) {}

type tappableContainerRenderer struct {
	content fyne.CanvasObject
}

func (r *tappableContainerRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
}

func (r *tappableContainerRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *tappableContainerRenderer) Refresh() {
	r.content.Refresh()
}

func (r *tappableContainerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *tappableContainerRenderer) Destroy() {}

// Utility functions
func findCommonPrefix(s []string) string {
	if len(s) == 0 {
		return ""
	}
	p := s[0]
	for _, x := range s {
		for !strings.HasPrefix(x, p) {
			p = p[:len(p)-1]
		}
	}
	if i := strings.LastIndex(p, "/"); i != -1 {
		p = p[:i+1]
	}
	return p
}

func truncMid(s string, m int) string {
	if len(s) <= m {
		return s
	}
	n := (m - 3) / 2
	return s[:n] + "..." + s[len(s)-n:]
}
