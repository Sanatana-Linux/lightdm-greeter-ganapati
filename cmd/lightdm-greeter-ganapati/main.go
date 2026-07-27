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
}

// OnAuthenticate starts authentication via D-Bus client
func (o *greeterOrchestrator) OnAuthenticate(username string) {
	err := o.client.Authenticate(username)
	if err != nil {
		log.Printf("Error triggering Authenticate: %v", err)
	}
}

// OnRespond sends credentials to display manager
func (o *greeterOrchestrator) OnRespond(password string) {
	err := o.client.Respond(password)
	if err != nil {
		log.Printf("Error sending Password response: %v", err)
	}
}

// OnCancel cancels current auth sequence
func (o *greeterOrchestrator) OnCancel() {
	err := o.client.CancelAuthentication()
	if err != nil {
		log.Printf("Error cancelling authentication: %v", err)
	}
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

	// 3. Setup signals listener
	listener := dbus.NewSignalListener(client)
	if client.GetConn() != nil {
		err = listener.Start()
		if err != nil {
			log.Fatalf("Fatal: Failed to start signal listener: %v", err)
		}
		defer listener.Stop()
	}

	// 4. Instantiate our orchestrator and window layout loop
	orchestrator := &greeterOrchestrator{client: client}
	window := ui.NewWindow(sessions)

	// 5. Start main loop (combines D-Bus events listener and presentation frames)
	err = window.Run(listener.Events(), orchestrator)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal: Greeter interface crash: %v\n", err)
		os.Exit(1)
	}
}
