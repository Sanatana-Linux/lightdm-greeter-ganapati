package dbus

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestWireHelpers verifies the big-endian framing helpers round-trip and
// reject truncated payloads.
func TestWireHelpers(t *testing.T) {
	var buf bytes.Buffer
	putU32(&buf, 0x01020304)
	putU32(&buf, 0)
	putString(&buf, "hello")
	putString(&buf, "")

	// Verify exact wire bytes: BE uint32s, strings length-prefixed without
	// NUL terminators.
	want := []byte{
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o',
		0x00, 0x00, 0x00, 0x00,
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("wire bytes mismatch:\n got %v\nwant %v", buf.Bytes(), want)
	}

	off := 0
	if v, ok := takeU32(buf.Bytes(), &off); !ok || v != 0x01020304 {
		t.Fatalf("takeU32 = %d, %v; want 0x01020304, true", v, ok)
	}
	if v, ok := takeU32(buf.Bytes(), &off); !ok || v != 0 {
		t.Fatalf("takeU32 = %d, %v; want 0, true", v, ok)
	}
	if s, ok := takeString(buf.Bytes(), &off); !ok || s != "hello" {
		t.Fatalf("takeString = %q, %v; want hello, true", s, ok)
	}
	if s, ok := takeString(buf.Bytes(), &off); !ok || s != "" {
		t.Fatalf("takeString = %q, %v; want empty, true", s, ok)
	}
	if off != len(buf.Bytes()) {
		t.Fatalf("offset = %d; want %d", off, len(buf.Bytes()))
	}

	// Truncated payloads must report ok=false.
	if _, ok := takeU32([]byte{1, 2, 3}, &off); ok {
		t.Fatal("takeU32 on 3 bytes should fail")
	}
	if _, ok := takeString([]byte{0, 0, 0, 5, 'a'}, &off); ok {
		t.Fatal("takeString with short body should fail")
	}
}

// buildFrame assembles a wire frame: [id uint32 BE][length uint32 BE][payload].
func buildFrame(id uint32, payload ...interface{}) []byte {
	var body bytes.Buffer
	for _, e := range payload {
		switch v := e.(type) {
		case uint32:
			putU32(&body, v)
		case string:
			putString(&body, v)
		}
	}
	var frame bytes.Buffer
	putU32(&frame, id)
	putU32(&frame, uint32(body.Len()))
	frame.Write(body.Bytes())
	return frame.Bytes()
}

func newTestConn() *protocolConn {
	return &protocolConn{
		eventsCh: make(chan GreeterEvent, 100),
		closeCh:  make(chan struct{}),
	}
}

// collect drains the event channel.
func collect(p *protocolConn) []GreeterEvent {
	var evs []GreeterEvent
	for {
		select {
		case ev := <-p.eventsCh:
			evs = append(evs, ev)
		default:
			return evs
		}
	}
}

// TestHandlePrompt verifies PROMPT_AUTHENTICATION parsing (PAM conversation
// messages: echo-off prompt + error message).
func TestHandlePrompt(t *testing.T) {
	p := newTestConn()
	frame := buildFrame(serverMessagePromptAuthentication,
		uint32(1), // sequence number
		"tlh",     // username
		uint32(2), // two messages
		uint32(pamPromptEchoOff), "Password: ",
		uint32(pamErrorMsg), "Sorry, try again",
	)
	// Strip the header; handleMessage operates on payloads.
	if !p.handleMessage(serverMessagePromptAuthentication, frame[8:]) {
		t.Fatal("handleMessage rejected valid PROMPT_AUTHENTICATION")
	}
	evs := collect(p)
	if len(evs) != 2 {
		t.Fatalf("got %d events; want 2", len(evs))
	}
	if evs[0].Type != EventShowPrompt || evs[0].Text != "Password: " || evs[0].Param != pamPromptEchoOff {
		t.Fatalf("event[0] = %+v; want ShowPrompt Password: ", evs[0])
	}
	if evs[1].Type != EventShowMessage || evs[1].Param != 1 {
		t.Fatalf("event[1] = %+v; want ShowMessage error", evs[1])
	}
}

// TestHandleEndAuthentication verifies success and failure results.
func TestHandleEndAuthentication(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		p := newTestConn()
		frame := buildFrame(serverMessageEndAuthentication,
			uint32(1), "tlh", uint32(pamSuccess))
		if !p.handleMessage(serverMessageEndAuthentication, frame[8:]) {
			t.Fatal("handleMessage rejected END_AUTHENTICATION")
		}
		evs := collect(p)
		if len(evs) != 1 || evs[0].Type != EventAuthComplete {
			t.Fatalf("events = %+v; want single AuthComplete", evs)
		}
	})

	t.Run("failure", func(t *testing.T) {
		p := newTestConn()
		frame := buildFrame(serverMessageEndAuthentication,
			uint32(2), "tlh", uint32(7)) // PAM_AUTH_ERR
		if !p.handleMessage(serverMessageEndAuthentication, frame[8:]) {
			t.Fatal("handleMessage rejected END_AUTHENTICATION")
		}
		evs := collect(p)
		// Error message first (shown in UI), then AuthComplete (resets UI
		// state without starting a session).
		if len(evs) != 2 {
			t.Fatalf("got %d events; want 2", len(evs))
		}
		if evs[0].Type != EventShowMessage || evs[0].Param != 1 {
			t.Fatalf("event[0] = %+v; want ShowMessage error", evs[0])
		}
		if evs[1].Type != EventAuthComplete {
			t.Fatalf("event[1] = %+v; want AuthComplete", evs[1])
		}
	})
}

// TestDispatchPartial feeds a message in two chunks to exercise the
// incremental buffering of the frame parser.
func TestDispatchPartial(t *testing.T) {
	p := newTestConn()
	frame := buildFrame(serverMessageEndAuthentication,
		uint32(1), "tlh", uint32(pamSuccess))

	// First dispatch gets only part of the payload.
	buf := p.dispatch(frame[:10])
	if len(buf) == 0 {
		t.Fatal("dispatch consumed incomplete message")
	}
	if got := collect(p); len(got) != 0 {
		t.Fatalf("events emitted before message complete: %+v", got)
	}

	// Second dispatch completes the message plus garbage-free tail.
	buf = p.dispatch(append(buf, frame[10:]...))
	if len(buf) != 0 {
		t.Fatalf("dispatch left %d unconsumed bytes", len(buf))
	}
	if got := collect(p); len(got) != 1 || got[0].Type != EventAuthComplete {
		t.Fatalf("events = %+v; want single AuthComplete", got)
	}
}

// TestDispatchOversized guards against arbitrarily large declared payloads.
func TestDispatchOversized(t *testing.T) {
	p := newTestConn()
	frame := make([]byte, 12)
	binary.BigEndian.PutUint32(frame[0:4], serverMessageConnected)
	binary.BigEndian.PutUint32(frame[4:8], maxServerMessage+1)
	if buf := p.dispatch(frame); buf != nil {
		t.Fatal("dispatch should abort on oversized message")
	}
}

// TestConnectedV2Parsing verifies the handshake reply is accepted for both
// protocol versions.
func TestConnectedV2Parsing(t *testing.T) {
	p := newTestConn()
	frame := buildFrame(serverMessageConnectedV2,
		uint32(1), "1.32.0", uint32(2),
		"default-session", "awesome",
		"show-manual-login", "true",
	)
	if !p.handleMessage(serverMessageConnectedV2, frame[8:]) {
		t.Fatal("handleMessage rejected CONNECTED_V2")
	}

	p2 := newTestConn()
	frame2 := buildFrame(serverMessageConnected,
		"1.30.0",
		"default-session", "awesome",
	)
	if !p2.handleMessage(serverMessageConnected, frame2[8:]) {
		t.Fatal("handleMessage rejected CONNECTED")
	}
}
