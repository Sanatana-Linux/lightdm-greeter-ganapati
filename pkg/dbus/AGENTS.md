<!-- Parent: ../../AGENTS.md -->
<!-- Generated: 2026-07-24T12:00:00Z | Updated: 2026-07-24T12:00:00Z -->

# pkg/dbus

## Purpose
D-Bus communication layer for communicating with the LightDM display manager service.

## Planned Key Files

| File | Description |
|------|-------------|
| `client.go` | D-Bus client connection and method calls |
| `signals.go` | Signal listening and channel dispatching |

## For AI Agents

### Working In This Directory
- Use `github.com/godbus/dbus/v5`
- Map asynchronous D-Bus signals (`ShowPrompt`, `AuthenticationComplete`) to Go channels
- Keep D-Bus operations non-blocking for smooth UI rendering

<!-- MANUAL: Add custom notes below -->
