<!-- Generated: 2026-07-24T12:00:00Z | Updated: 2026-07-24T12:00:00Z -->

# LightDM Elephant Greeter - Project Instructions

## Overview

A premium, graphical login manager/greeter rewritten in Go, designed to:
- Boots lightning-fast with minimal dependencies
- Run under both X11 and Wayland session selection
- Present a gorgeous, modern minimalist aesthetic
- Be static-binary compile safe for NixOS and standard systems

## Technology Stack

- **Language**: Go
- **UI Toolkit**: Gio (gioui.org) or other lightweight graphical toolkit
- **D-Bus Library**: `github.com/godbus/dbus/v5`
- **LSP Support**: `gopls`

## Build Commands

| Command | Description |
|---------|-------------|
| `go mod download` | Download dependencies |
| `go build -v ./...` | Build all packages |
| `go test -v ./...` | Run test suite |
| `go vet ./...` | Lint with go vet |

## Code Style

- Strict compliance with idiomatic Go (naming conventions, error wrapping)
- Small, single-responsibility functions and packages
- Construct UI code dynamically without large external widget sets
- Functional options pattern for configurations

## Testing Requirements

- Target minimum 80% test coverage on domain logic and D-Bus marshalling
- Table-driven unit tests
- Mock implementations of the LightDM D-Bus interface for headless testing

## Git Workflow

- Conventional Commits enforced
- Branches prefixed with `feature/` or `fix/`

<!-- MANUAL: Add custom instructions below this line -->
