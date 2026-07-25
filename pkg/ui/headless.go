//go:build headless

package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/max-moser/lightdm-greeter-ganapati/pkg/dbus"
)

// UIActionHandler defines actions triggered by user interaction in the UI
type UIActionHandler interface {
	OnAuthenticate(username string)
	OnRespond(password string)
	OnCancel()
	OnStartSession(sessionID string)
}

// Window handles the text-based user interface and user inputs for headless environments
type Window struct {
	sessions    []dbus.Session
	currentSess int
	statusMsg   string
	isError     bool
}

// NewWindow initializes the headless UI window
func NewWindow(sessions []dbus.Session) *Window {
	return &Window{
		sessions: sessions,
	}
}

// Run starts the headless interactive keyboard input loop and logs display signals
func (w *Window) Run(events <-chan dbus.GreeterEvent, handler UIActionHandler) error {
	fmt.Println("==================================================")
	fmt.Println("   LIGHTDM ELEPHANT GREETER (Headless Mode)       ")
	fmt.Println("==================================================")

	if len(w.sessions) > 0 {
		fmt.Println("Available Sessions:")
		for i, s := range w.sessions {
			fmt.Printf("  [%d] %s (%s)\n", i, s.Name, s.Type)
		}
	} else {
		fmt.Println("No sessions available.")
	}

	// Channel to signal shutdown
	done := make(chan struct{})

	// Loop to handle LightDM signals in the background
	go func() {
		for ev := range events {
			switch ev.Type {
			case dbus.EventShowPrompt:
				fmt.Printf("\n[LightDM Prompt] %s", ev.Text)
				// If running interactively, prompt for password
				if strings.Contains(strings.ToLower(ev.Text), "password") {
					reader := bufio.NewReader(os.Stdin)
					pass, _ := reader.ReadString('\n')
					pass = strings.TrimSpace(pass)
					handler.OnRespond(pass)
				}

			case dbus.EventShowMessage:
				prefix := "[Info]"
				if ev.Param == 1 {
					prefix = "[Error]"
				}
				fmt.Printf("\n%s %s\n", prefix, ev.Text)

			case dbus.EventAuthComplete:
				fmt.Println("\n[Success] Authentication successful!")
				if w.currentSess < len(w.sessions) {
					sess := w.sessions[w.currentSess]
					fmt.Printf("Launching session: %s...\n", sess.Name)
					handler.OnStartSession(sess.ID)
				}
				close(done)
				return

			case dbus.EventReset:
				fmt.Println("\n[Reset] Authentication reset.")
			}
		}
	}()

	// If we are in non-interactive terminal (e.g. testing), we don't block on stdin
	if os.Getenv("NON_INTERACTIVE") != "" {
		<-done
		return nil
	}

	// Interactive terminal input loop
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-done:
			return nil
		default:
			fmt.Printf("\nEnter Username to log in (or 'q' to quit): ")
			username, err := reader.ReadString('\n')
			if err != nil {
				return nil
			}
			username = strings.TrimSpace(username)
			if username == "q" {
				return nil
			}

			// Choose session if multiple are present
			if len(w.sessions) > 1 {
				fmt.Printf("Select Session index [0-%d] (default %d): ", len(w.sessions)-1, w.currentSess)
				idxStr, _ := reader.ReadString('\n')
				idxStr = strings.TrimSpace(idxStr)
				var idx int
				if idxStr != "" {
					_, err := fmt.Sscanf(idxStr, "%d", &idx)
					if err == nil && idx >= 0 && idx < len(w.sessions) {
						w.currentSess = idx
					}
				}
			}

			handler.OnAuthenticate(username)
			// Wait for prompt event to trigger password input inside background goroutine
			<-done
			return nil
		}
	}
}
