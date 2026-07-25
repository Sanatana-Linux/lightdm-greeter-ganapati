<!-- Parent: ../../AGENTS.md -->
<!-- Generated: 2026-07-24T12:00:00Z | Updated: 2026-07-24T12:00:00Z -->

# cmd/lightdm-elephant-greeter

## Purpose
Main entrypoint and application CLI interface for the LightDM Elephant Greeter.

## Planned Key Files

| File | Description |
|------|-------------|
| `main.go` | Main application loop, argument parsing, initialization |

## For AI Agents

### Working In This Directory
- Initialize configuration from files/environment
- Connect to LightDM D-Bus service
- Boot GUI window and delegate event processing
- Handle OS signals (SIGINT, SIGTERM) for graceful shutdown

<!-- MANUAL: Add custom notes below -->
