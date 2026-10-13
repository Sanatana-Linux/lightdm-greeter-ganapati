package theme

import (
	"os"
	"path/filepath"
	"strings"
)

// iconDirsFn is a package-level hook so tests can substitute a fixed set of
// icon directories (mirrors searchDirsFn for themes).
var iconDirsFn = iconDirs

// iconDirs returns the standard icon-theme search locations in priority order.
// NixOS keeps system icons under /run/current-system/sw/share instead of
// /usr/share.
func iconDirs() []string {
	var dirs []string
	add := func(d string) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}

	add("/run/current-system/sw/share/icons")

	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".icons"))
		add(filepath.Join(home, ".local", "share", "icons"))
		add(filepath.Join(home, ".nix-profile", "share", "icons"))
	}

	if dataDirs := os.Getenv("XDG_DATA_DIRS"); dataDirs != "" {
		for _, dir := range filepath.SplitList(dataDirs) {
			add(filepath.Join(dir, "icons"))
		}
	}

	add("/usr/share/icons")
	add("/usr/local/share/icons")

	return dirs
}

// iconSizes lists the standard icon size directories in descending order, so
// the largest available raster icon is chosen first.
var iconSizes = []string{
	"512x512", "256x256", "128x128", "96x96", "64x64",
	"48x48", "32x32", "24x24", "22x22", "16x16",
}

// iconContexts lists the icon subdirectories that commonly hold an avatar or
// user icon.
var iconContexts = []string{
	"status", "apps", "places", "mimetypes", "actions",
	"devices", "categories", "emotes", "",
}

// ResolveIcon returns the path to an installed raster (PNG) icon by name for
// the given icon theme, preferring the largest available size. SVG icons are
// deliberately ignored: the Gio renderer cannot rasterize SVG.
//
// The search falls back to the AdwaitaLegacy and hicolor themes (which ship
// PNG variants when the primary theme is SVG-only) and finally to any theme
// directory providing a matching raster icon.
func ResolveIcon(iconTheme, name string) string {
	want := name + ".png"

	// Theme directories to try, in priority order.
	var themes []string
	if iconTheme != "" {
		themes = append(themes, iconTheme)
	}
	for _, t := range []string{"AdwaitaLegacy", "hicolor", "Adwaita"} {
		if t != iconTheme {
			themes = append(themes, t)
		}
	}

	for _, dir := range iconDirsFn() {
		for _, theme := range themes {
			root := filepath.Join(dir, theme)
			if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
				continue
			}
			for _, size := range iconSizes {
				for _, ctx := range iconContexts {
					var p string
					if ctx == "" {
						p = filepath.Join(root, size, want)
					} else {
						p = filepath.Join(root, size, ctx, want)
					}
					if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
						return p
					}
				}
			}
			// Flat layout: <theme>/<name>.png
			flat := filepath.Join(root, want)
			if fi, err := os.Stat(flat); err == nil && !fi.IsDir() {
				return flat
			}
		}
	}
	return ""
}

// avatarIconNames are the icon names that represent a default/placeholder user
// avatar, tried in order.
var avatarIconNames = []string{
	"avatar-default",
	"avatar-default-symbolic",
	"user-info",
	"user-avatar",
	"stock_person",
}

// AvatarPath returns the path to a raster default-user avatar for the given
// icon theme, or "" when the theme provides none (callers then fall back to a
// bundled asset).
func AvatarPath(iconTheme string) string {
	for _, n := range avatarIconNames {
		if p := ResolveIcon(iconTheme, n); p != "" {
			return p
		}
	}
	return ""
}

// used to keep strings imported if the file evolves; documentation helper.
var _ = strings.TrimSpace
