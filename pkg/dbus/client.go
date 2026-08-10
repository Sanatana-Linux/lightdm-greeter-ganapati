package dbus

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
)

// Session represents a desktop session option (e.g. GNOME, Sway, Awesome)
type Session struct {
	ID   string // Unique identifier (e.g. "awesome", "gnome-xorg")
	Name string // Human-readable name (e.g. "AwesomeWM", "GNOME on Xorg")
	Type string // Session protocol type ("x11" or "wayland")
}

// GreeterClient defines the interface for communicating with LightDM over D-Bus
type GreeterClient interface {
	Connect() error
	Close() error
	GetSessions() ([]Session, error)
	Authenticate(username string) error
	Respond(response string) error
	CancelAuthentication() error
	StartSession(sessionID string) error
	GetConn() *dbus.Conn
	GetSeatPath() dbus.ObjectPath
}

// LightDMClient implements the GreeterClient interface for production
type LightDMClient struct {
	conn     *dbus.Conn
	seatPath dbus.ObjectPath
	dest     string
}

// NewLightDMClient creates a new uninitialized D-Bus client for LightDM
func NewLightDMClient() *LightDMClient {
	seatPathStr := os.Getenv("LIGHTDM_GREETER_SEAT_PATH")
	if seatPathStr == "" {
		seatPathStr = "/org/freedesktop/DisplayManager/Seat0" // Fallback for dev/testing
	}

	return &LightDMClient{
		seatPath: dbus.ObjectPath(seatPathStr),
		dest:     "org.freedesktop.DisplayManager",
	}
}

// Connect establishes a connection to the system D-Bus
func (c *LightDMClient) Connect() error {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return fmt.Errorf("connect system bus: %w", err)
	}
	c.conn = conn
	return nil
}

// connected reports whether the client has a live system-bus connection.
func (c *LightDMClient) connected() bool { return c != nil && c.conn != nil }

// Close terminates the D-Bus connection
func (c *LightDMClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetConn returns the underlying D-Bus connection
func (c *LightDMClient) GetConn() *dbus.Conn {
	return c.conn
}

// GetSeatPath returns the seat object path
func (c *LightDMClient) GetSeatPath() dbus.ObjectPath {
	return c.seatPath
}

// GetSessions returns all available Wayland and X11 sessions
func (c *LightDMClient) GetSessions() ([]Session, error) {
	if !c.connected() {
		// No system bus: fall back to scanning the standard session
		// directories so the greeter still lists real sessions.
		return c.listSessionsFromDirectories()
	}
	// First, try reading from LightDM D-Bus property
	obj := c.conn.Object(c.dest, c.seatPath)
	variant, err := obj.GetProperty("org.freedesktop.DisplayManager.Seat.Sessions")
	if err == nil {
		var sessionPaths []dbus.ObjectPath
		if err := variant.Store(&sessionPaths); err == nil && len(sessionPaths) > 0 {
			sessions := make([]Session, 0, len(sessionPaths))
			for _, path := range sessionPaths {
				sessObj := c.conn.Object(c.dest, path)

				idVar, errId := sessObj.GetProperty("org.freedesktop.DisplayManager.Session.Id")
				nameVar, errName := sessObj.GetProperty("org.freedesktop.DisplayManager.Session.Name")
				typeVar, errType := sessObj.GetProperty("org.freedesktop.DisplayManager.Session.Type")

				if errId == nil && errName == nil && errType == nil {
					var id, name, sType string
					_ = idVar.Store(&id)
					_ = nameVar.Store(&name)
					_ = typeVar.Store(&sType)

					sessions = append(sessions, Session{
						ID:   id,
						Name: name,
						Type: sType,
					})
				}
			}
			if len(sessions) > 0 {
				return sessions, nil
			}
		}
	}

	// Fallback: Read directly from standard directory paths (highly resilient)
	return c.listSessionsFromDirectories()
}

// listSessionsFromDirectories scans /usr/share/xsessions and /usr/share/wayland-sessions
func (c *LightDMClient) listSessionsFromDirectories() ([]Session, error) {
	sessions := []Session{}

	// Scan X11 sessions
	xFiles, _ := filepath.Glob("/usr/share/xsessions/*.desktop")
	for _, f := range xFiles {
		if sess, err := parseDesktopFile(f, "x11"); err == nil {
			sessions = append(sessions, sess)
		}
	}

	// Scan Wayland sessions
	waylandFiles, _ := filepath.Glob("/usr/share/wayland-sessions/*.desktop")
	for _, f := range waylandFiles {
		if sess, err := parseDesktopFile(f, "wayland"); err == nil {
			sessions = append(sessions, sess)
		}
	}

	// If directories are empty (e.g. inside headless mock/dev environment), return dummy values
	if len(sessions) == 0 {
		sessions = []Session{
			{ID: "awesome", Name: "AwesomeWM (X11 Default)", Type: "x11"},
			{ID: "sway", Name: "Sway (Wayland)", Type: "wayland"},
		}
	}

	return sessions, nil
}

// Authenticate starts user authentication sequence
func (c *LightDMClient) Authenticate(username string) error {
	if !c.connected() {
		return fmt.Errorf("not connected to system bus")
	}
	obj := c.conn.Object(c.dest, c.seatPath)
	var call *dbus.Call
	if username == "" {
		call = obj.Call("org.freedesktop.DisplayManager.Seat.AuthenticateAsGuest", 0)
	} else {
		call = obj.Call("org.freedesktop.DisplayManager.Seat.Authenticate", 0, username)
	}
	if call.Err != nil {
		return fmt.Errorf("call Authenticate: %w", call.Err)
	}
	return nil
}

// Respond sends the response (password/PIN) back to LightDM prompt requests
func (c *LightDMClient) Respond(response string) error {
	if !c.connected() {
		return fmt.Errorf("not connected to system bus")
	}
	obj := c.conn.Object(c.dest, c.seatPath)
	call := obj.Call("org.freedesktop.DisplayManager.Seat.Respond", 0, response)
	if call.Err != nil {
		return fmt.Errorf("call Respond: %w", call.Err)
	}
	return nil
}

// CancelAuthentication aborts the current authentication process
func (c *LightDMClient) CancelAuthentication() error {
	if !c.connected() {
		return fmt.Errorf("not connected to system bus")
	}
	obj := c.conn.Object(c.dest, c.seatPath)
	call := obj.Call("org.freedesktop.DisplayManager.Seat.CancelAuthentication", 0)
	if call.Err != nil {
		return fmt.Errorf("call CancelAuthentication: %w", call.Err)
	}
	return nil
}

// StartSession requests LightDM to launch the validated desktop session
func (c *LightDMClient) StartSession(sessionID string) error {
	if !c.connected() {
		return fmt.Errorf("not connected to system bus")
	}
	obj := c.conn.Object(c.dest, c.seatPath)
	call := obj.Call("org.freedesktop.DisplayManager.Seat.StartSession", 0, sessionID)
	if call.Err != nil {
		return fmt.Errorf("call StartSession: %w", call.Err)
	}
	return nil
}

// parseDesktopFile parses a standard desktop entry file and returns a Session struct
func parseDesktopFile(path string, sType string) (Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer file.Close()

	base := filepath.Base(path)
	id := strings.TrimSuffix(base, ".desktop")
	name := id // Fallback name

	// Simple scanner for Name= line
	var content []byte
	stat, err := file.Stat()
	if err == nil {
		content = make([]byte, stat.Size())
		_, _ = file.Read(content)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Name=") {
			name = strings.TrimPrefix(line, "Name=")
			break
		}
	}

	return Session{
		ID:   id,
		Name: name,
		Type: sType,
	}, nil
}
