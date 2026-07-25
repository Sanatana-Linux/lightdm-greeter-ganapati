---
name: go-conventions
description: Coding guidelines and idiom checks for Go projects
level: 1
---

# Go Conventions & Best Practices

This skill provides the standard conventions and idioms for developing the LightDM Elephant Greeter in Go.

## When to Use
- Implementing new packages, structs, or functions in Go
- Doing code reviews or refactoring Go code

## Core Guidelines
1. **Short local variable names**: Use single-letter or short abbreviations where context is clear (e.g. `u` for User, `err` for error, `r` for Reader/Receiver).
2. **Error handling**: Always check and handle errors immediately. Wrap them with context before bubbling up:
   ```go
   if err != nil {
       return fmt.Errorf("retrieve user profile: %w", err)
   }
   ```
3. **No global variables**: Pass dependencies explicitly using constructors (Dependency Injection) instead of relying on package-level state.
4. **Interfaces define behavior**: Keep interfaces small (ideally 1-3 methods). Accept interfaces, return concrete structs.
