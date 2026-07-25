package ui

import (
	"os"
	"testing"

	"github.com/max-moser/lightdm-greeter-ganapati/pkg/dbus"
)

type MockHandler struct {
	authedUser string
	lastPass   string
	sessStart  string
}

func (m *MockHandler) OnAuthenticate(username string)  { m.authedUser = username }
func (m *MockHandler) OnRespond(password string)       { m.lastPass = password }
func (m *MockHandler) OnCancel()                       {}
func (m *MockHandler) OnStartSession(sessionID string) { m.sessStart = sessionID }

func TestHeadlessUI_Run(t *testing.T) {
	// Put loop in non-interactive testing mode so it doesn't block on stdin reader
	os.Setenv("NON_INTERACTIVE", "true")
	defer os.Unsetenv("NON_INTERACTIVE")

	sessions := []dbus.Session{
		{ID: "awesome", Name: "AwesomeWM", Type: "x11"},
	}

	window := NewWindow(sessions)
	eventsCh := make(chan dbus.GreeterEvent, 5)
	handler := &MockHandler{}

	// Enqueue events to verify the event processor branches
	eventsCh <- dbus.GreeterEvent{
		Type:  dbus.EventShowPrompt,
		Text:  "Password: ",
		Param: 0,
	}
	eventsCh <- dbus.GreeterEvent{
		Type:  dbus.EventShowMessage,
		Text:  "Authentication complete",
		Param: 0,
	}
	eventsCh <- dbus.GreeterEvent{
		Type: dbus.EventAuthComplete,
	}

	err := window.Run(eventsCh, handler)
	if err != nil {
		t.Fatalf("Window Run failed: %v", err)
	}

	// Verify callbacks was triggered by EventAuthComplete launch trigger
	if handler.sessStart != "awesome" {
		t.Errorf("Expected session 'awesome' to start, got %s", handler.sessStart)
	}
}
