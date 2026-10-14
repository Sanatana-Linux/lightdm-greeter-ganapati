//go:build !headless

package ui

import "testing"

// fakeHandler records the callbacks the UI invokes so tests can assert the
// exact action taken by a control activation.
type fakeHandler struct {
	authed    string
	responded string
	canceled  bool
	started   string
}

func (f *fakeHandler) OnAuthenticate(username string)  { f.authed = username }
func (f *fakeHandler) OnRespond(password string)       { f.responded = password }
func (f *fakeHandler) OnCancel()                       { f.canceled = true }
func (f *fakeHandler) OnStartSession(sessionID string) { f.started = sessionID }
func (f *fakeHandler) OnError(message string)          {}

// TestActivateSignIn_AuthenticatesUser covers pressing Sign In while idle: it
// must begin authentication with the entered username.
func TestActivateSignIn_AuthenticatesUser(t *testing.T) {
	w := &Window{}
	w.usernameEditor.SetText("alice")
	h := &fakeHandler{}

	w.activateSignIn("test", h)

	if h.authed != "alice" {
		t.Fatalf("OnAuthenticate = %q, want %q", h.authed, "alice")
	}
	if !w.isAuthenticating || !w.needPassword {
		t.Fatalf("expected authenticating+needPassword, got authenticating=%v needPassword=%v",
			w.isAuthenticating, w.needPassword)
	}
}

// TestActivateSignIn_EmptyUsernameNoop covers pressing Sign In with no
// username: it must be a no-op.
func TestActivateSignIn_EmptyUsernameNoop(t *testing.T) {
	w := &Window{}
	h := &fakeHandler{}

	w.activateSignIn("test", h)

	if h.authed != "" || h.responded != "" {
		t.Fatalf("expected no action, got authed=%q responded=%q", h.authed, h.responded)
	}
}

// TestActivateSignIn_SubmitsPassword covers pressing Sign In while the server
// awaits a password: the typed value must be submitted.
func TestActivateSignIn_SubmitsPassword(t *testing.T) {
	w := &Window{isAuthenticating: true, needPassword: true}
	w.usernameEditor.SetText("alice")
	w.passwordEditor.SetText("hunter2")
	h := &fakeHandler{}

	w.activateSignIn("test", h)

	if h.responded != "hunter2" {
		t.Fatalf("OnRespond = %q, want %q", h.responded, "hunter2")
	}
	if h.authed != "" {
		t.Fatalf("did not expect a second OnAuthenticate, got %q", h.authed)
	}
}
