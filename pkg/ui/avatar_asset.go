package ui

import _ "embed"

// fallbackAvatarPNG is the Sanatana Linux icon, shown when the active icon
// theme provides no raster default-user avatar. Modern Adwaita ships the
// avatar icon as SVG only, which the Gio renderer cannot rasterize, so a
// bundled PNG keeps the avatar meaningful on every system.
//
//go:embed avatar-fallback.png
var fallbackAvatarPNG []byte
