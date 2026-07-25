//go:build !headless

package ui

import (
	"image/color"
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/max-moser/lightdm-elephant-greeter/pkg/dbus"
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

	// Set custom minimalist colors (charcoal dark GRUB-like theme)
	th.Palette.Bg = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}         // Dark Charcoal
	th.Palette.Fg = color.NRGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}         // Warm White
	th.Palette.ContrastBg = color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xFF} // Cool Gray Accent
	th.Palette.ContrastFg = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // White

	w := &Window{
		theme:          th,
		sessions:       sessions,
		sessionClicks:  make([]widget.Clickable, len(sessions)),
		usernameEditor: widget.Editor{SingleLine: true, Submit: true},
		passwordEditor: widget.Editor{SingleLine: true, Submit: true, Mask: '*'},
	}

	// Ensure username is initially focused
	w.usernameEditor.Focus()
	return w
}

// Run starts the graphical event loop and listens to background events
func (w *Window) Run(events <-chan dbus.GreeterEvent, handler UIActionHandler) error {
	// Channel to signal Gio window events
	gioEvents := make(chan interface{}, 10)

	go func() {
		// New window options
		window := new(app.Window)
		window.Option(app.Title("LightDM Elephant Greeter"))
		window.Option(app.Size(unit.Dp(800), unit.Dp(600)))

		var ops op.Ops
		for {
			select {
			case e := <-gioEvents:
				switch event := e.(type) {
				case system.DestroyEvent:
					os.Exit(0)
				case system.FrameEvent:
					gtx := layout.NewContext(&ops, event)
					w.handleActions(gtx, handler)
					w.Render(gtx)
					event.Frame(gtx.Ops)
				}
			}
		}
	}()

	// Event router combining LightDM D-Bus signals and UI drawing frames
	go func() {
		for ev := range events {
			switch ev.Type {
			case dbus.EventShowPrompt:
				w.isAuthenticating = true
				w.needPassword = true
				w.promptText = ev.Text
				w.statusMsg = ""
				w.passwordEditor.Focus()

			case dbus.EventShowMessage:
				w.statusMsg = ev.Text
				w.isError = ev.Param == 1 // 1 represents Error message in LightDM specification

			case dbus.EventAuthComplete:
				w.isAuthenticating = false
				w.needPassword = false
				if w.statusMsg == "" {
					w.statusMsg = "Logged in successfully. Starting session..."
					w.isError = false
					// Trigger session launch for the chosen selection
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
				w.usernameEditor.Focus()
			}

			// Request redraw after modifying display states
			app.NewWindow().Invalidate()
		}
	}()

	// Run Gio main window loop
	app.Main()
	return nil
}

// handleActions processes button clicks and submission keystrokes inside the frame context
func (w *Window) handleActions(gtx layout.Context, handler UIActionHandler) {
	// Form submission handling (Enter pressed)
	for _, ev := range w.usernameEditor.Events() {
		if _, ok := ev.(widget.SubmitEvent); ok {
			user := w.usernameEditor.Text()
			if user != "" {
				handler.OnAuthenticate(user)
			}
		}
	}

	for _, ev := range w.passwordEditor.Events() {
		if _, ok := ev.(widget.SubmitEvent); ok {
			pass := w.passwordEditor.Text()
			handler.OnRespond(pass)
		}
	}

	// Login button click handling
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
		w.usernameEditor.Focus()
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
