package theme

import (
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestParseHex(t *testing.T) {
	tests := []struct {
		in   string
		want color.NRGBA
		ok   bool
	}{
		{"#1c1c1c", color.NRGBA{R: 0x1c, G: 0x1c, B: 0x1c, A: 0xff}, true},
		{"#fff", color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, true},
		{"#3584e4", color.NRGBA{R: 0x35, G: 0x84, B: 0xe4, A: 0xff}, true},
		{"#12", color.NRGBA{}, false},
		{"#gggggg", color.NRGBA{}, false},
		{"", color.NRGBA{}, false},
	}
	for _, tt := range tests {
		got, ok := parseHex(tt.in)
		if ok != tt.ok {
			t.Errorf("parseHex(%q) ok=%v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("parseHex(%q)=%+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseFunc(t *testing.T) {
	tests := []struct {
		in   string
		want color.NRGBA
		ok   bool
	}{
		{"rgb(28, 28, 28)", color.NRGBA{R: 28, G: 28, B: 28, A: 0xff}, true},
		{"rgba(255, 255, 255, 0.5)", color.NRGBA{R: 255, G: 255, B: 255, A: 127}, true},
		{"rgb(50%, 0%, 0%)", color.NRGBA{R: 127, G: 0, B: 0, A: 0xff}, true},
		{"mix(#1c1c1c,#2c2c2c,0.4)", color.NRGBA{}, false},
		{"alpha(#fff, 0.8)", color.NRGBA{}, false},
		{"rgb(notanumber,1,1)", color.NRGBA{}, false},
	}
	for _, tt := range tests {
		got, ok := parseFunc(tt.in)
		if ok != tt.ok {
			t.Errorf("parseFunc(%q) ok=%v, want %v", tt.in, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("parseFunc(%q)=%+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestResolveFindsTheme(t *testing.T) {
	// Install a fake theme into a temp dir and verify Resolve picks it up.
	dir := t.TempDir()
	themeDir := filepath.Join(dir, "test-theme", "gtk-3.0")
	writeFile(t, filepath.Join(themeDir, "gtk.css"), `
/* comment */
@define-color theme_bg_color #101010;
@define-color theme_fg_color #e0e0e0;
@define-color theme_base_color #202020;
@define-color theme_selected_bg_color #00aaff;
@define-color theme_selected_fg_color #ffffff;
@define-color insensitive_bg_color mix(#101010,#202020,0.4);
@define-color borders rgba(255, 255, 255, 0.12);
`)

	old := searchDirsFn
	searchDirsFn = func() []string { return []string{dir} }
	defer func() { searchDirsFn = old }()

	pal := Resolve("test-theme")
	if !pal.Found {
		t.Fatal("Resolve did not find the test theme")
	}
	if pal.Background != (color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff}) {
		t.Errorf("Background=%+v", pal.Background)
	}
	if pal.Foreground != (color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}) {
		t.Errorf("Foreground=%+v", pal.Foreground)
	}
	if pal.SelectedBG != (color.NRGBA{R: 0x00, G: 0xaa, B: 0xff, A: 0xff}) {
		t.Errorf("SelectedBG=%+v", pal.SelectedBG)
	}
}

func TestResolveMissingTheme(t *testing.T) {
	if pal := Resolve("does-not-exist-theme-xyz"); pal.Found {
		t.Error("Resolve should not find a nonexistent theme")
	}
}
