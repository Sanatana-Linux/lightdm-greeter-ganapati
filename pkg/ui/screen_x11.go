//go:build !headless

package ui

/*
#cgo pkg-config: x11
#include <X11/Xlib.h>
#include <stdlib.h>
*/
import "C"

// screenSize reports the pixel dimensions of the default X display's root
// window, or (0, 0) when the display cannot be opened.
//
// The greeter runs under LightDM without a window manager, so Gio's
// Fullscreen option — which merely sends the EWMH
// `_NET_WM_STATE_FULLSCREEN` client message — is silently ignored and the
// window stays at Gio's 800x600 default. Sizing the window explicitly to the
// X screen is what actually makes it cover the whole display.
func screenSize() (int, int) {
	d := C.XOpenDisplay(nil)
	if d == nil {
		return 0, 0
	}
	defer C.XCloseDisplay(d)
	screen := C.XDefaultScreen(d)
	return int(C.XDisplayWidth(d, screen)), int(C.XDisplayHeight(d, screen))
}
