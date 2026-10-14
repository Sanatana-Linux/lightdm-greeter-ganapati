package dbus

import (
	"bufio"
	"fmt"
	"log"
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
	Icon string // Icon name/path from the desktop entry (DE/WM/Compositor icon)
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

// LightDMClient implements the GreeterClient interface for production.
//
// LightDM >= 1.31 authenticates over the greeter pipe protocol
// (LIGHTDM_TO_SERVER_FD/LIGHTDM_FROM_SERVER_FD, see protocol.go); the D-Bus
// methods remain as a fallback for older LightDM versions and for property
// access (seat and session properties are still exposed over D-Bus).
type LightDMClient struct {
	conn     *dbus.Conn
	proto    *protocolConn
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

// Connect establishes the connection to LightDM. The greeter pipe protocol
// (LightDM >= 1.31) is the primary channel; the system D-Bus connection is
// secondary and used for property access (sessions, seat info). The call
// only fails when neither channel is available.
func (c *LightDMClient) Connect() error {
	if proto, ok := openProtocol(); ok {
		c.proto = proto
		c.proto.start()
		log.Printf("Connected to LightDM via greeter protocol (version %s)", greeterVersion)
	}

	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		if c.proto == nil {
			return fmt.Errorf("connect system bus: %w", err)
		}
		// Protocol is live; D-Bus is only needed for optional properties.
		log.Printf("Warning: system D-Bus unavailable: %v", err)
		return nil
	}
	c.conn = conn
	return nil
}

// connected reports whether the client has a live system-bus connection.
func (c *LightDMClient) connected() bool { return c != nil && c.conn != nil }

// Close terminates the protocol and D-Bus connections
func (c *LightDMClient) Close() error {
	if c.proto != nil {
		c.proto.stop()
		c.proto = nil
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ProtocolConnected reports whether the greeter pipe protocol is active, in
// which case ProtocolEvents delivers the authentication event stream.
func (c *LightDMClient) ProtocolConnected() bool {
	return c.proto != nil && c.proto.connected()
}

// ProtocolEvents returns the channel of greeter events parsed from the
// protocol stream. Only valid when ProtocolConnected is true.
func (c *LightDMClient) ProtocolEvents() <-chan GreeterEvent {
	if c.proto == nil {
		return nil
	}
	return c.proto.Events()
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

// sessionScanDirsFn is a package-level hook so tests can substitute a fixed
// set of session directories.
var sessionScanDirsFn = sessionScanDirs

// sessionDir is a directory holding session .desktop entries plus the session
// protocol type its contents represent.
type sessionDir struct {
	path string
	typ  string // "x11" or "wayland"
}

// sessionScanDirs returns the concrete directories to scan for session desktop
// entries. LightDM's own `sessions-directory` configuration is authoritative:
// on NixOS sessions live under a store path, not /usr/share, so scanning only
// the standard locations finds nothing. The standard locations are kept as a
// fallback for non-NixOS systems.
func sessionScanDirs() []sessionDir {
	var dirs []sessionDir
	seen := map[string]bool{}
	add := func(path, typ string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		dirs = append(dirs, sessionDir{path: path, typ: typ})
	}

	for _, d := range configuredSessionDirs() {
		typ := "x11"
		if strings.Contains(d, "wayland") {
			typ = "wayland"
		}
		add(d, typ)
	}

	// Standard fallback locations.
	for _, base := range []string{"/usr/share", "/usr/local/share", "/run/current-system/sw/share"} {
		add(filepath.Join(base, "xsessions"), "x11")
		add(filepath.Join(base, "wayland-sessions"), "wayland")
	}

	return dirs
}

// configuredSessionDirs extracts the `sessions-directory` values from
// LightDM's configuration (lightdm.conf and lightdm.conf.d/*.conf). Entries
// are colon- or whitespace-separated directory paths.
func configuredSessionDirs() []string {
	paths := []string{"/etc/lightdm/lightdm.conf"}
	if extra, _ := filepath.Glob("/etc/lightdm/lightdm.conf.d/*.conf"); len(extra) > 0 {
		paths = append(paths, extra...)
	}

	var dirs []string
	for _, p := range paths {
		dirs = append(dirs, parseSessionsDirectory(p)...)
	}
	return dirs
}

// parseSessionsDirectory reads a LightDM config file and returns the
// directories listed in its `sessions-directory` key.
func parseSessionsDirectory(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var dirs []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		if strings.TrimSpace(line[:eq]) != "sessions-directory" {
			continue
		}
		val := strings.TrimSpace(line[eq+1:])
		for _, part := range strings.FieldsFunc(val, func(r rune) bool {
			return r == ':' || r == ' ' || r == '\t'
		}) {
			if part != "" {
				dirs = append(dirs, part)
			}
		}
	}
	return dirs
}

// listSessionsFromDirectories scans the configured and standard session
// directories for session desktop entries.
func (c *LightDMClient) listSessionsFromDirectories() ([]Session, error) {
	sessions := []Session{}
	seen := map[string]bool{}

	for _, sd := range sessionScanDirsFn() {
		files, _ := filepath.Glob(filepath.Join(sd.path, "*.desktop"))
		for _, f := range files {
			id := filepath.Base(f)
			if seen[id] {
				continue
			}
			if sess, err := parseDesktopFile(f, sd.typ); err == nil {
				sessions = append(sessions, sess)
				seen[id] = true
			}
		}
	}

	// If no sessions were found (e.g. a dev environment with no session
	// files), return dummy values so the UI still renders.
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
	// Primary channel: greeter protocol (LightDM >= 1.31)
	if c.proto != nil && c.proto.connected() {
		return c.proto.sendAuthSequence(username)
	}
	if !c.connected() {
		return fmt.Errorf("not connected to display manager")
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
	// Primary channel: greeter protocol (LightDM >= 1.31)
	if c.proto != nil && c.proto.connected() {
		return c.proto.sendResponse(response)
	}
	if !c.connected() {
		return fmt.Errorf("not connected to display manager")
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
	// Primary channel: greeter protocol (LightDM >= 1.31)
	if c.proto != nil && c.proto.connected() {
		return c.proto.sendSimple(greeterMessageCancelAuthentication)
	}
	if !c.connected() {
		return fmt.Errorf("not connected to display manager")
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
	// Primary channel: greeter protocol (LightDM >= 1.31)
	if c.proto != nil && c.proto.connected() {
		return c.proto.sendStartSession(sessionID)
	}
	if !c.connected() {
		return fmt.Errorf("not connected to display manager")
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
	icon := "" // Optional Icon= entry (DE/WM/Compositor icon)

	// Simple scanner for Name= and Icon= lines
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
		} else if strings.HasPrefix(line, "Icon=") {
			icon = strings.TrimPrefix(line, "Icon=")
		}
	}

	return Session{
		ID:   id,
		Name: name,
		Type: sType,
		Icon: icon,
	}, nil
}
