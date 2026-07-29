package config

import (
	"bufio"
	"os"
	"strings"
)

// Config holds the greeter configuration settings (compatible with Slick / Elephant Greeter INI specs)
type Config struct {
	Background     string
	ThemeName      string
	IconThemeName  string
	DarkTheme      bool
	DefaultSession string
}

// DefaultConfig returns sensible defaults matching the Libadwaita aesthetic
func DefaultConfig() *Config {
	return &Config{
		Background:     "",
		ThemeName:      "Adwaita-dark",
		IconThemeName:  "Adwaita",
		DarkTheme:      true,
		DefaultSession: "awesome",
	}
}

// LoadConfig loads configuration from an INI file (e.g. /etc/lightdm/lightdm-greeter-ganapati.conf)
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	file, err := os.Open(path)
	if err != nil {
		// If file doesn't exist, return defaults gracefully
		return cfg, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])
		// Remove surrounding quotes if any
		val = strings.Trim(val, "\"'")

		switch currentSection {
		case "gtk":
			switch key {
			case "gtk-theme-name", "theme-name":
				cfg.ThemeName = val
			case "gtk-icon-theme-name", "icon-theme-name":
				cfg.IconThemeName = val
			case "gtk-application-prefer-dark-theme", "dark-theme":
				cfg.DarkTheme = strings.ToLower(val) == "true" || val == "1"
			}
		case "greeter":
			switch key {
			case "background", "wallpaper":
				cfg.Background = val
			case "default-session":
				cfg.DefaultSession = val
			}
		}
	}

	return cfg, scanner.Err()
}
