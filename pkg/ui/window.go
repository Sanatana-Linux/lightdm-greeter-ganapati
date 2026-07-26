//go:build !headless

package ui

import (
	"image/color"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/dbus"
)

// UIActionHandler defines actions triggered by user interaction in the UI
type UIActionHandler interface {
	OnAuthenticate(username string)
	OnRespond(password string)
	OnCancel()
	OnStartSession(sessionID string)
}

// Window handles the main Gio presentation loop and immediate-mode layout rendering
type Window struct {
	theme       *material.Theme
	sessions    []dbus.Session
	currentSess int
	statusMsg   string
	isError     bool

	// Widgets state
	usernameEditor widget.Editor
	passwordEditor widget.Editor
	loginClick     widget.Clickable
	cancelClick    widget.Clickable
	sessionClicks  []widget.Clickable
	showSessions   bool
	sessMenuClick  widget.Clickable

	// Authentication state tracking
	isAuthenticating bool
	needPassword     bool
	promptText       string
}

// NewWindow initializes the presentation state with standard system fonts
func NewWindow(sessions []dbus.Session) *Window {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	// Set custom minimalist colors (Libadwaita dark background with blue accents)
	th.Palette.Bg = color.NRGBA{R: 0x24, G: 0x24, B: 0x24, A: 0xFF}         // Libadwaita Dark Background
	th.Palette.Fg = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}         // Pure White Text
	th.Palette.ContrastBg = color.NRGBA{R: 0x35, G: 0x84, B: 0xE4, A: 0xFF} // Libadwaita Blue Accent
	th.Palette.ContrastFg = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // White Text

	w := &Window{
		theme:          th,
		sessions:       sessions,
		sessionClicks:  make([]widget.Clickable, len(sessions)),
		usernameEditor: widget.Editor{SingleLine: true, Submit: true},
		passwordEditor: widget.Editor{SingleLine: true, Submit: true, Mask: '*'},
	}

	return w
}

// Run starts the graphical event loop and listens to background events
func (w *Window) Run(events <-chan dbus.GreeterEvent, handler UIActionHandler) error {
	// Event router combining LightDM D-Bus signals and UI drawing frames
	go func() {
		for ev := range events {
			switch ev.Type {
			case dbus.EventShowPrompt:
				w.isAuthenticating = true
				w.needPassword = true
				w.promptText = ev.Text
				w.statusMsg = ""
			case dbus.EventShowMessage:
				w.statusMsg = ev.Text
				w.isError = ev.Param == 1
			case dbus.EventAuthComplete:
				w.isAuthenticating = false
				w.needPassword = false
				if w.statusMsg == "" {
					w.statusMsg = "Logged in successfully. Starting session..."
					w.isError = false
					if w.currentSess < len(w.sessions) {
						handler.OnStartSession(w.sessions[w.currentSess].ID)
					}
				}
			case dbus.EventReset:
				w.isAuthenticating = false
				w.needPassword = false
				w.usernameEditor.SetText("")
				w.passwordEditor.SetText("")
				w.statusMsg = ""
			}
		}
	}()

	// Run Gio window loop in a goroutine per official documentation pattern, and block with app.Main() on main thread
	go func() {
		window := new(app.Window)
		window.Option(app.Title("LightDM Greeter Ganapati"))
		window.Option(app.Size(unit.Dp(800), unit.Dp(600)))

		var ops op.Ops
		for {
			e := window.Event()
			switch event := e.(type) {
			case app.DestroyEvent:
				os.Exit(0)
			case app.FrameEvent:
				gtx := app.NewContext(&ops, event)
				w.handleActions(gtx, handler)
				w.Render(gtx)
				event.Frame(gtx.Ops)
			}
		}
	}()

	app.Main()
	return nil
}

// handleActions processes button clicks
func (w *Window) handleActions(gtx layout.Context, handler UIActionHandler) {
	// Login button click handling (simplified: text retrieved directly)
	if w.loginClick.Clicked(gtx) {
		if !w.isAuthenticating {
			user := w.usernameEditor.Text()
			if user != "" {
				handler.OnAuthenticate(user)
			}
		} else if w.needPassword {
			pass := w.passwordEditor.Text()
			handler.OnRespond(pass)
		}
	}

	// Cancel button click handling
	if w.cancelClick.Clicked(gtx) {
		handler.OnCancel()
		w.isAuthenticating = false
		w.needPassword = false
		w.usernameEditor.SetText("")
		w.passwordEditor.SetText("")
		w.statusMsg = ""
	}

	// Session selection menu handling
	if w.sessMenuClick.Clicked(gtx) {
		w.showSessions = !w.showSessions
	}

	for i := range w.sessionClicks {
		if w.sessionClicks[i].Clicked(gtx) {
			w.currentSess = i
			w.showSessions = false
		}
	}
}
