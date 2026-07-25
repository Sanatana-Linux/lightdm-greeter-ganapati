package dbus

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

// EventType represents the category of the greeter event
type EventType int

const (
	// EventShowPrompt is emitted when the user needs to provide credentials (password, etc.)
	EventShowPrompt EventType = iota
	// EventShowMessage is emitted when an informational message or error must be displayed
	EventShowMessage
	// EventAuthComplete is emitted when the authentication sequence finishes
	EventAuthComplete
	// EventReset is emitted when the display manager resets authentication state
	EventReset
)

// GreeterEvent wraps D-Bus signals from LightDM into structured Go events
type GreeterEvent struct {
	Type  EventType
	Text  string
	Param uint32 // Represents prompt type (0=secret, 1=visible) or message type (0=info, 1=error)
}

// SignalListener subscribes to and dispatches D-Bus signals from the seat
type SignalListener struct {
	client   GreeterClient
	eventsCh chan GreeterEvent
	closeCh  chan struct{}
}

// NewSignalListener creates a new listener for a given LightDM client
func NewSignalListener(client GreeterClient) *SignalListener {
	return &SignalListener{
		client:   client,
		eventsCh: make(chan GreeterEvent, 100),
		closeCh:  make(chan struct{}),
	}
}

// Start registers signal matches and begins listening in a background goroutine
func (l *SignalListener) Start() error {
	conn := l.client.GetConn()
	seatPath := l.client.GetSeatPath()

	// Register D-Bus match rules to intercept signals from the specific seat object
	matchRule := fmt.Sprintf("type='signal',sender='org.freedesktop.DisplayManager',path='%s'", string(seatPath))
	call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	if call.Err != nil {
		return fmt.Errorf("add match rule: %w", call.Err)
	}

	dbusCh := make(chan *dbus.Signal, 50)
	conn.Signal(dbusCh)

	go l.listenLoop(dbusCh)

	return nil
}

// listenLoop loops over raw D-Bus signals and converts them to GreeterEvents
func (l *SignalListener) listenLoop(dbusCh <-chan *dbus.Signal) {
	for {
		select {
		case <-l.closeCh:
			return
		case sig, ok := <-dbusCh:
			if !ok {
				return
			}

			// Parse signal member name (e.g., "ShowPrompt")
			switch sig.Name {
			case "org.freedesktop.DisplayManager.Seat.ShowPrompt":
				if len(sig.Body) >= 2 {
					text, okText := sig.Body[0].(string)
					pType, okType := sig.Body[1].(uint32)
					if okText && okType {
						l.eventsCh <- GreeterEvent{
							Type:  EventShowPrompt,
							Text:  text,
							Param: pType,
						}
					}
				}

			case "org.freedesktop.DisplayManager.Seat.ShowMessage":
				if len(sig.Body) >= 2 {
					text, okText := sig.Body[0].(string)
					mType, okType := sig.Body[1].(uint32)
					if okText && okType {
						l.eventsCh <- GreeterEvent{
							Type:  EventShowMessage,
							Text:  text,
							Param: mType,
						}
					}
				}

			case "org.freedesktop.DisplayManager.Seat.AuthenticationComplete":
				l.eventsCh <- GreeterEvent{
					Type: EventAuthComplete,
				}

			case "org.freedesktop.DisplayManager.Seat.Reset":
				l.eventsCh <- GreeterEvent{
					Type: EventReset,
				}
			}
		}
	}
}

// Events returns the channel of structured greeter events
func (l *SignalListener) Events() <-chan GreeterEvent {
	return l.eventsCh
}

// Stop terminates the signal listener goroutine and removes match rules
func (l *SignalListener) Stop() {
	close(l.closeCh)

	// Unregister match rule
	conn := l.client.GetConn()
	if conn != nil {
		seatPath := l.client.GetSeatPath()
		matchRule := fmt.Sprintf("type='signal',sender='org.freedesktop.DisplayManager',path='%s'", string(seatPath))
		_ = conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule)
	}
}
