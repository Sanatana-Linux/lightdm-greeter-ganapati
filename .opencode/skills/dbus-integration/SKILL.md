---
name: dbus-integration
description: Implementation steps for D-Bus LightDM interface in Go
level: 1
---

# D-Bus LightDM Integration

This skill outlines the steps to integrate the Go rewrite of the greeter with LightDM over D-Bus using `github.com/godbus/dbus/v5`.

## When to Use
- Implementing login/greeter authentication sequence
- Retrieving the user session selection lists from LightDM
- Transitioning display state from greeter to desktop session

## D-Bus Interface Specifications
LightDM exposes the greeter interface at destination `org.freedesktop.DisplayManager` on object path `/org/freedesktop/DisplayManager`:

1. **Authentication Flow**:
   - Call `org.freedesktop.DisplayManager.Greeter.Authenticate(username string)` to initiate.
   - Wait for the `ShowPrompt` signal containing the prompt text and type.
   - Reply by calling `org.freedesktop.DisplayManager.Greeter.Respond(response string)`.
   - Wait for `AuthenticationComplete` signal.
   - On success, select session and call `org.freedesktop.DisplayManager.Greeter.StartSession(sessionName string)`.

2. **Listing Sessions**:
   - Access the `Sessions` property of `org.freedesktop.DisplayManager` which contains a list of available sessions read from desktop files.
