package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/max-moser/lightdm-greeter-ganapati/pkg/dbus"
	"github.com/max-moser/lightdm-greeter-ganapati/pkg/ui"
)

// greeterOrchestrator binds the UI events and LightDM D-Bus method invocations together
type greeterOrchestrator struct {
	client dbus.GreeterClient
	// statusFn surfaces D-Bus operation failures in the UI. Set by main
	// after the window exists (UI cannot be touched before that).
	statusFn func(msg string, isError bool)
}

// OnAuthenticate starts authentication via D-Bus client
func (o *greeterOrchestrator) OnAuthenticate(username string) {
	err := o.client.Authenticate(username)
	if err != nil {
		log.Printf("Error triggering Authenticate: %v", err)
		if o.statusFn != nil {
			o.statusFn("Failed to reach the display manager: "+err.Error(), true)
		}
	}
}

// OnRespond sends credentials to display manager
func (o *greeterOrchestrator) OnRespond(password string) {
	err := o.client.Respond(password)
	if err != nil {
		log.Printf("Error sending Password response: %v", err)
		if o.statusFn != nil {
			o.statusFn("Failed to send password: "+err.Error(), true)
		}
	}
}

// OnCancel cancels current auth sequence
func (o *greeterOrchestrator) OnCancel() {
	err := o.client.CancelAuthentication()
	if err != nil {
		log.Printf("Error cancelling authentication: %v", err)
	}
}

// OnError reports a failed D-Bus operation. It is surfaced by the UI as a
// status message so failures are visible instead of silent.
func (o *greeterOrchestrator) OnError(message string) {
	log.Printf("Greeter error: %s", message)
}

// OnStartSession launches the chosen session
func (o *greeterOrchestrator) OnStartSession(sessionID string) {
	err := o.client.StartSession(sessionID)
	if err != nil {
		log.Printf("Error launching session %s: %v", sessionID, err)
	}
}

func main() {
	log.Println("Starting LightDM Greeter Ganapati...")

	// Allow display server socket to stabilize if spawned concurrently by LightDM
	time.Sleep(1500 * time.Millisecond)

	// 1. Initialize and connect the LightDM D-Bus Client
	client := dbus.NewLightDMClient()
	err := client.Connect()
	if err != nil {
		// Log and print, but continue so that UI can render locally in dev/headless modes
		log.Printf("Warning: Failed to connect to system D-Bus: %v. Running in standalone local mode.", err)
	}
	defer client.Close()

	// 2. Query available Wayland and X11 sessions
	sessions, err := client.GetSessions()
	if err != nil {
		log.Printf("Warning: Failed to retrieve sessions: %v. Using standard system fallbacks.", err)
		sessions = []dbus.Session{
			{ID: "awesome", Name: "AwesomeWM (X11 Default)", Type: "x11"},
			{ID: "sway", Name: "Sway (Wayland)", Type: "wayland"},
		}
	}

	// 3. Setup event stream: the greeter protocol (LightDM >= 1.31) when the
	// daemon provided its pipe descriptors, otherwise the legacy D-Bus signal
	// listener (LightDM <= 1.30).
	var events <-chan dbus.GreeterEvent
	if client.ProtocolConnected() {
		events = client.ProtocolEvents()
		log.Println("Using greeter protocol event stream")
	} else if client.GetConn() != nil {
		listener := dbus.NewSignalListener(client)
		err = listener.Start()
		if err != nil {
			log.Fatalf("Fatal: Failed to start signal listener: %v", err)
		}
		defer listener.Stop()
		events = listener.Events()
		log.Println("Using legacy D-Bus signal event stream")
	}

	// 4. Instantiate our orchestrator and window layout loop. The window is
	// created first so the orchestrator can surface D-Bus operation failures
	// in the UI via statusFn (SetStatus).
	orchestrator := &greeterOrchestrator{client: client}
	window := ui.NewWindow(sessions)
	orchestrator.statusFn = window.SetStatus

	// 5. Start main loop (combines LightDM events listener and presentation frames)
	err = window.Run(events, orchestrator)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Greeter interface crash: %v\n", err)
		os.Exit(1)
	}
}
