package gui

import (
	"fmt"
	"image/color"
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
	w.Resize(fyne.NewSize(700, 500))

	text := widget.NewLabel("")
	text.Wrapping = fyne.TextWrapWord

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

	buttons := container.NewHBox(layout.NewSpacer(), okBtn, cancelBtn)

	content := container.NewBorder(nil, buttons, nil, nil, container.NewVScroll(text))
	w.SetContent(content)

	lw := &LogWindow{
		app:     a,
		window:  w,
		text:    text,
		okBtn:   okBtn,
		cancelBtn: cancelBtn,
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
		current := l.text.Text
		if current != "" {
			current += "\n"
		}
		current += message
		l.text.SetText(current)
	})
}

func (l *LogWindow) Wait() bool {
	select {
	case <-l.okCh:
		return true
	case <-l.cancelCh:
		return false
	}
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

func (l *LogWindow) Select(title, prompt string, options []string) (string, bool) {
	resultCh := make(chan string, 1)
	cancelCh := make(chan struct{}, 1)
	selected := ""

	list := widget.NewList(
		func() int { return len(options) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(options[i])
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(options) {
			selected = options[id]
		}
	}

	content := container.NewBorder(
		widget.NewLabel(prompt),
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
