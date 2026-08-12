package dbus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
)

// Greeter protocol implementation for LightDM >= 1.31.
//
// LightDM 1.31+ removed the greeter D-Bus API (Seat.Authenticate,
// Seat.Respond, Seat.StartSession and the ShowPrompt/ShowMessage/
// AuthenticationComplete signals). Greeters now communicate with the display
// manager over two pipes inherited from the daemon:
//
//	LIGHTDM_TO_SERVER_FD   - the greeter writes client messages to this fd
//	LIGHTDM_FROM_SERVER_FD - the greeter reads server messages from this fd
//
// Messages are framed as:
//
//	[uint32 BE message id][uint32 BE payload length][payload]
//
// Payload elements are uint32 BE integers and strings encoded as
// [uint32 BE byte length][raw bytes] (no NUL terminator). The protocol is
// described in LightDM's src/greeter.c (GreeterMessage/ServerMessage enums,
// read_cb/write_message).

// Client message ids (greeter -> server).
const (
	greeterMessageConnect                = 0
	greeterMessageAuthenticate           = 1
	greeterMessageAuthenticateAsGuest    = 2
	greeterMessageContinueAuthentication = 3
	greeterMessageStartSession           = 4
	greeterMessageCancelAuthentication   = 5
	greeterMessageSetLanguage            = 6
	greeterMessageAuthenticateRemote     = 7
	greeterMessageEnsureSharedDir        = 8
)

// Server message ids (server -> greeter).
const (
	serverMessageConnected            = 0
	serverMessagePromptAuthentication = 1
	serverMessageEndAuthentication    = 2
	serverMessageSessionResult        = 3
	serverMessageSharedDirResult      = 4
	serverMessageIdle                 = 5
	serverMessageReset                = 6
	serverMessageConnectedV2          = 7
)

// Protocol version we advertise during the CONNECT handshake. LightDM
// responds with min(client, server) and uses CONNECTED_V2 when >= 1.
const greeterAPIVersion = 1

// greeterVersion is the version string sent in the CONNECT message.
const greeterVersion = "1.0"

// PAM message styles carried in PROMPT_AUTHENTICATION payloads.
const (
	pamPromptEchoOff = 1
	pamPromptEchoOn  = 2
	pamErrorMsg      = 3
	pamTextInfo      = 4
)

// pamSuccess is the PAM return code for a successful authentication.
const pamSuccess = 0

// maxServerMessage caps the accepted incoming message payload size. LightDM
// itself uses a 1024 byte buffer, so anything beyond 64 KiB is corrupt.
const maxServerMessage = 64 * 1024

// protocolConn is a client connection to LightDM's greeter protocol.
type protocolConn struct {
	toServer   *os.File // writes go to the daemon
	fromServer *os.File // messages from the daemon arrive here

	writeMu sync.Mutex // serializes writes to toServer
	authSeq uint32     // sequence number for the next AUTHENTICATE

	eventsCh chan GreeterEvent
	closeCh  chan struct{}
	done     sync.Once
}

// openProtocol connects to LightDM's greeter protocol using the pipe file
// descriptors the daemon passed to us. Returns ok=false when the environment
// does not provide both descriptors (e.g. running outside LightDM).
func openProtocol() (*protocolConn, bool) {
	toFD, err1 := envFD("LIGHTDM_TO_SERVER_FD")
	fromFD, err2 := envFD("LIGHTDM_FROM_SERVER_FD")
	if err1 != nil || err2 != nil {
		return nil, false
	}
	return &protocolConn{
		toServer:   os.NewFile(uintptr(toFD), "lightdm-to-server"),
		fromServer: os.NewFile(uintptr(fromFD), "lightdm-from-server"),
		eventsCh:   make(chan GreeterEvent, 100),
		closeCh:    make(chan struct{}),
	}, true
}

// envFD parses an inherited file descriptor from the environment.
func envFD(name string) (int, error) {
	raw, ok := os.LookupEnv(name)
	if !ok || raw == "" {
		return 0, fmt.Errorf("%s not set", name)
	}
	fd, err := strconv.Atoi(raw)
	if err != nil || fd < 0 {
		return 0, fmt.Errorf("invalid %s value %q", name, raw)
	}
	return fd, nil
}

// connected reports whether the protocol connection is usable.
func (p *protocolConn) connected() bool { return p != nil && p.toServer != nil && p.fromServer != nil }

// start performs the CONNECT handshake and begins reading server messages in
// the background. LightDM only considers the greeter ready once it has
// received the CONNECT message.
func (p *protocolConn) start() {
	var payload bytes.Buffer
	putString(&payload, greeterVersion)
	putU32(&payload, 1) // resettable: we handle SERVER_MESSAGE_RESET
	putU32(&payload, greeterAPIVersion)
	if err := p.send(greeterMessageConnect, &payload); err != nil {
		log.Printf("Greeter protocol: failed to send CONNECT: %v", err)
		return
	}
	go p.readLoop()
}

// send writes a single framed message to the daemon.
func (p *protocolConn) send(id uint32, payload *bytes.Buffer) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()

	var frame bytes.Buffer
	putU32(&frame, id)
	putU32(&frame, uint32(payload.Len()))
	frame.Write(payload.Bytes())
	_, err := p.toServer.Write(frame.Bytes())
	return err
}

// sendSimple writes a message with no payload.
func (p *protocolConn) sendSimple(id uint32) error {
	return p.send(id, &bytes.Buffer{})
}

// sendAuthSequence writes AUTHENTICATE with the next sequence number.
func (p *protocolConn) sendAuthSequence(username string) error {
	p.authSeq++
	var payload bytes.Buffer
	putU32(&payload, p.authSeq)
	putString(&payload, username)
	return p.send(greeterMessageAuthenticate, &payload)
}

// sendResponse writes CONTINUE_AUTHENTICATION with a single secret (the
// typical password case; multi-prompt PAM conversations are rare in login).
func (p *protocolConn) sendResponse(secret string) error {
	var payload bytes.Buffer
	putU32(&payload, 1) // n_secrets
	putString(&payload, secret)
	return p.send(greeterMessageContinueAuthentication, &payload)
}

// sendStartSession writes START_SESSION for the given session id.
func (p *protocolConn) sendStartSession(session string) error {
	var payload bytes.Buffer
	putString(&payload, session)
	return p.send(greeterMessageStartSession, &payload)
}

// Events returns the channel of structured greeter events parsed from the
// server stream.
func (p *protocolConn) Events() <-chan GreeterEvent {
	return p.eventsCh
}

// stop terminates the reader goroutine and closes the descriptors.
func (p *protocolConn) stop() {
	p.done.Do(func() {
		close(p.closeCh)
		p.toServer.Close()
		p.fromServer.Close()
	})
}

// readLoop incrementally parses framed messages from the daemon until EOF or
// stop. Reads are chunked and partial messages are buffered between reads.
func (p *protocolConn) readLoop() {
	defer p.stop()

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 4096)
	for {
		n, err := p.fromServer.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			buf = p.dispatch(buf)
			if buf == nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("Greeter protocol: read error: %v", err)
			}
			return
		}
	}
}

// dispatch consumes every complete message in buf and returns the remaining
// partial bytes. A nil return indicates a fatal framing error.
func (p *protocolConn) dispatch(buf []byte) []byte {
	for {
		if len(buf) < 8 {
			return buf
		}
		id := binary.BigEndian.Uint32(buf[0:4])
		plen := binary.BigEndian.Uint32(buf[4:8])
		if plen > maxServerMessage {
			log.Printf("Greeter protocol: message %d payload length %d exceeds limit", id, plen)
			return nil
		}
		if len(buf) < 8+int(plen) {
			return buf
		}
		payload := buf[8 : 8+int(plen)]
		if !p.handleMessage(id, payload) {
			return nil
		}
		buf = buf[8+int(plen):]
	}
}

// handleMessage interprets one server message and emits corresponding
// GreeterEvents. Returns false on a malformed payload (fatal).
func (p *protocolConn) handleMessage(id uint32, payload []byte) bool {
	switch id {
	case serverMessageConnected:
		return p.handleConnected(payload, false)
	case serverMessageConnectedV2:
		return p.handleConnected(payload, true)
	case serverMessagePromptAuthentication:
		return p.handlePrompt(payload)
	case serverMessageEndAuthentication:
		return p.handleEndAuthentication(payload)
	case serverMessageSessionResult:
		return p.handleSessionResult(payload)
	case serverMessageSharedDirResult:
		return p.handleSharedDirResult(payload)
	case serverMessageIdle:
		// No payload. The daemon is idle, e.g. waiting for the greeter to
		// request a session. Nothing to do.
		return true
	case serverMessageReset:
		// The daemon resets the greeter (hints are echoed). Clear any
		// in-progress authentication state in the UI.
		p.emit(GreeterEvent{Type: EventReset})
		return true
	default:
		log.Printf("Greeter protocol: unknown server message id %d", id)
		return true
	}
}

// handleConnected parses the CONNECTED/CONNECTED_V2 handshake reply. The
// payload carries the daemon version and a key/value hint table (default
// session, autologin user, ...) which we currently only log.
func (p *protocolConn) handleConnected(payload []byte, v2 bool) bool {
	off := 0
	if v2 {
		if _, ok := takeU32(payload, &off); !ok {
			return false
		}
	}
	version, ok := takeString(payload, &off)
	if !ok {
		return false
	}
	nHints := uint32(0)
	if v2 {
		n, ok := takeU32(payload, &off)
		if !ok {
			return false
		}
		nHints = n
	}
	hints := []string{}
	if v2 {
		for i := uint32(0); i < nHints; i++ {
			k, ok := takeString(payload, &off)
			if !ok {
				return false
			}
			v, ok := takeString(payload, &off)
			if !ok {
				return false
			}
			hints = append(hints, k+"="+v)
		}
	} else {
		// v1 CONNECTED: hints are an implicit alternating key/value list to
		// the end of the payload.
		for off < len(payload) {
			k, ok := takeString(payload, &off)
			if !ok {
				return false
			}
			v, ok := takeString(payload, &off)
			if !ok {
				return false
			}
			hints = append(hints, k+"="+v)
		}
	}
	log.Printf("Greeter protocol: connected to LightDM %s (hints: %v)", version, hints)
	return true
}

// handlePrompt parses PROMPT_AUTHENTICATION, which carries the PAM
// conversation: any number of prompt/info/error messages.
func (p *protocolConn) handlePrompt(payload []byte) bool {
	off := 0
	if _, ok := takeU32(payload, &off); !ok { // sequence number
		return false
	}
	if _, ok := takeString(payload, &off); !ok { // username
		return false
	}
	n, ok := takeU32(payload, &off)
	if !ok {
		return false
	}
	for i := uint32(0); i < n; i++ {
		style, ok := takeU32(payload, &off)
		if !ok {
			return false
		}
		text, ok := takeString(payload, &off)
		if !ok {
			return false
		}
		switch style {
		case pamPromptEchoOff, pamPromptEchoOn:
			// The UI switches to the password prompt and the user responds.
			p.emit(GreeterEvent{Type: EventShowPrompt, Text: text, Param: style})
		case pamErrorMsg:
			p.emit(GreeterEvent{Type: EventShowMessage, Text: text, Param: 1})
		case pamTextInfo:
			p.emit(GreeterEvent{Type: EventShowMessage, Text: text, Param: 0})
		}
	}
	return true
}

// handleEndAuthentication parses END_AUTHENTICATION. A result of PAM_SUCCESS
// (0) means the user is authorized; anything else is a failed login. Failure
// is surfaced as a message first so the UI shows it and does not start a
// session (the UI only starts a session when no status message is pending).
func (p *protocolConn) handleEndAuthentication(payload []byte) bool {
	off := 0
	if _, ok := takeU32(payload, &off); !ok { // sequence number
		return false
	}
	username, ok := takeString(payload, &off)
	if !ok {
		return false
	}
	result, ok := takeU32(payload, &off)
	if !ok {
		return false
	}
	if result != pamSuccess {
		p.emit(GreeterEvent{Type: EventShowMessage, Text: "Authentication failed", Param: 1})
	}
	p.emit(GreeterEvent{Type: EventAuthComplete, Text: username, Param: result})
	return true
}

// handleSessionResult parses SERVER_MESSAGE_SESSION_RESULT (0 = started,
// 1 = failure). StartSession is fire-and-forget in the UI, so failures are
// logged; the message still gets surfaced so it is not silent.
func (p *protocolConn) handleSessionResult(payload []byte) bool {
	off := 0
	result, ok := takeU32(payload, &off)
	if !ok {
		return false
	}
	if result == 0 {
		log.Printf("Greeter protocol: session started")
	} else {
		log.Printf("Greeter protocol: session failed to start (result %d)", result)
		p.emit(GreeterEvent{Type: EventShowMessage, Text: "Failed to start session", Param: 1})
	}
	return true
}

func (p *protocolConn) handleSharedDirResult(payload []byte) bool {
	off := 0
	dir, ok := takeString(payload, &off)
	if !ok {
		return false
	}
	log.Printf("Greeter protocol: shared data dir: %s", dir)
	return true
}

// emit sends an event to the UI channel, dropping it if the UI already
// stopped consuming (channel full).
func (p *protocolConn) emit(ev GreeterEvent) {
	select {
	case p.eventsCh <- ev:
	default:
	}
}

// --- wire format helpers (all integers big-endian) ---

func putU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

func putString(buf *bytes.Buffer, s string) {
	putU32(buf, uint32(len(s)))
	buf.WriteString(s)
}

// takeU32 reads a big-endian uint32 and advances off. ok is false when the
// payload is truncated.
func takeU32(payload []byte, off *int) (uint32, bool) {
	if *off+4 > len(payload) {
		return 0, false
	}
	v := binary.BigEndian.Uint32(payload[*off : *off+4])
	*off += 4
	return v, true
}

// takeString reads a length-prefixed string and advances off. ok is false
// when the payload is truncated.
func takeString(payload []byte, off *int) (string, bool) {
	n, ok := takeU32(payload, off)
	if !ok || *off+int(n) > len(payload) {
		return "", false
	}
	s := string(payload[*off : *off+int(n)])
	*off += int(n)
	return s, true
}
