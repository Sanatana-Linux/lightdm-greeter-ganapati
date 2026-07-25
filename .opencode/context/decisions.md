# Architecture Decisions — LightDM Elephant Greeter Go Rewrite

> Auto-extracted by /init-project setup --full (Phase 6).

## Decisions

- **Decision: Rewrite in Go** — Rewriting the greeter from Python/GTK3 to Go produces a self-contained static binary with zero runtime dependencies, eliminating startup latency and package dependency issues under NixOS.
- **Decision: Modular Decoupling** — Separate UI drawing logic (`pkg/ui`) entirely from display manager messaging (`pkg/dbus`), binding them at the CLI entry point (`cmd/lightdm-elephant-greeter`). This allows isolated unit testing of D-Bus protocols and independent UI mockup.
- **Decision: Table-Driven Testing** — Adopt standard Go table-driven tests for validating protocol signals, marshaling, and state machine transitions.
