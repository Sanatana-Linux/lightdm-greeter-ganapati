# LightDM Elephant Greeter (Go Rewrite)

A modern, graphical login manager/greeter rewritten in Go, optimized for lightning-fast performance, Wayland & X11 support, and a polished Libadwaita-inspired aesthetic.

## Features
- **Fast & Minimal**: Written in Go with minimal runtime dependencies.
- **Protocol Agnostic**: Supports both Wayland and X11 session selection natively.
- **Libadwaita Styling**: Adopts a minimalist, modern dark aesthetic matching current GNOME/Libadwaita standards.
- **NixOS Native**: Packaged for NixOS with built-in flake-based development shells and virtual machine testing.

## Prerequisites
- Nix package manager (with flakes enabled)
- QEMU (for local VM testing)

## Setup & Building

Initialize the repository:
```bash
git clone <url>
cd LightDM_Elephant_Fork
```

Use `just` for automated development tasks:
- `just test`: Run the full unit and D-Bus state-machine test suite.
- `just build`: Compile the standard graphical binary.
- `just build-headless`: Compile a text-based headless binary (for CI/servers).
- `just vm-build`: Build the NixOS QEMU VM testing suite.
- `just vm-run`: Run the QEMU VM to test greeter integration live.

## NixOS Configuration

To register this greeter in your `configuration.nix`:

```nix
services.xserver.displayManager.lightdm = {
  enable = true;
  greeters.gtk.enable = false;
  extraConfig = ''
    [Seat:*]
    greeter-session=lightdm-elephant-greeter
  '';
};

# Ensure the greeter package is available on the system
environment.systemPackages = [
  # Reference the package from your flake inputs or local folder
  config.packages.x86_64-linux.default
];
```

## Styling
The greeter uses Libadwaita-inspired default colors:
- Background: `#242424`
- Accent: `#3584e4` (Libadwaita Blue)
- Panel Grey: `#303030`
- Text: `#ffffff`

Users can customize the configuration via `/etc/lightdm/elephant-greeter.conf` if needed, matching the original Elephant Greeter's configuration scheme.
