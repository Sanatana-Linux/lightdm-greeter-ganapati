# LightDM Elephant Greeter (Go Rewrite)

A premium, graphical login manager/greeter rewritten in Go, designed for lightning-fast performance, seamless Wayland & X11 support, and a polished, Libadwaita-inspired minimalist aesthetic.

## Features
- **Fast & Minimal**: Written in Go, compile-to-static-binary for NixOS/system safety.
- **Protocol Agnostic**: Supports both Wayland (via Cage) and X11 session selection.
- **Libadwaita Styling**: Adopts a minimalist dark aesthetic matching GNOME/Libadwaita standards.
- **NixOS Native**: Full flake-based development environment and NixOS VM testing suite.
- **Testable**: Supports headless build tags for CI/CD and unit testing without CGO/graphical dependencies.

## Prerequisites
- Nix package manager (with flakes enabled)
- QEMU (for local VM testing)

## Quick Start
Initialize and enter the development environment:
```bash
git clone <url>
cd LightDM_Elephant_Fork
nix develop
```

## Building & Testing

We use a `justfile` to standardize the development workflow:

| Command | Description |
| :--- | :--- |
| `just test` | Run unit and D-Bus state-machine tests. |
| `just build` | Compile the full graphical binary (requires graphics libs). |
| `just build-headless` | Compile the portable headless CLI/TUI binary (zero CGO dependencies). |
| `just vm-build` | Build the integrated NixOS QEMU testing VM. |
| `just vm-run` | Run the compiled NixOS QEMU VM live. |

## NixOS Configuration

To register this greeter in your system `configuration.nix`:

```nix
services.xserver.displayManager.lightdm = {
  enable = true;
  # Disable standard GTK greeter to avoid conflicts
  greeters.gtk.enable = false;
  
  # Configure LightDM to use our greeter
  greeter = {
    name = "lightdm-elephant-greeter";
    package = (import ./default.nix { /* ... dependencies ... */ });
  };
};

# Ensure the package is installed
environment.systemPackages = [
  pkgs.lightdm-elephant-greeter
];
```

## Styling
The greeter implements the standard Libadwaita dark theme palette:
- **Background**: `#242424`
- **Accent (Blue)**: `#3584e4`
- **Panel Grey**: `#303030`
- **Text**: `#ffffff`

## Project Architecture
- `cmd/`: Application orchestrator.
- `pkg/dbus/`: State machine handling D-Bus authentication and signals.
- `pkg/ui/`: Gio-based graphical layout, with a `headless` build-tag fallback for CLI testing.
