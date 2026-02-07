package gui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
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
	app    fyne.App
	window fyne.Window
	text   *widget.Label
	okBtn  *widget.Button
	cancelBtn *widget.Button
	stepList *widget.List
	steps []string
	stepStatus map[string]string
	logLines []string
	logList *widget.List
	stepTitle *widget.Label
	stepSubtitle *widget.Label
	buttonContainer *fyne.Container

	okCh     chan struct{}
	cancelCh chan struct{}
	closeOnce sync.Once
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
	w.Resize(fyne.NewSize(900, 560))

	text := widget.NewLabel("")
	text.Wrapping = fyne.TextWrapWord
	text.TextStyle = fyne.TextStyle{Monospace: true}

	stepTitle := widget.NewLabelWithStyle("Initializing", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	stepSubtitle := widget.NewLabel("Starting installation process...")

	steps := []string{
		"Initializing",
		"Mounting ISO",
		"Adding to Steam",
		"Running Installer",
		"Finding Game",
		"Finalizing",
	}
	stepStatus := map[string]string{}
	for _, s := range steps {
		stepStatus[s] = "pending"
	}

	stepList := widget.NewList(
		func() int { return len(steps) },
		func() fyne.CanvasObject {
			icon := widget.NewLabel("")
			name := widget.NewLabel("")
			name.Wrapping = fyne.TextWrapOff
			name.Resize(fyne.NewSize(180, name.MinSize().Height))
			return container.NewHBox(icon, name)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container).Objects
			icon := row[0].(*widget.Label)
			label := row[1].(*widget.Label)
			name := steps[i]
			switch stepStatus[name] {
			case "done":
				icon.SetText("✔")
				label.SetText(name)
			case "current":
				icon.SetText("▶")
				label.SetText(name)
			default:
				icon.SetText("•")
				label.SetText(name)
			}
		},
	)

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

	buttonContainer := container.NewHBox(layout.NewSpacer(), okBtn, cancelBtn)
	buttonContainer.Hide()

	logLines := []string{}
	logList := widget.NewList(
		func() int { return len(logLines) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(logLines[i])
		},
	)

	stepsCard := widget.NewCard("Steps", "", container.NewVScroll(stepList))
	logHeader := container.NewVBox(stepTitle, stepSubtitle)
	logCard := widget.NewCard("Activity", "", container.NewBorder(logHeader, buttonContainer, nil, nil, container.NewVScroll(logList)))

	split := container.NewHSplit(stepsCard, logCard)
	split.Offset = 0.32

	content := container.NewBorder(nil, nil, nil, nil, split)
	w.SetContent(content)

	lw := &LogWindow{
		app:     a,
		window:  w,
		text:    text,
		okBtn:   okBtn,
		cancelBtn: cancelBtn,
		buttonContainer: buttonContainer,
		stepList: stepList,
		steps: steps,
		stepStatus: stepStatus,
		logLines: logLines,
		logList: logList,
		stepTitle: stepTitle,
		stepSubtitle: stepSubtitle,
		okCh:    okCh,
		cancelCh: cancelCh,
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
	l.runOnUI(func() {
		l.logLines = append(l.logLines, message)
		l.logList.Refresh()
		// Scroll to the bottom to show the latest log
		l.logList.ScrollToBottom()
	})
}

func (l *LogWindow) SetStep(name string) {
	l.runOnUI(func() {
		seenCurrent := false
		for _, step := range l.steps {
			if step == name {
				l.stepStatus[step] = "current"
				seenCurrent = true
				continue
			}
			if !seenCurrent {
				l.stepStatus[step] = "done"
			} else if l.stepStatus[step] != "done" {
				l.stepStatus[step] = "pending"
			}
		}
		l.stepList.Refresh()
		l.stepTitle.SetText(name)
		
		// Update subtitle with contextual information
		subtitle := l.getStepSubtitle(name)
		l.stepSubtitle.SetText(subtitle)
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
		return "Action required - Complete the installation in the game window"
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
		l.stepSubtitle.SetText("Waiting for user input...")
		l.buttonContainer.Show()
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
		subtitle := l.getStepSubtitle(l.stepTitle.Text)
		l.stepSubtitle.SetText(subtitle)
	})
	return res
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
