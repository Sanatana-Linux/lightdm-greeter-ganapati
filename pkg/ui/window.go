//go:build !headless

package ui

import (
	"image"
	"image/color"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"os"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/config"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/dbus"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/theme"
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

	// Theme-derived colors (from the resolved GTK theme, else Libadwaita defaults)
	panelColor     color.NRGBA // login panel / dropdown background
	secondaryColor color.NRGBA // secondary buttons and selectors
	wallpaper      *image.Image

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

// NewWindow initializes the presentation state with standard system fonts and greeter config
func NewWindow(sessions []dbus.Session) *Window {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))

	// Load INI configuration (wallpaper, GTK theme name, etc.)
	cfg, _ := config.LoadConfig("/etc/lightdm/lightdm-greeter-ganapati.conf")

	// Resolve the named GTK theme (set by Stylix or manually) so the greeter
	// matches the desktop's actual theme instead of a hardcoded palette.
	pal := theme.Resolve(cfg.ThemeName)

	// Set colors based on the resolved theme when available, otherwise fall
	// back to the Libadwaita palette (dark/light per configuration).
	bg := color.NRGBA{R: 0xFA, G: 0xFA, B: 0xFA, A: 0xFF} // Light Theme Background
	fg := color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF} // Dark Text
	if cfg.DarkTheme {
		bg = color.NRGBA{R: 0x24, G: 0x24, B: 0x24, A: 0xFF} // Libadwaita Dark Background
		fg = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF} // Pure White Text
	}
	accent := color.NRGBA{R: 0x35, G: 0x84, B: 0xE4, A: 0xFF} // Libadwaita Blue Accent
	accentFg := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	if pal.Found {
		if !isZero(pal.Background) {
			bg = pal.Background
		}
		if !isZero(pal.Foreground) {
			fg = pal.Foreground
		}
		if !isZero(pal.SelectedBG) {
			accent = pal.SelectedBG
		}
		if !isZero(pal.SelectedFG) {
			accentFg = pal.SelectedFG
		}
	}

	th.Palette.Bg = bg
	th.Palette.Fg = fg
	th.Palette.ContrastBg = accent
	th.Palette.ContrastFg = accentFg

	// Panel and secondary surfaces derive from the theme's base/surface color.
	panel := color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xFF} // Libadwaita Panel Gray
	secondary := color.NRGBA{R: 0x3E, G: 0x3E, B: 0x3E, A: 0xFF}
	if pal.Found {
		if !isZero(pal.Base) {
			panel = pal.Base
			secondary = shade(pal.Base, -12)
		}
	}

	w := &Window{
		theme:          th,
		sessions:       sessions,
		sessionClicks:  make([]widget.Clickable, len(sessions)),
		usernameEditor: widget.Editor{SingleLine: true, Submit: true},
		passwordEditor: widget.Editor{SingleLine: true, Submit: true, Mask: '*'},
		panelColor:     panel,
		secondaryColor: secondary,
	}

	// Load the wallpaper image (from Stylix or manual config) so it can be
	// painted behind the login panel.
	if img, err := loadImage(cfg.Background); err == nil {
		w.wallpaper = &img
	}

	return w
}

// loadImage decodes a PNG or JPEG image from disk.
func loadImage(path string) (image.Image, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

// isZero reports whether a color is the NRGBA zero value, which we treat as
// "not provided by the theme".
func isZero(c color.NRGBA) bool { return c == color.NRGBA{} }

// shade darkens (negative delta) or lightens (positive delta) a color by the
// given channel amount, clamping to valid byte range.
func shade(c color.NRGBA, delta int) color.NRGBA {
	clamp := func(v int) uint8 {
		if v < 0 {
			return 0
		}
		if v > 0xff {
			return 0xff
		}
		return uint8(v)
	}
	return color.NRGBA{
		R: clamp(int(c.R) + delta),
		G: clamp(int(c.G) + delta),
		B: clamp(int(c.B) + delta),
		A: c.A,
	}
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
