package ui

/*
#cgo pkg-config: x11 xrandr
#include <X11/Xlib.h>
#include <X11/extensions/Xrandr.h>
#include <stdlib.h>
*/
import "C"
import "unsafe"

// Monitor describes a single physical display as reported by XRandR.
type Monitor struct {
	Name    string // eDP-1, HDMI-1, DP-1, ...
	X, Y    int    // origin relative to the root window
	W, H    int    // dimensions
	Primary bool   // the XRandR primary output
	IsEDP   bool   // built-in panel (output name begins with "eDP")
}

// boundingBox returns the root-window rectangle that covers every monitor.
func boundingBox(mons []Monitor) (x, y, w, h int) {
	if len(mons) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY := mons[0].X, mons[0].Y
	maxX, maxY := mons[0].X+mons[0].W, mons[0].Y+mons[0].H
	for _, m := range mons[1:] {
		if m.X < minX {
			minX = m.X
		}
		if m.Y < minY {
			minY = m.Y
		}
		if m.X+m.W > maxX {
			maxX = m.X + m.W
		}
		if m.Y+m.H > maxY {
			maxY = m.Y + m.H
		}
	}
	return minX, minY, maxX - minX, maxY - minY
}

// choosePrimary picks the monitor that should initially show the login
// dialog: an eDP (built-in) panel is preferred, then the XRandR primary,
// then the first connected monitor.
func choosePrimary(mons []Monitor) int {
	for i, m := range mons {
		if m.IsEDP {
			return i
		}
	}
	for i, m := range mons {
		if m.Primary {
			return i
		}
	}
	return 0
}

// monitorAt returns the index of the monitor containing (x, y), or -1.
func monitorAt(mons []Monitor, x, y int) int {
	for i, m := range mons {
		if x >= m.X && x < m.X+m.W && y >= m.Y && y < m.Y+m.H {
			return i
		}
	}
	return -1
}

// monitors enumerates all connected displays via XRandR with their geometry.
// It returns nil, nil when XRandR is unavailable so callers can fall back
// to a single default screen.
func monitors() ([]Monitor, error) {
	d := C.XOpenDisplay(nil)
	if d == nil {
		return nil, nil
	}
	defer C.XCloseDisplay(d)
	screen := C.XDefaultScreen(d)
	root := C.XRootWindow(d, screen)

	var nmon C.int
	monInfos := C.XRRGetMonitors(d, root, C.True, &nmon)
	if monInfos == nil || int(nmon) == 0 {
		if monInfos != nil {
			C.XRRFreeMonitors(monInfos)
		}
		return nil, nil
	}
	defer C.XRRFreeMonitors(monInfos)

	// Primary output atom (may be None). Compare each monitor's name atom
	// against it to flag the primary display.
	primAtom := C.XRRGetOutputPrimary(d, root)

	infos := unsafe.Slice(monInfos, int(nmon))

	var mons []Monitor
	for i := 0; i < int(nmon); i++ {
		mi := infos[i]
		if mi.width == 0 || mi.height == 0 {
			continue
		}
		name := atomName(d, mi.name)
		isPrimary := primAtom != 0 && primAtom == mi.name
		mons = append(mons, Monitor{
			Name:    name,
			X:       int(mi.x),
			Y:       int(mi.y),
			W:       int(mi.width),
			H:       int(mi.height),
			Primary: isPrimary,
			IsEDP:   len(name) >= 3 && name[:3] == "eDP",
		})
	}
	return mons, nil
}

// atomName resolves an X11 Atom to its string name.
func atomName(d *C.Display, a C.Atom) string {
	s := C.XGetAtomName(d, a)
	if s == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(s))
	return C.GoString(s)
}
