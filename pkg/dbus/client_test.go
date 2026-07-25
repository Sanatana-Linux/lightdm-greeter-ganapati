package dbus

import (
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
)

// MockClient is a mock implementation of the GreeterClient interface for unit testing
type MockClient struct {
	conn       *dbus.Conn
	seatPath   dbus.ObjectPath
	Sessions   []Session
	AuthedUser string
	LastPass   string
	Canceled   bool
	SessStart  string
}

func (m *MockClient) Connect() error                      { return nil }
func (m *MockClient) Close() error                        { return nil }
func (m *MockClient) GetConn() *dbus.Conn                 { return m.conn }
func (m *MockClient) GetSeatPath() dbus.ObjectPath        { return m.seatPath }
func (m *MockClient) GetSessions() ([]Session, error)     { return m.Sessions, nil }
func (m *MockClient) Authenticate(username string) error  { m.AuthedUser = username; return nil }
func (m *MockClient) Respond(response string) error       { m.LastPass = response; return nil }
func (m *MockClient) CancelAuthentication() error         { m.Canceled = true; return nil }
func (m *MockClient) StartSession(sessionID string) error { m.SessStart = sessionID; return nil }

func TestNewLightDMClient(t *testing.T) {
	// Setup environment
	testSeatPath := "/org/freedesktop/DisplayManager/SeatTest"
	os.Setenv("LIGHTDM_GREETER_SEAT_PATH", testSeatPath)
	defer os.Unsetenv("LIGHTDM_GREETER_SEAT_PATH")

	client := NewLightDMClient()
	if client.GetSeatPath() != dbus.ObjectPath(testSeatPath) {
		t.Errorf("Expected seat path %s, got %s", testSeatPath, client.GetSeatPath())
	}
}

func TestNewLightDMClientDefault(t *testing.T) {
	os.Unsetenv("LIGHTDM_GREETER_SEAT_PATH")
	client := NewLightDMClient()
	expected := dbus.ObjectPath("/org/freedesktop/DisplayManager/Seat0")
	if client.GetSeatPath() != expected {
		t.Errorf("Expected default seat path %s, got %s", expected, client.GetSeatPath())
	}
}

func TestMockClientAuthSequence(t *testing.T) {
	mock := &MockClient{
		seatPath: "/org/freedesktop/DisplayManager/SeatTest",
		Sessions: []Session{
			{ID: "awesome", Name: "AwesomeWM", Type: "x11"},
			{ID: "sway", Name: "Sway", Type: "wayland"},
		},
	}

	tests := []struct {
		name       string
		username   string
		password   string
		sessionID  string
		wantUser   string
		wantPass   string
		wantSess   string
		wantCancel bool
	}{
		{
			name:       "Standard X11 Login",
			username:   "testuser",
			password:   "testpass123",
			sessionID:  "awesome",
			wantUser:   "testuser",
			wantPass:   "testpass123",
			wantSess:   "awesome",
			wantCancel: false,
		},
		{
			name:       "Wayland Login",
			username:   "anotheruser",
			password:   "secure999",
			sessionID:  "sway",
			wantUser:   "anotheruser",
			wantPass:   "secure999",
			wantSess:   "sway",
			wantCancel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate interactive authentication cycle
			_ = mock.Authenticate(tt.username)
			_ = mock.Respond(tt.password)
			_ = mock.StartSession(tt.sessionID)

			if mock.AuthedUser != tt.wantUser {
				t.Errorf("Authenticate: expected user %s, got %s", tt.wantUser, mock.AuthedUser)
			}
			if mock.LastPass != tt.wantPass {
				t.Errorf("Respond: expected password %s, got %s", tt.wantPass, mock.LastPass)
			}
			if mock.SessStart != tt.wantSess {
				t.Errorf("StartSession: expected session %s, got %s", tt.wantSess, mock.SessStart)
			}
		})
	}
}

func TestMockClientCancel(t *testing.T) {
	mock := &MockClient{}
	_ = mock.Authenticate("admin")
	_ = mock.CancelAuthentication()

	if !mock.Canceled {
		t.Error("Expected transaction to be canceled")
	}
}

func TestGetSessionsFallback(t *testing.T) {
	client := NewLightDMClient()
	sessions, err := client.listSessionsFromDirectories()
	if err != nil {
		t.Fatalf("listSessionsFromDirectories error: %v", err)
	}

	// Should fall back to standard mock items when directories are empty (test sandbox)
	if len(sessions) < 2 {
		t.Errorf("Expected at least 2 default sessions, got %d", len(sessions))
	}

	foundAwesome := false
	foundSway := false
	for _, s := range sessions {
		if s.ID == "awesome" && s.Type == "x11" {
			foundAwesome = true
		}
		if s.ID == "sway" && s.Type == "wayland" {
			foundSway = true
		}
	}

	if !foundAwesome || !foundSway {
		t.Errorf("Fallback sessions list missing defaults. Found awesome: %t, Sway: %t", foundAwesome, foundSway)
	}
}

func TestSignalListener_ListenLoop(t *testing.T) {
	mock := &MockClient{
		seatPath: "/org/freedesktop/DisplayManager/SeatTest",
	}
	listener := NewSignalListener(mock)

	// Channel to feed signals
	dbusCh := make(chan *dbus.Signal, 10)

	// Start listener loop in background
	go listener.listenLoop(dbusCh)
	defer listener.Stop()

	// 1. Test ShowPrompt signal
	dbusCh <- &dbus.Signal{
		Name: "org.freedesktop.DisplayManager.Seat.ShowPrompt",
		Body: []interface{}{"Password: ", uint32(0)},
	}

	ev := <-listener.Events()
	if ev.Type != EventShowPrompt || ev.Text != "Password: " || ev.Param != 0 {
		t.Errorf("Expected EventShowPrompt, got %+v", ev)
	}

	// 2. Test ShowMessage signal
	dbusCh <- &dbus.Signal{
		Name: "org.freedesktop.DisplayManager.Seat.ShowMessage",
		Body: []interface{}{"Authentication failed", uint32(1)},
	}

	ev = <-listener.Events()
	if ev.Type != EventShowMessage || ev.Text != "Authentication failed" || ev.Param != 1 {
		t.Errorf("Expected EventShowMessage, got %+v", ev)
	}

	// 3. Test AuthenticationComplete signal
	dbusCh <- &dbus.Signal{
		Name: "org.freedesktop.DisplayManager.Seat.AuthenticationComplete",
		Body: []interface{}{},
	}

	ev = <-listener.Events()
	if ev.Type != EventAuthComplete {
		t.Errorf("Expected EventAuthComplete, got %+v", ev)
	}

	// 4. Test Reset signal
	dbusCh <- &dbus.Signal{
		Name: "org.freedesktop.DisplayManager.Seat.Reset",
		Body: []interface{}{},
	}

	ev = <-listener.Events()
	if ev.Type != EventReset {
		t.Errorf("Expected EventReset, got %+v", ev)
	}
}

func TestListSessionsFromDirectories_WithFiles(t *testing.T) {
	// Create temporary directory to simulate /usr/share/xsessions
	tempDir, err := os.MkdirTemp("", "xsessions")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write a mock desktop file
	desktopContent := `[Desktop Entry]
Name=MockWM
Comment=A Mock Window Manager
Exec=mockwm
Type=Application
`
	desktopFilePath := tempDir + "/mockwm.desktop"
	err = os.WriteFile(desktopFilePath, []byte(desktopContent), 0644)
	if err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	sess, err := parseDesktopFile(desktopFilePath, "x11")
	if err != nil {
		t.Fatalf("parseDesktopFile failed: %v", err)
	}

	if sess.ID != "mockwm" {
		t.Errorf("expected session ID to be mockwm, got %s", sess.ID)
	}
	if sess.Name != "MockWM" {
		t.Errorf("expected session name to be MockWM, got %s", sess.Name)
	}
	if sess.Type != "x11" {
		t.Errorf("expected session type to be x11, got %s", sess.Type)
	}
}
