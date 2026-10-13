package theme

import (
	"os"
	"path/filepath"
	"testing"
)

// withIconDirs points the icon resolver at a single root for the test's
// duration.
func withIconDirs(t *testing.T, root string) {
	t.Helper()
	old := iconDirsFn
	iconDirsFn = func() []string { return []string{root} }
	t.Cleanup(func() { iconDirsFn = old })
}

// TestResolveIconPrefersRasterAndFallsBackToLegacy checks that an SVG-only
// primary theme is skipped in favour of a PNG in the legacy theme, and that
// the largest available size wins.
func TestResolveIconPrefersRasterAndFallsBackToLegacy(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Adwaita", "scalable", "status", "avatar-default.svg"))
	writeFile(t, filepath.Join(root, "AdwaitaLegacy", "32x32", "status", "avatar-default.png"))
	writeFile(t, filepath.Join(root, "AdwaitaLegacy", "48x48", "status", "avatar-default.png"))

	withIconDirs(t, root)

	want := filepath.Join(root, "AdwaitaLegacy", "48x48", "status", "avatar-default.png")
	if got := ResolveIcon("Adwaita", "avatar-default"); got != want {
		t.Fatalf("ResolveIcon = %q, want %q", got, want)
	}
	if got := AvatarPath("Adwaita"); got != want {
		t.Fatalf("AvatarPath = %q, want %q", got, want)
	}
}

// TestResolveIconPrefersNamedTheme verifies the requested theme is searched
// before fallbacks.
func TestResolveIconPrefersNamedTheme(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Papirus", "64x64", "status", "avatar-default.png"))
	writeFile(t, filepath.Join(root, "AdwaitaLegacy", "48x48", "status", "avatar-default.png"))

	withIconDirs(t, root)

	want := filepath.Join(root, "Papirus", "64x64", "status", "avatar-default.png")
	if got := ResolveIcon("Papirus", "avatar-default"); got != want {
		t.Fatalf("ResolveIcon = %q, want %q", got, want)
	}
}

// TestResolveIconMissing returns "" when no raster icon exists.
func TestResolveIconMissing(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Adwaita", "scalable", "status", "avatar-default.svg"))
	withIconDirs(t, root)

	if got := ResolveIcon("Adwaita", "avatar-default"); got != "" {
		t.Fatalf("ResolveIcon = %q, want empty", got)
	}
	if got := AvatarPath("Adwaita"); got != "" {
		t.Fatalf("AvatarPath = %q, want empty", got)
	}
}
