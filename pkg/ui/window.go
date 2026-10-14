//go:build !headless

package ui

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
	"os"
	"runtime/debug"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/config"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/dbus"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/theme"
	"golang.org/x/image/draw"
)

// UIActionHandler defines actions triggered by user interaction in the UI
type UIActionHandler interface {
	OnAuthenticate(username string)
	OnRespond(password string)
	OnCancel()
	OnStartSession(sessionID string)
	// OnError reports a failed D-Bus operation to the UI so it can be
	// surfaced to the user instead of failing silently.
	OnError(message string)
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
	mutedColor     color.NRGBA // secondary/hint text (clock, subtitle)
	avatarColor    color.NRGBA // profile avatar circle fill
	errorColor     color.NRGBA // error status text
	successColor   color.NRGBA // success status text

	// Raster images are wrapped in paint.ImageOp values created exactly once
	// and reused every frame. Creating an ImageOp inside the draw calls would
	// allocate a fresh GPU cache handle (and a full RGBA copy) on every frame,
	// forcing a texture re-upload per frame and exhausting memory.
	wallpaperOp  paint.ImageOp
	hasWallpaper bool
	avatarOp     paint.ImageOp
	hasAvatar    bool

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
	// Neutral gray accent (replaces the Libadwaita blue): used for the
	// avatar, the primary action button and selection highlights.
	accent := color.NRGBA{R: 0x6C, G: 0x6C, B: 0x6C, A: 0xFF}
	accentFg := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	if pal.Found {
		if !isZero(pal.Background) {
			bg = pal.Background
		}
		if !isZero(pal.Foreground) {
			fg = pal.Foreground
		}
		// The accent is deliberately a fixed neutral gray rather than the
		// theme's selected_background_color; the default is a subdued gray
		// #6c6c6c.
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

	// Adaptive secondary colors derived from the foreground so they remain
	// readable on any theme: muted text is fg at ~55% opacity, the avatar
	// reuses the accent, and error/success pick a shade that contrasts with
	// the background.
	muted := fg
	muted.A = 0x8C

	errorColor := color.NRGBA{R: 0xFF, G: 0x55, B: 0x55, A: 0xFF}
	successColor := accent
	if !isDark(bg) {
		errorColor = color.NRGBA{R: 0xC0, G: 0x22, B: 0x22, A: 0xFF}
	}

	w := &Window{
		theme:          th,
		sessions:       sessions,
		sessionClicks:  make([]widget.Clickable, len(sessions)),
		usernameEditor: widget.Editor{SingleLine: true, Submit: true},
		passwordEditor: widget.Editor{SingleLine: true, Submit: true, Mask: '*'},
		panelColor:     panel,
		secondaryColor: secondary,
		mutedColor:     muted,
		avatarColor:    accent,
		errorColor:     errorColor,
		successColor:   successColor,
	}

	// Load the wallpaper image (from Stylix or manual config) so it can be
	// painted behind the login panel. The image is downscaled to fit within
	// maxWallpaperDim so GPU texture uploads never exceed driver limits
	// (huge 8K wallpapers would otherwise fail or grind on software GL).
	if img, err := loadImage(cfg.Background); err == nil {
		w.wallpaperOp = newImageOp(img)
		w.hasWallpaper = true
	}

	// Load the default user avatar. Prefer the active icon theme's raster
	// avatar icon; fall back to the bundled Sanatana Linux icon so the
	// avatar is a real image rather than a bare colored circle.
	if img, err := loadAvatar(cfg.IconThemeName); err == nil {
		w.avatarOp = newImageOp(img)
		w.hasAvatar = true
	}

	// Decoding a high-resolution wallpaper allocates a large intermediate
	// buffer that is not needed once the image has been downscaled. Return
	// that memory to the OS immediately so the greeter's resident set stays
	// small for its whole lifetime (the display manager may impose a tight
	// memory limit).
	debug.FreeOSMemory()

	return w
}

// newImageOp wraps a decoded image in a reusable paint.ImageOp. The op must be
// created once and reused across frames: paint.NewImageOp allocates a fresh
// GPU cache handle (and, for non-RGBA sources, a full RGBA copy), so calling
// it inside the draw path would re-upload the texture every frame.
func newImageOp(img image.Image) paint.ImageOp {
	op := paint.NewImageOp(img)
	op.Filter = paint.FilterLinear
	return op
}

// maxWallpaperDim is the largest dimension (in pixels) a wallpaper is scaled
// to before upload. Wallpapers larger than this are downscaled to keep GPU
// memory use bounded and rendering fast.
const maxWallpaperDim = 2560

// isDark reports whether a color reads as dark (used to pick readable
// error/status text).
func isDark(c color.NRGBA) bool {
	// Relative luminance (Rec. 709 coefficients), normalized to 0..255.
	lum := 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
	return lum < 128
}

// maxAvatarDim caps the avatar image dimension so the avatar texture stays
// small.
const maxAvatarDim = 256

// loadAvatar returns the default user avatar image. It prefers the active
// icon theme's raster (PNG) avatar icon and falls back to the bundled
// Sanatana Linux icon, so the avatar is always a real image.
func loadAvatar(iconTheme string) (image.Image, error) {
	if p := theme.AvatarPath(iconTheme); p != "" {
		if img, err := loadImageMax(p, maxAvatarDim); err == nil {
			return img, nil
		}
	}
	img, _, err := image.Decode(bytes.NewReader(fallbackAvatarPNG))
	if err != nil {
		return nil, err
	}
	// The bundled icon is large; downscale it so the avatar texture stays
	// small.
	return fitImage(img, maxAvatarDim), nil
}

// loadImage decodes a PNG or JPEG image from disk, downscaling it to fit
// within maxWallpaperDim so GPU texture uploads stay within driver limits.
func loadImage(path string) (image.Image, error) {
	return loadImageMax(path, maxWallpaperDim)
}

// loadImageMax decodes an image from disk and downscales it so its largest
// dimension is at most maxDim.
func loadImageMax(path string, maxDim int) (image.Image, error) {
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
	return fitImage(img, maxDim), nil
}

// fitImage downscales img (preserving aspect ratio) so that its largest
// dimension is at most maxDim. Images already within the limit are returned
// unchanged.
func fitImage(img image.Image, maxDim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return img
	}
	scale := float64(maxDim) / float64(max(w, h))
	nw := int(float64(w) * scale)
	nh := int(float64(h) * scale)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
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
				// Deliberately do NOT clear statusMsg: a failure message
				// emitted just before a retry prompt must stay visible.
			case dbus.EventShowMessage:
				w.statusMsg = ev.Text
				w.isError = ev.Param == 1
			case dbus.EventAuthComplete:
				w.isAuthenticating = false
				w.needPassword = false
				if ev.Param != 0 {
					// The greeter protocol reports a non-zero
					// END_AUTHENTICATION result for any failed attempt
					// (unknown user or wrong password). Surface it.
					w.statusMsg = "Authentication failed — incorrect username or password."
					w.isError = true
				} else if w.statusMsg == "" {
					w.statusMsg = "Logged in successfully. Starting session..."
					w.isError = false
					if w.currentSess < len(w.sessions) {
						handler.OnStartSession(w.sessions[w.currentSess].ID)
					}
				}
			case dbus.EventReset:
				// The daemon resets the greeter (e.g. after a failed login).
				// Clear in-progress auth state and the credential editors,
				// but preserve any status/error message so the user can see
				// why the last attempt failed.
				w.isAuthenticating = false
				w.needPassword = false
				w.usernameEditor.SetText("")
				w.passwordEditor.SetText("")
			}
		}
	}()

	// Run Gio window loop in a goroutine per official documentation pattern, and block with app.Main() on main thread
	go func() {
		window := new(app.Window)
		window.Option(app.Title("LightDM Greeter Ganapati"))
		// The greeter must cover the entire display at its native
		// resolution. It runs without a window manager, so Gio's
		// Fullscreen option — an EWMH `_NET_WM_STATE_FULLSCREEN` request —
		// is ignored, and the window would stay at its 800x600 default.
		// Size the window explicitly to the X screen instead, falling back
		// to the Fullscreen hint when the screen size is unavailable.
		if sw, sh := screenSize(); sw > 0 && sh > 0 {
			window.Option(app.Windowed.Option())
			window.Option(func(_ unit.Metric, cnf *app.Config) {
				cnf.Size = image.Point{X: sw, Y: sh}
			})
		} else {
			window.Option(app.Fullscreen.Option())
		}

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

// handleActions processes button clicks and editor submissions
func (w *Window) handleActions(gtx layout.Context, handler UIActionHandler) {
	// Enter key in the username field starts authentication.
	if !w.isAuthenticating {
		if text, submitted := editorSubmitted(gtx, &w.usernameEditor); submitted && text != "" {
			w.startAuth(text, handler)
		}
	}

	// Login button click — same path as Enter.
	if w.loginClick.Clicked(gtx) {
		if !w.isAuthenticating {
			user := w.usernameEditor.Text()
			if user != "" {
				w.startAuth(user, handler)
			}
		} else if w.needPassword {
			pass := w.passwordEditor.Text()
			handler.OnRespond(pass)
		}
	}

	// Enter key in the password field sends the response.
	if w.isAuthenticating && w.needPassword {
		if text, submitted := editorSubmitted(gtx, &w.passwordEditor); submitted {
			handler.OnRespond(text)
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

// editorSubmitted processes pending editor events and reports whether a
// SubmitEvent (Enter key with Submit enabled) occurred, returning the
// submitted text. Events are consumed so the editor's layout won't reprocess
// them.
func editorSubmitted(gtx layout.Context, ed *widget.Editor) (string, bool) {
	for {
		ev, ok := ed.Update(gtx)
		if !ok {
			return "", false
		}
		if se, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			return se.Text, true
		}
	}
}

// startAuth begins authentication for the given username. The UI switches to
// the password prompt immediately (optimistic) so the user can type their
// password without waiting for the D-Bus ShowPrompt round-trip; the
// orchestrator reports failures via OnError.
func (w *Window) startAuth(user string, handler UIActionHandler) {
	w.isAuthenticating = true
	w.needPassword = true
	w.promptText = "Password:"
	w.statusMsg = ""
	// Always start a fresh password; a stale value from a previous attempt
	// would be appended to and guarantee a wrong password.
	w.passwordEditor.SetText("")
	handler.OnAuthenticate(user)
}

// SetStatus displays a status message in the login panel, colored red when
// isError is true. It is called by the orchestrator to surface D-Bus
// operation failures. Called from the frame goroutine (via handleActions)
// and from the D-Bus event goroutine, matching the existing direct-field
// pattern used for statusMsg elsewhere.
func (w *Window) SetStatus(msg string, isError bool) {
	w.statusMsg = msg
	w.isError = isError
}
