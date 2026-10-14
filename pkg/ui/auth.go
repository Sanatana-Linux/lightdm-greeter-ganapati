package ui

// signInIntent describes what activating the primary sign-in control should do
// for the current authentication state.
type signInIntent int

const (
	// signInNone means the activation has no effect (e.g. an empty username).
	signInNone signInIntent = iota
	// signInAuthenticate means a username is ready and authentication should
	// begin.
	signInAuthenticate
	// signInRespond means the server is waiting for a password and the typed
	// value should be submitted.
	signInRespond
)

// resolveSignIn decides what the primary "Sign In" control should do given the
// current state:
//
//   - not authenticating + non-empty username -> begin authentication
//   - authenticating + server awaiting a password -> submit the password
//   - otherwise -> no-op
//
// It is a pure function so the decision can be tested without a display, and
// it backs every sign-in entry point (the Sign In button and the Enter key in
// either credential field) so the paths cannot diverge.
func resolveSignIn(isAuthenticating, needPassword bool, username string) signInIntent {
	if !isAuthenticating {
		if username != "" {
			return signInAuthenticate
		}
		return signInNone
	}
	if needPassword {
		return signInRespond
	}
	return signInNone
}
