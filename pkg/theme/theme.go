// Package theme resolves a named GTK theme on the system and extracts its
// color palette so a native (non-GTK) renderer — such as the Gio greeter —
// can visually match the active desktop theme, including themes applied by
// Stylix.
package theme

import (
	"bufio"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Palette carries the meaningful color roles extracted from a GTK theme.
// Values are only valid when Found is true.
type Palette struct {
	Found bool

	Background color.NRGBA // theme_bg_color — window background
	Foreground color.NRGBA // theme_fg_color — primary text
	Base       color.NRGBA // theme_base_color — panel/card surface
	SelectedBG color.NRGBA // theme_selected_bg_color — accent / focus
	SelectedFG color.NRGBA // theme_selected_fg_color — text on accent
}

// searchDirs are the standard locations where GTK themes are installed,
// in priority order. NixOS store paths appear early because that is where
// system-managed (and Stylix-applied) themes are actually installed.
//
// searchDirsFn is a package-level hook so tests can substitute a fixed set
// of directories.
var searchDirsFn = searchDirs

func searchDirs() []string {
	var dirs []string
	add := func(d string) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}

	add("/run/current-system/sw/share/themes")

	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".themes"))
		add(filepath.Join(home, ".local", "share", "themes"))
		add(filepath.Join(home, ".nix-profile", "share", "themes"))
	}

	if dataDirs := os.Getenv("XDG_DATA_DIRS"); dataDirs != "" {
		for _, dir := range filepath.SplitList(dataDirs) {
			add(filepath.Join(dir, "themes"))
		}
	}

	// Standard non-Nix locations last, as a fallback.
	add("/usr/share/themes")
	add("/usr/local/share/themes")

	return dirs
}

// ResolveDir returns the directory containing a theme of the given name, or
// "" if the theme cannot be found in any standard search location.
func ResolveDir(name string) string {
	if name == "" {
		return ""
	}
	for _, dir := range searchDirsFn() {
		themeDir := filepath.Join(dir, name)
		if fi, err := os.Stat(themeDir); err == nil && fi.IsDir() {
			return themeDir
		}
	}
	return ""
}

// Resolve loads the color palette for the named theme. It returns a zero
// Palette (Found == false) when the theme cannot be located or contains no
// usable GTK 3 color definitions.
func Resolve(name string) Palette {
	dir := ResolveDir(name)
	if dir == "" {
		return Palette{}
	}
	return parseCSS(filepath.Join(dir, "gtk-3.0", "gtk.css"))
}

// parseCSS scans a GTK 3 stylesheet for @define-color declarations and maps
// the conventional color roles into a Palette.
func parseCSS(path string) Palette {
	f, err := os.Open(path)
	if err != nil {
		return Palette{}
	}
	defer f.Close()

	pal := Palette{Found: true}
	roles := make(map[string]color.NRGBA)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "@define-color") {
			continue
		}
		// @define-color NAME VALUE;
		rest := strings.TrimSpace(strings.TrimPrefix(line, "@define-color"))
		rest = strings.TrimSuffix(rest, ";")
		parts := strings.Fields(rest)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		value := strings.Join(parts[1:], " ")
		if c, ok := parseColor(value); ok {
			roles[name] = c
		}
	}

	// Map the conventional roles; fall back to the un-prefixed variants when
	// a theme only defines those.
	pal.Background = pick(roles, "theme_bg_color", "bg_color")
	pal.Foreground = pick(roles, "theme_fg_color", "fg_color")
	pal.Base = pick(roles, "theme_base_color", "base_color")
	pal.SelectedBG = pick(roles, "theme_selected_bg_color", "selected_bg_color")
	pal.SelectedFG = pick(roles, "theme_selected_fg_color", "selected_fg_color")

	// A theme that defined no recognizable roles is treated as not found.
	if pal.Background.A == 0 && pal.Foreground.A == 0 && pal.Base.A == 0 &&
		pal.SelectedBG.A == 0 && pal.SelectedFG.A == 0 {
		return Palette{}
	}
	return pal
}

// pick returns the first color found among the given role names, or a zero
// color when none are defined.
func pick(roles map[string]color.NRGBA, names ...string) color.NRGBA {
	for _, n := range names {
		if c, ok := roles[n]; ok {
			return c
		}
	}
	return color.NRGBA{}
}

// parseColor converts a CSS color value to NRGBA. It supports #RGB, #RRGGBB,
// rgb(r, g, b) and rgba(r, g, b, a). Unparseable values (mix(), alpha(),
// currentColor, named colors) return false.
func parseColor(v string) (color.NRGBA, bool) {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "mix(") || strings.HasPrefix(v, "alpha(") ||
		v == "currentColor" || v == "transparent" || v == "inherit" {
		return color.NRGBA{}, false
	}

	if strings.HasPrefix(v, "#") {
		return parseHex(v)
	}

	if strings.HasPrefix(v, "rgb") {
		return parseFunc(v)
	}

	// Named CSS colors are deliberately not supported; a theme relying on
	// them is unusual and their hex equivalents are what matter for us.
	return color.NRGBA{}, false
}

// parseHex parses #RGB or #RRGGBB forms.
func parseHex(v string) (color.NRGBA, bool) {
	h := strings.TrimPrefix(v, "#")
	switch len(h) {
	case 3:
		r, e1 := strconv.ParseUint(string(h[0])+string(h[0]), 16, 8)
		g, e2 := strconv.ParseUint(string(h[1])+string(h[1]), 16, 8)
		b, e3 := strconv.ParseUint(string(h[2])+string(h[2]), 16, 8)
		if e1 != nil || e2 != nil || e3 != nil {
			return color.NRGBA{}, false
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xFF}, true
	case 6:
		r, e1 := strconv.ParseUint(h[0:2], 16, 8)
		g, e2 := strconv.ParseUint(h[2:4], 16, 8)
		b, e3 := strconv.ParseUint(h[4:6], 16, 8)
		if e1 != nil || e2 != nil || e3 != nil {
			return color.NRGBA{}, false
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 0xFF}, true
	}
	return color.NRGBA{}, false
}

// parseFunc parses rgb(r, g, b) and rgba(r, g, b, a) forms. Component values
// may be integers (0-255) or percentages.
func parseFunc(v string) (color.NRGBA, bool) {
	open := strings.IndexByte(v, '(')
	close := strings.LastIndexByte(v, ')')
	if open == -1 || close == -1 || close <= open {
		return color.NRGBA{}, false
	}
	body := strings.TrimSpace(v[open+1 : close])
	parts := strings.Split(body, ",")
	if len(parts) < 3 {
		return color.NRGBA{}, false
	}

	r, ok := cssComponent(parts[0])
	if !ok {
		return color.NRGBA{}, false
	}
	g, ok := cssComponent(parts[1])
	if !ok {
		return color.NRGBA{}, false
	}
	b, ok := cssComponent(parts[2])
	if !ok {
		return color.NRGBA{}, false
	}

	alpha := uint8(0xFF)
	if len(parts) > 3 {
		a := strings.TrimSpace(parts[3])
		if strings.HasSuffix(a, "%") {
			pct, err := strconv.ParseFloat(strings.TrimSuffix(a, "%"), 64)
			if err != nil {
				return color.NRGBA{}, false
			}
			alpha = uint8(pct / 100.0 * 255.0)
		} else if f, err := strconv.ParseFloat(a, 64); err == nil {
			alpha = uint8(f * 255.0)
		} else {
			return color.NRGBA{}, false
		}
	}

	return color.NRGBA{R: r, G: g, B: b, A: alpha}, true
}

// cssComponent parses a single 0-255 integer or 0-100% percentage channel.
func cssComponent(s string) (uint8, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		pct, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64)
		if err != nil {
			return 0, false
		}
		return uint8(pct / 100.0 * 255.0), true
	}
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0, false
	}
	return uint8(n), true
}
